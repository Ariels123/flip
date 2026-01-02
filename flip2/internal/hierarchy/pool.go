// Package hierarchy provides 3-tier agent hierarchy schema and role definitions.
// This file implements the WorkerPool type for managing worker lifecycle with
// size limits and health monitoring.
package hierarchy

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// HealthStatus represents the health state of a worker.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the worker is operating normally.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusDegraded indicates the worker has some issues but is functional.
	HealthStatusDegraded HealthStatus = "degraded"

	// HealthStatusUnhealthy indicates the worker is not functioning properly.
	HealthStatusUnhealthy HealthStatus = "unhealthy"

	// HealthStatusUnknown indicates health status cannot be determined.
	HealthStatusUnknown HealthStatus = "unknown"
)

// WorkerHealth tracks health information for a worker.
type WorkerHealth struct {
	// WorkerID is the unique identifier of the worker.
	WorkerID string `json:"worker_id"`

	// Status is the current health status.
	Status HealthStatus `json:"status"`

	// LastCheckTime is when the last health check occurred.
	LastCheckTime time.Time `json:"last_check_time"`

	// LastHealthyTime is when the worker was last confirmed healthy.
	LastHealthyTime time.Time `json:"last_healthy_time"`

	// ConsecutiveFailures is the number of consecutive health check failures.
	ConsecutiveFailures int `json:"consecutive_failures"`

	// TotalFailures is the total number of health check failures since registration.
	TotalFailures int `json:"total_failures"`

	// ErrorMessage contains the last error message if unhealthy.
	ErrorMessage string `json:"error_message,omitempty"`

	// ResponseTimeMs is the last health check response time in milliseconds.
	ResponseTimeMs int64 `json:"response_time_ms"`

	// Metadata stores additional health-related information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PoolConfig configures worker pool behavior.
type PoolConfig struct {
	// MaxSize is the maximum number of workers allowed in the pool.
	MaxSize int `json:"max_size"`

	// MinSize is the minimum number of workers to maintain (for auto-scaling).
	MinSize int `json:"min_size"`

	// HealthCheckInterval is how often to check worker health.
	HealthCheckInterval time.Duration `json:"health_check_interval"`

	// HealthCheckTimeout is the maximum time to wait for a health check response.
	HealthCheckTimeout time.Duration `json:"health_check_timeout"`

	// UnhealthyThreshold is the number of consecutive failures before marking unhealthy.
	UnhealthyThreshold int `json:"unhealthy_threshold"`

	// DegradedThreshold is the number of consecutive failures before marking degraded.
	DegradedThreshold int `json:"degraded_threshold"`

	// AutoRemoveUnhealthy enables automatic removal of unhealthy workers.
	AutoRemoveUnhealthy bool `json:"auto_remove_unhealthy"`

	// MaxResponseTimeMs is the maximum acceptable response time before marking degraded.
	MaxResponseTimeMs int64 `json:"max_response_time_ms"`
}

// DefaultPoolConfig returns sensible default pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxSize:             10,
		MinSize:             0,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		UnhealthyThreshold:  3,
		DegradedThreshold:   1,
		AutoRemoveUnhealthy: true,
		MaxResponseTimeMs:   5000, // 5 seconds
	}
}

// HealthChecker is a function that checks worker health.
// Returns true if healthy, false otherwise, with optional error message.
type HealthChecker func(ctx context.Context, workerID string) (healthy bool, responseTimeMs int64, err error)

// WorkerPoolEventType defines types of pool events.
type WorkerPoolEventType string

const (
	EventWorkerAdded         WorkerPoolEventType = "worker_added"
	EventWorkerRemoved       WorkerPoolEventType = "worker_removed"
	EventWorkerHealthChanged WorkerPoolEventType = "worker_health_changed"
	EventPoolSizeLimitHit    WorkerPoolEventType = "pool_size_limit_hit"
	EventHealthCheckFailed   WorkerPoolEventType = "health_check_failed"
)

