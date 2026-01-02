// Package mcp provides interfaces and types for Model Context Protocol (MCP) server integration.
//
// This file implements resource subscriptions - a mechanism for real-time updates
// from MCP servers when resources change.
//
// # Design
//
// The ResourceSubscriber manages subscriptions to resources across multiple MCP servers.
// When a resource changes on the server, the server sends a notification that is
// routed to the appropriate subscription handler. The subscriber maintains:
//
//   - A registry of active subscriptions (URI -> subscription metadata)
//   - Update channels for each subscription (buffered to prevent blocking)
//   - Thread-safe access via sync.RWMutex
//   - Automatic cleanup of failed subscriptions
//
// # Subscription Lifecycle
//
//   1. Client calls Subscribe(serverID, resourceURI, handler)
//   2. ResourceSubscriber sends subscribe_resource request to server
//   3. Server acknowledges and begins monitoring the resource
//   4. When resource changes, server sends resource_update notification
//   5. ResourceSubscriber routes the notification to the handler
//   6. Client calls Unsubscribe(subscriptionID) to stop receiving updates
//   7. ResourceSubscriber sends unsubscribe_resource request to server
//   8. Server stops monitoring and ResourceSubscriber cleans up resources
//
// # Error Handling
//
// If a subscription fails:
//   - The error is sent to the handler via the update channel
//   - The subscription remains active (client can retry)
//   - Health check periodically validates subscriptions
//
// # Thread Safety
//
// All methods are safe for concurrent use. Subscriptions are stored in
// a map protected by RWMutex, allowing multiple concurrent reads and
// exclusive writes.
package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubscriptionID is a unique identifier for a resource subscription.
type SubscriptionID string

// Subscription represents an active resource subscription.
type Subscription struct {
	// ID is the unique subscription identifier
	ID SubscriptionID

	// ServerID is the name of the MCP server providing the resource
	ServerID string

	// ResourceURI is the URI of the subscribed resource
	ResourceURI string

	// Channel receives updates for this subscription
	// The channel is buffered to prevent blocking on sends
	Channel <-chan *ResourceUpdate

	// CreatedAt is when the subscription was created
	CreatedAt time.Time

	// LastUpdate is when the last update was received
	LastUpdate *time.Time

	// Active indicates if the subscription is still valid
	Active bool

	// Error is set if the subscription encountered an error
	Error error
}

// ResourceSubscriber manages subscriptions to resources from MCP servers.
//
// The subscriber acts as a hub for resource subscriptions, handling the
// lifecycle of subscriptions and routing notifications from servers to
// the appropriate handlers.
type ResourceSubscriber interface {
	// Subscribe creates a new subscription to a resource on an MCP server.
	// Returns a subscription ID and a channel that receives updates.
	//
	// The handler function is called for each update. It should process
	// the update and is responsible for handling errors via the update.Error field.
	//
	// Returns an error if:
	// - The server is not found
	// - The server doesn't support subscriptions
	// - The subscription request fails
	Subscribe(ctx context.Context, serverID, resourceURI string, handler func(*ResourceUpdate)) (SubscriptionID, error)

	// Unsubscribe cancels a resource subscription.
	// The server is notified to stop monitoring the resource.
	// The update channel is closed.
	//
	// Returns an error if:
	// - The subscription is not found
	// - The unsubscribe request fails on the server
	Unsubscribe(ctx context.Context, subscriptionID SubscriptionID) error

	// GetSubscription returns the current state of a subscription.
	// Returns nil if the subscription is not found.
	GetSubscription(subscriptionID SubscriptionID) *Subscription

	// ListSubscriptions returns all active subscriptions.
	ListSubscriptions() []*Subscription

	// ListSubscriptionsByServer returns all subscriptions for a specific server.
	ListSubscriptionsByServer(serverID string) []*Subscription

	// ListSubscriptionsByResource returns all subscriptions for a specific resource.
	ListSubscriptionsByResource(resourceURI string) []*Subscription

	// HandleNotification processes a resource_update notification from a server.
	// This is called internally by the registry when notifications arrive.
	// Returns an error if the notification cannot be processed.
	HandleNotification(ctx context.Context, serverID string, update *ResourceUpdate) error

	// Close closes all subscriptions and releases resources.
	// Existing subscriptions are unsubscribed from their servers.
	Close() error
}