// WorkerPoolEvent represents an event in the worker pool.
type WorkerPoolEvent struct {
	Type      WorkerPoolEventType `json:"type"`
	WorkerID  string              `json:"worker_id,omitempty"`
	Timestamp time.Time           `json:"timestamp"`
	Details   string              `json:"details,omitempty"`
	OldHealth *WorkerHealth       `json:"old_health,omitempty"`
	NewHealth *WorkerHealth       `json:"new_health,omitempty"`
}

// WorkerPool manages a pool of workers with size limits and health monitoring.
type WorkerPool struct {
	mu sync.RWMutex

	// id is a unique identifier for this pool.
	id string

	// supervisorID is the ID of the supervisor that owns this pool.
	supervisorID string

	// config contains pool configuration.
	config PoolConfig

	// workers maps worker IDs to their hierarchy nodes.
	workers map[string]*HierarchyNode

	// health maps worker IDs to their health status.
	health map[string]*WorkerHealth

	// healthChecker is the function used to check worker health.
	healthChecker HealthChecker

	// eventHandler receives pool events (optional).
	eventHandler func(event *WorkerPoolEvent)

	// createdAt is when the pool was created.
	createdAt time.Time

	// lastHealthCheck is when the last health check cycle completed.
	lastHealthCheck time.Time

	// stopChan signals the health monitoring goroutine to stop.
	stopChan chan struct{}

	// running indicates if health monitoring is active.
	running bool
}

// PoolStats contains pool statistics.
type PoolStats struct {
	// TotalWorkers is the current number of workers in the pool.
	TotalWorkers int `json:"total_workers"`

	// HealthyWorkers is the number of healthy workers.
	HealthyWorkers int `json:"healthy_workers"`

	// DegradedWorkers is the number of degraded workers.
	DegradedWorkers int `json:"degraded_workers"`

	// UnhealthyWorkers is the number of unhealthy workers.
	UnhealthyWorkers int `json:"unhealthy_workers"`

	// UnknownWorkers is the number of workers with unknown health.
	UnknownWorkers int `json:"unknown_workers"`

	// MaxSize is the maximum pool size.
	MaxSize int `json:"max_size"`

	// MinSize is the minimum pool size.
	MinSize int `json:"min_size"`

	// AvailableSlots is how many more workers can be added.
	AvailableSlots int `json:"available_slots"`

	// LastHealthCheck is when the last health check occurred.
	LastHealthCheck time.Time `json:"last_health_check"`

	// CreatedAt is when the pool was created.
	CreatedAt time.Time `json:"created_at"`

	// UptimeSeconds is how long the pool has been running.
	UptimeSeconds float64 `json:"uptime_seconds"`
}

// NewWorkerPool creates a new worker pool with the given configuration.
func NewWorkerPool(id, supervisorID string, config PoolConfig) *WorkerPool {
	if config.MaxSize <= 0 {
		config.MaxSize = DefaultPoolConfig().MaxSize
	}
	if config.HealthCheckInterval <= 0 {
		config.HealthCheckInterval = DefaultPoolConfig().HealthCheckInterval
	}
	if config.UnhealthyThreshold <= 0 {
		config.UnhealthyThreshold = DefaultPoolConfig().UnhealthyThreshold
	}

	return &WorkerPool{
		id:           id,
		supervisorID: supervisorID,
		config:       config,
		workers:      make(map[string]*HierarchyNode),
		health:       make(map[string]*WorkerHealth),
		createdAt:    time.Now(),
		stopChan:     make(chan struct{}),
	}
}

// ID returns the pool's unique identifier.
func (p *WorkerPool) ID() string {
	return p.id
}

// SupervisorID returns the ID of the supervisor that owns this pool.
func (p *WorkerPool) SupervisorID() string {
	return p.supervisorID
}

// Config returns the current pool configuration.
func (p *WorkerPool) Config() PoolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// SetHealthChecker sets the health check function for the pool.
func (p *WorkerPool) SetHealthChecker(checker HealthChecker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthChecker = checker
}

// SetEventHandler sets the event handler for pool events.
func (p *WorkerPool) SetEventHandler(handler func(event *WorkerPoolEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.eventHandler = handler
}

// AddWorker adds a worker to the pool.
// Returns an error if the pool is at capacity.
func (p *WorkerPool) AddWorker(worker *HierarchyNode) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if worker == nil {
		return fmt.Errorf("worker cannot be nil")
	}

	if worker.AgentID == "" {
		return fmt.Errorf("worker ID cannot be empty")
	}

	// Check size limit
	if len(p.workers) >= p.config.MaxSize {
		p.emitEventLocked(&WorkerPoolEvent{
			Type:      EventPoolSizeLimitHit,
			WorkerID:  worker.AgentID,
			Timestamp: time.Now(),
			Details:   fmt.Sprintf("pool at capacity (%d/%d)", len(p.workers), p.config.MaxSize),
		})
		return fmt.Errorf("pool at maximum capacity (%d workers)", p.config.MaxSize)
	}

	// Check if worker already exists
	if _, exists := p.workers[worker.AgentID]; exists {
		return fmt.Errorf("worker %q already in pool", worker.AgentID)
	}

	// Add worker
	p.workers[worker.AgentID] = worker

	// Initialize health status
	now := time.Now()
	p.health[worker.AgentID] = &WorkerHealth{
		WorkerID:        worker.AgentID,
		Status:          HealthStatusUnknown,
		LastCheckTime:   time.Time{},
		LastHealthyTime: time.Time{},
		Metadata:        make(map[string]interface{}),
	}

	p.emitEventLocked(&WorkerPoolEvent{
		Type:      EventWorkerAdded,
		WorkerID:  worker.AgentID,
		Timestamp: now,
		Details:   fmt.Sprintf("worker added to pool (size: %d/%d)", len(p.workers), p.config.MaxSize),
	})

	return nil
}

// RemoveWorker removes a worker from the pool.
func (p *WorkerPool) RemoveWorker(workerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.removeWorkerLocked(workerID, "manual removal")
}

// removeWorkerLocked removes a worker (must be called with lock held).
func (p *WorkerPool) removeWorkerLocked(workerID, reason string) error {
	worker, exists := p.workers[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found in pool", workerID)
	}

	oldHealth := p.health[workerID]

	delete(p.workers, workerID)
	delete(p.health, workerID)

	p.emitEventLocked(&WorkerPoolEvent{
		Type:      EventWorkerRemoved,
		WorkerID:  workerID,
		Timestamp: time.Now(),
		Details:   fmt.Sprintf("%s (was: %s, size: %d/%d)", reason, worker.Status, len(p.workers), p.config.MaxSize),
		OldHealth: oldHealth,
	})

	return nil
}

// GetWorker returns a worker by ID.
func (p *WorkerPool) GetWorker(workerID string) (*HierarchyNode, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	worker, exists := p.workers[workerID]
	return worker, exists
}

// GetWorkerHealth returns the health status of a worker.
func (p *WorkerPool) GetWorkerHealth(workerID string) (*WorkerHealth, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	health, exists := p.health[workerID]
	if !exists {
		return nil, false
	}

	// Return a copy
	healthCopy := *health
	return &healthCopy, true
}

// ListWorkers returns all workers in the pool.
func (p *WorkerPool) ListWorkers() []*HierarchyNode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]*HierarchyNode, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	
	// Sort by AgentID for deterministic order
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].AgentID < workers[j].AgentID
	})

	return workers
}

// ListHealthyWorkers returns all healthy workers.
func (p *WorkerPool) ListHealthyWorkers() []*HierarchyNode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]*HierarchyNode, 0)
	for workerID, worker := range p.workers {
		if health, exists := p.health[workerID]; exists && health.Status == HealthStatusHealthy {
			workers = append(workers, worker)
		}
	}
	return workers
}