// subscriptionImpl implements the ResourceSubscriber interface.
type subscriptionImpl struct {
	mu sync.RWMutex

	// subscriptions maps subscription ID to subscription metadata
	subscriptions map[SubscriptionID]*subscriptionState

	// serverSubscriptions maps server ID to a set of subscription IDs
	serverSubscriptions map[string]map[SubscriptionID]bool

	// resourceSubscriptions maps resource URI to a set of subscription IDs
	resourceSubscriptions map[string]map[SubscriptionID]bool

	// registry provides access to MCP servers
	registry Registry

	// channelBuffer is the size of the update channel buffer
	channelBuffer int

	// subscriptionTimeout is the maximum time to wait for a subscription to complete
	subscriptionTimeout time.Duration
}

// subscriptionState tracks internal state for a subscription.
type subscriptionState struct {
	// subscription is the public subscription info
	subscription *Subscription

	// channel is the internal channel (read-write)
	channel chan *ResourceUpdate

	// handler is called for each update
	handler func(*ResourceUpdate)

	// cancelFunc cancels the subscription context
	cancelFunc context.CancelFunc
}

// Compile-time check that subscriptionImpl implements ResourceSubscriber interface
var _ ResourceSubscriber = (*subscriptionImpl)(nil)

// NewResourceSubscriber creates a new resource subscriber with default settings.
func NewResourceSubscriber(registry Registry) ResourceSubscriber {
	return &subscriptionImpl{
		subscriptions:         make(map[SubscriptionID]*subscriptionState),
		serverSubscriptions:   make(map[string]map[SubscriptionID]bool),
		resourceSubscriptions: make(map[string]map[SubscriptionID]bool),
		registry:              registry,
		channelBuffer:         100, // Buffer up to 100 updates per subscription
		subscriptionTimeout:   30 * time.Second,
	}
}

// NewResourceSubscriberWithOptions creates a new resource subscriber with custom options.
func NewResourceSubscriberWithOptions(registry Registry, channelBuffer int, timeout time.Duration) ResourceSubscriber {
	return &subscriptionImpl{
		subscriptions:         make(map[SubscriptionID]*subscriptionState),
		serverSubscriptions:   make(map[string]map[SubscriptionID]bool),
		resourceSubscriptions: make(map[string]map[SubscriptionID]bool),
		registry:              registry,
		channelBuffer:         channelBuffer,
		subscriptionTimeout:   timeout,
	}
}

// Subscribe creates a new subscription to a resource on an MCP server.
func (rs *subscriptionImpl) Subscribe(ctx context.Context, serverID, resourceURI string, handler func(*ResourceUpdate)) (SubscriptionID, error) {
	// Validate inputs
	if serverID == "" {
		return "", fmt.Errorf("server ID cannot be empty")
	}
	if resourceURI == "" {
		return "", fmt.Errorf("resource URI cannot be empty")
	}
	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}

	// Get the server
	server, ok := rs.registry.Get(serverID)
	if !ok {
		return "", fmt.Errorf("server %q not found in registry", serverID)
	}

	// Check if server supports subscriptions
	caps := server.Capabilities()
	if caps == nil || caps.Resources == nil || !caps.Resources.Subscribe {
		return "", fmt.Errorf("server %q does not support resource subscriptions", serverID)
	}

	// Create subscription context with timeout
	subCtx, cancel := context.WithTimeout(ctx, rs.subscriptionTimeout)
	defer cancel()

	// Subscribe to the resource on the server
	updateChan, err := server.SubscribeResource(subCtx, resourceURI)
	if err != nil {
		return "", fmt.Errorf("failed to subscribe to resource %q on server %q: %w", resourceURI, serverID, err)
	}

	// Generate subscription ID
	subID := SubscriptionID(uuid.New().String())

	// Create internal channel
	internalChan := make(chan *ResourceUpdate, rs.channelBuffer)

	// Create subscription state
	state := &subscriptionState{
		subscription: &Subscription{
			ID:          subID,
			ServerID:    serverID,
			ResourceURI: resourceURI,
			Channel:     internalChan,
			CreatedAt:   time.Now(),
			Active:      true,
		},
		channel:    internalChan,
		handler:    handler,
		cancelFunc: cancel,
	}

	// Register subscription
	rs.mu.Lock()
	rs.subscriptions[subID] = state
	if rs.serverSubscriptions[serverID] == nil {
		rs.serverSubscriptions[serverID] = make(map[SubscriptionID]bool)
	}
	rs.serverSubscriptions[serverID][subID] = true
	if rs.resourceSubscriptions[resourceURI] == nil {
		rs.resourceSubscriptions[resourceURI] = make(map[SubscriptionID]bool)
	}
	rs.resourceSubscriptions[resourceURI][subID] = true
	rs.mu.Unlock()

	// Start a goroutine to handle updates from the server
	go rs.handleSubscriptionUpdates(state, updateChan)

	return subID, nil
}