// ListUnhealthyWorkers returns all unhealthy workers.
func (p *WorkerPool) ListUnhealthyWorkers() []*HierarchyNode {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]*HierarchyNode, 0)
	for workerID, worker := range p.workers {
		if health, exists := p.health[workerID]; exists && health.Status == HealthStatusUnhealthy {
			workers = append(workers, worker)
		}
	}
	return workers
}

// Size returns the current number of workers in the pool.
func (p *WorkerPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// AvailableSlots returns how many more workers can be added.
func (p *WorkerPool) AvailableSlots() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.MaxSize - len(p.workers)
}

// IsFull returns true if the pool is at capacity.
func (p *WorkerPool) IsFull() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers) >= p.config.MaxSize
}

// Stats returns current pool statistics.
func (p *WorkerPool) Stats() *PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := &PoolStats{
		TotalWorkers:    len(p.workers),
		MaxSize:         p.config.MaxSize,
		MinSize:         p.config.MinSize,
		AvailableSlots:  p.config.MaxSize - len(p.workers),
		LastHealthCheck: p.lastHealthCheck,
		CreatedAt:       p.createdAt,
		UptimeSeconds:   time.Since(p.createdAt).Seconds(),
	}

	// Count workers by health status
	for _, health := range p.health {
		switch health.Status {
		case HealthStatusHealthy:
			stats.HealthyWorkers++
		case HealthStatusDegraded:
			stats.DegradedWorkers++
		case HealthStatusUnhealthy:
			stats.UnhealthyWorkers++
		case HealthStatusUnknown:
			stats.UnknownWorkers++
		}
	}

	return stats
}

// StartHealthMonitoring starts the background health check goroutine.
func (p *WorkerPool) StartHealthMonitoring(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("health monitoring already running")
	}

	if p.healthChecker == nil {
		p.mu.Unlock()
		return fmt.Errorf("health checker not configured")
	}

	p.running = true
	p.stopChan = make(chan struct{})
	p.mu.Unlock()

	go p.healthMonitorLoop(ctx)
	return nil
}

// StopHealthMonitoring stops the background health check goroutine.
func (p *WorkerPool) StopHealthMonitoring() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	close(p.stopChan)
	p.running = false
}

// IsMonitoring returns true if health monitoring is active.
func (p *WorkerPool) IsMonitoring() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// healthMonitorLoop runs the health check loop.
func (p *WorkerPool) healthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	// Run initial health check
	p.RunHealthChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			p.mu.Lock()
			p.running = false
			p.mu.Unlock()
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.RunHealthChecks(ctx)
		}
	}
}

// RunHealthChecks performs health checks on all workers.
func (p *WorkerPool) RunHealthChecks(ctx context.Context) {
	p.mu.Lock()
	checker := p.healthChecker
	workerIDs := make([]string, 0, len(p.workers))
	for id := range p.workers {
		workerIDs = append(workerIDs, id)
	}
	p.mu.Unlock()

	if checker == nil {
		return
	}

	// Create a timeout context for health checks
	checkCtx, cancel := context.WithTimeout(ctx, p.config.HealthCheckTimeout)
	defer cancel()

	// Check each worker
	for _, workerID := range workerIDs {
		p.checkWorkerHealth(checkCtx, workerID, checker)
	}

	p.mu.Lock()
	p.lastHealthCheck = time.Now()
	p.mu.Unlock()
}

// checkWorkerHealth checks the health of a single worker.
func (p *WorkerPool) checkWorkerHealth(ctx context.Context, workerID string, checker HealthChecker) {
	healthy, responseTimeMs, err := checker(ctx, workerID)

	p.mu.Lock()
	defer p.mu.Unlock()

	health, exists := p.health[workerID]
	if !exists {
		return // Worker was removed
	}

	oldHealth := *health // Copy for event
	now := time.Now()

	health.LastCheckTime = now
	health.ResponseTimeMs = responseTimeMs

	if healthy && err == nil {
		// Worker is healthy
		health.ConsecutiveFailures = 0
		health.ErrorMessage = ""
		health.LastHealthyTime = now

		// Check response time for degradation
		if responseTimeMs > p.config.MaxResponseTimeMs {
			health.Status = HealthStatusDegraded
			health.ErrorMessage = fmt.Sprintf("slow response: %dms > %dms", responseTimeMs, p.config.MaxResponseTimeMs)
		} else {
			health.Status = HealthStatusHealthy
		}
	} else {
		// Health check failed
		health.ConsecutiveFailures++
		health.TotalFailures++
		if err != nil {
			health.ErrorMessage = err.Error()
		} else {
			health.ErrorMessage = "health check returned unhealthy"
		}

		// Determine status based on consecutive failures
		if health.ConsecutiveFailures >= p.config.UnhealthyThreshold {
			health.Status = HealthStatusUnhealthy
		} else if health.ConsecutiveFailures >= p.config.DegradedThreshold {
			health.Status = HealthStatusDegraded
		}

		p.emitEventLocked(&WorkerPoolEvent{
			Type:      EventHealthCheckFailed,
			WorkerID:  workerID,
			Timestamp: now,
			Details:   fmt.Sprintf("consecutive failures: %d, error: %s", health.ConsecutiveFailures, health.ErrorMessage),
		})
	}

	// Emit health change event if status changed
	if oldHealth.Status != health.Status {
		p.emitEventLocked(&WorkerPoolEvent{
			Type:      EventWorkerHealthChanged,
			WorkerID:  workerID,
			Timestamp: now,
			Details:   fmt.Sprintf("health changed: %s -> %s", oldHealth.Status, health.Status),
			OldHealth: &oldHealth,
			NewHealth: health,
		})

		// Auto-remove unhealthy workers if configured
		if p.config.AutoRemoveUnhealthy && health.Status == HealthStatusUnhealthy {
			_ = p.removeWorkerLocked(workerID, "auto-removed due to unhealthy status")
		}
	}
}

// CheckWorkerHealthNow performs an immediate health check on a specific worker.
func (p *WorkerPool) CheckWorkerHealthNow(ctx context.Context, workerID string) (*WorkerHealth, error) {
	p.mu.RLock()
	checker := p.healthChecker
	_, exists := p.workers[workerID]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("worker %q not found in pool", workerID)
	}

	if checker == nil {
		return nil, fmt.Errorf("health checker not configured")
	}

	checkCtx, cancel := context.WithTimeout(ctx, p.config.HealthCheckTimeout)
	defer cancel()

	p.checkWorkerHealth(checkCtx, workerID, checker)

	health, exists := p.GetWorkerHealth(workerID)
	if !exists {
		return nil, fmt.Errorf("worker %q not found in pool", workerID)
	}
	return health, nil
}

// MarkWorkerHealthy manually marks a worker as healthy.
func (p *WorkerPool) MarkWorkerHealthy(workerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	health, exists := p.health[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found in pool", workerID)
	}

	oldHealth := *health
	now := time.Now()

	health.Status = HealthStatusHealthy
	health.ConsecutiveFailures = 0
	health.ErrorMessage = ""
	health.LastHealthyTime = now
	health.LastCheckTime = now

	if oldHealth.Status != health.Status {
		p.emitEventLocked(&WorkerPoolEvent{
			Type:      EventWorkerHealthChanged,
			WorkerID:  workerID,
			Timestamp: now,
			Details:   fmt.Sprintf("manually marked healthy (was: %s)", oldHealth.Status),
			OldHealth: &oldHealth,
			NewHealth: health,
		})
	}

	return nil
}

// MarkWorkerUnhealthy manually marks a worker as unhealthy.
func (p *WorkerPool) MarkWorkerUnhealthy(workerID string, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	health, exists := p.health[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found in pool", workerID)
	}

	oldHealth := *health
	now := time.Now()

	health.Status = HealthStatusUnhealthy
	health.ErrorMessage = reason
	health.LastCheckTime = now

	if oldHealth.Status != health.Status {
		p.emitEventLocked(&WorkerPoolEvent{
			Type:      EventWorkerHealthChanged,
			WorkerID:  workerID,
			Timestamp: now,
			Details:   fmt.Sprintf("manually marked unhealthy: %s", reason),
			OldHealth: &oldHealth,
			NewHealth: health,
		})

		// Auto-remove if configured
		if p.config.AutoRemoveUnhealthy {
			_ = p.removeWorkerLocked(workerID, "auto-removed after manual unhealthy mark")
		}
	}

	return nil
}