// handleSubscriptionUpdates processes updates from a server subscription.
func (rs *subscriptionImpl) handleSubscriptionUpdates(state *subscriptionState, serverChan <-chan *ResourceUpdate) {
	for {
		select {
		case update, ok := <-serverChan:
			if !ok {
				// Server closed the channel (subscription ended)
				rs.markSubscriptionInactive(state.subscription.ID, fmt.Errorf("server closed subscription channel"))
				return
			}

			// Update the timestamp
			if update != nil {
				now := time.Now()
				rs.mu.Lock()
				if sub, exists := rs.subscriptions[state.subscription.ID]; exists {
					sub.subscription.LastUpdate = &now
				}
				rs.mu.Unlock()
			}

			// Send to internal channel
			select {
			case state.channel <- update:
				// Update sent successfully, call handler
				if state.handler != nil && update != nil {
					go state.handler(update)
				}
			default:
				// Channel buffer full, log error
				if update != nil {
					update.Error = fmt.Errorf("subscription channel buffer full")
					if state.handler != nil {
						go state.handler(update)
					}
				}
			}

		case <-time.After(5 * time.Minute):
			// Check if subscription is still active
			rs.mu.RLock()
			sub, exists := rs.subscriptions[state.subscription.ID]
			rs.mu.RUnlock()
			if !exists || !sub.subscription.Active {
				return
			}
		}
	}
}

// markSubscriptionInactive marks a subscription as inactive and removes it from registries.
func (rs *subscriptionImpl) markSubscriptionInactive(subID SubscriptionID, err error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	state, exists := rs.subscriptions[subID]
	if !exists {
		return
	}

	state.subscription.Active = false
	state.subscription.Error = err

	// Close the internal channel
	close(state.channel)

	// Remove from server subscriptions
	if subscriptions, ok := rs.serverSubscriptions[state.subscription.ServerID]; ok {
		delete(subscriptions, subID)
	}

	// Remove from resource subscriptions
	if subscriptions, ok := rs.resourceSubscriptions[state.subscription.ResourceURI]; ok {
		delete(subscriptions, subID)
	}
}

// Unsubscribe cancels a resource subscription.
func (rs *subscriptionImpl) Unsubscribe(ctx context.Context, subscriptionID SubscriptionID) error {
	rs.mu.Lock()
	state, exists := rs.subscriptions[subscriptionID]
	if !exists {
		rs.mu.Unlock()
		return fmt.Errorf("subscription %q not found", subscriptionID)
	}
	rs.mu.Unlock()

	// Get the server
	server, ok := rs.registry.Get(state.subscription.ServerID)
	if !ok {
		// Server is gone, just clean up locally
		rs.mu.Lock()
		delete(rs.subscriptions, subscriptionID)
		if subscriptions, ok := rs.serverSubscriptions[state.subscription.ServerID]; ok {
			delete(subscriptions, subscriptionID)
		}
		if subscriptions, ok := rs.resourceSubscriptions[state.subscription.ResourceURI]; ok {
			delete(subscriptions, subscriptionID)
		}
		rs.mu.Unlock()
		return nil
	}

	// Unsubscribe from the server
	unsubCtx, cancel := context.WithTimeout(ctx, rs.subscriptionTimeout)
	defer cancel()

	err := server.UnsubscribeResource(unsubCtx, state.subscription.ResourceURI)

	// Mark subscription as inactive and clean up
	rs.markSubscriptionInactive(subscriptionID, err)

	// Remove from subscriptions map
	rs.mu.Lock()
	delete(rs.subscriptions, subscriptionID)
	rs.mu.Unlock()

	return err
}