// ReplaceWorker replaces an existing worker with a new one.
func (p *WorkerPool) ReplaceWorker(oldWorkerID string, newWorker *HierarchyNode) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.workers[oldWorkerID]; !exists {
		return fmt.Errorf("worker %q not found in pool", oldWorkerID)
	}

	if newWorker == nil {
		return fmt.Errorf("new worker cannot be nil")
	}

	if newWorker.AgentID == "" {
		return fmt.Errorf("new worker ID cannot be empty")
	}

	if newWorker.AgentID == oldWorkerID {
		return fmt.Errorf("new worker has same ID as old worker")
	}

	// Remove old worker (without emitting event for removal)
	delete(p.workers, oldWorkerID)
	delete(p.health, oldWorkerID)

	// Add new worker
	p.workers[newWorker.AgentID] = newWorker
	now := time.Now()
	p.health[newWorker.AgentID] = &WorkerHealth{
		WorkerID:        newWorker.AgentID,
		Status:          HealthStatusUnknown,
		LastCheckTime:   time.Time{},
		LastHealthyTime: time.Time{},
		Metadata:        make(map[string]interface{}),
	}

	p.emitEventLocked(&WorkerPoolEvent{
		Type:      EventWorkerAdded,
		WorkerID:  newWorker.AgentID,
		Timestamp: now,
		Details:   fmt.Sprintf("replaced worker %s (size: %d/%d)", oldWorkerID, len(p.workers), p.config.MaxSize),
	})

	return nil
}

// UpdateConfig updates the pool configuration.
// Note: This does not affect currently running health monitoring; restart is required.
func (p *WorkerPool) UpdateConfig(config PoolConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config.MaxSize <= 0 {
		return fmt.Errorf("max size must be positive")
	}

	if config.MaxSize < len(p.workers) {
		return fmt.Errorf("cannot reduce max size below current worker count (%d)", len(p.workers))
	}

	p.config = config
	return nil
}

// GetAllHealth returns health status for all workers.
func (p *WorkerPool) GetAllHealth() map[string]*WorkerHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*WorkerHealth, len(p.health))
	for id, h := range p.health {
		healthCopy := *h
		result[id] = &healthCopy
	}
	return result
}

// emitEventLocked emits a pool event (must be called with lock held).
func (p *WorkerPool) emitEventLocked(event *WorkerPoolEvent) {
	if p.eventHandler != nil {
		// Call handler asynchronously to avoid blocking
		handler := p.eventHandler
		go handler(event)
	}
}

// DefaultHealthChecker creates a simple health checker that always returns healthy.
// This is useful for testing or when no actual health check is needed.
func DefaultHealthChecker() HealthChecker {
	return func(ctx context.Context, workerID string) (bool, int64, error) {
		// Simulate minimal response time
		return true, 1, nil
	}
}

// HeartbeatHealthChecker creates a health checker based on worker heartbeat/last-seen time.
// Workers are considered unhealthy if they haven't been seen for longer than maxStaleTime.
func HeartbeatHealthChecker(getLastSeen func(workerID string) (time.Time, bool), maxStaleTime time.Duration) HealthChecker {
	return func(ctx context.Context, workerID string) (bool, int64, error) {
		lastSeen, exists := getLastSeen(workerID)
		if !exists {
			return false, 0, fmt.Errorf("worker not found")
		}

		staleDuration := time.Since(lastSeen)
		responseTimeMs := staleDuration.Milliseconds()

		if staleDuration > maxStaleTime {
			return false, responseTimeMs, fmt.Errorf("worker stale for %v (threshold: %v)", staleDuration, maxStaleTime)
		}

		return true, responseTimeMs, nil
	}
}