// GetSubscription returns the current state of a subscription.
func (rs *subscriptionImpl) GetSubscription(subscriptionID SubscriptionID) *Subscription {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	state, exists := rs.subscriptions[subscriptionID]
	if !exists {
		return nil
	}

	// Return a copy to avoid external modifications
	sub := *state.subscription
	return &sub
}

// ListSubscriptions returns all active subscriptions.
func (rs *subscriptionImpl) ListSubscriptions() []*Subscription {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	subs := make([]*Subscription, 0, len(rs.subscriptions))
	for _, state := range rs.subscriptions {
		sub := *state.subscription
		subs = append(subs, &sub)
	}
	return subs
}

// ListSubscriptionsByServer returns all subscriptions for a specific server.
func (rs *subscriptionImpl) ListSubscriptionsByServer(serverID string) []*Subscription {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	subIDs, ok := rs.serverSubscriptions[serverID]
	if !ok {
		return nil
	}

	subs := make([]*Subscription, 0, len(subIDs))
	for subID := range subIDs {
		if state, exists := rs.subscriptions[subID]; exists {
			sub := *state.subscription
			subs = append(subs, &sub)
		}
	}
	return subs
}

// ListSubscriptionsByResource returns all subscriptions for a specific resource.
func (rs *subscriptionImpl) ListSubscriptionsByResource(resourceURI string) []*Subscription {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	subIDs, ok := rs.resourceSubscriptions[resourceURI]
	if !ok {
		return nil
	}

	subs := make([]*Subscription, 0, len(subIDs))
	for subID := range subIDs {
		if state, exists := rs.subscriptions[subID]; exists {
			sub := *state.subscription
			subs = append(subs, &sub)
		}
	}
	return subs
}

// HandleNotification processes a resource_update notification from a server.
func (rs *subscriptionImpl) HandleNotification(ctx context.Context, serverID string, update *ResourceUpdate) error {
	if update == nil {
		return fmt.Errorf("update cannot be nil")
	}

	// Find all subscriptions for this resource
	rs.mu.RLock()
	subIDs, ok := rs.resourceSubscriptions[update.URI]
	if !ok {
		rs.mu.RUnlock()
		return fmt.Errorf("no subscriptions found for resource %q", update.URI)
	}

	// Create a list of subscription states to update
	statesToUpdate := make([]*subscriptionState, 0)
	for subID := range subIDs {
		if state, exists := rs.subscriptions[subID]; exists && state.subscription.ServerID == serverID {
			statesToUpdate = append(statesToUpdate, state)
		}
	}
	rs.mu.RUnlock()

	// Send update to each subscription
	for _, state := range statesToUpdate {
		select {
		case state.channel <- update:
			// Update sent successfully, call handler
			if state.handler != nil {
				go state.handler(update)
			}
		default:
			// Channel buffer full, call handler with error
			errorUpdate := *update
			errorUpdate.Error = fmt.Errorf("subscription channel buffer full")
			if state.handler != nil {
				go state.handler(&errorUpdate)
			}
		}
	}

	return nil
}

// Close closes all subscriptions and releases resources.
func (rs *subscriptionImpl) Close() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	var lastErr error

	// Unsubscribe from all subscriptions
	for _, state := range rs.subscriptions {
		server, ok := rs.registry.Get(state.subscription.ServerID)
		if !ok {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), rs.subscriptionTimeout)
		if err := server.UnsubscribeResource(ctx, state.subscription.ResourceURI); err != nil {
			lastErr = err
		}
		cancel()

		// Close the channel
		close(state.channel)
	}

	// Clear all maps
	rs.subscriptions = make(map[SubscriptionID]*subscriptionState)
	rs.serverSubscriptions = make(map[string]map[SubscriptionID]bool)
	rs.resourceSubscriptions = make(map[string]map[SubscriptionID]bool)

	return lastErr
}
