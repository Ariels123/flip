package sync

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPPeer implements the Peer interface for HTTP-based communication
type HTTPPeer struct {
	id         string
	baseURL    string
	apiKey     string
	nodeID     string // Our node ID for JWT requests
	httpClient *http.Client
	logger     *slog.Logger
	// JWT token cache
	jwtToken  string
	jwtExpiry time.Time
}

// NewHTTPPeer creates a new HTTP peer
func NewHTTPPeer(id, baseURL, apiKey string, logger *slog.Logger) *HTTPPeer {
	if logger == nil {
		logger = slog.Default()
	}
	// Ensure baseURL doesn't have trailing slash
	baseURL = strings.TrimRight(baseURL, "/")

	return &HTTPPeer{
		id:      id,
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Allow self-signed certs for peer sync
				},
			},
		},
		logger: logger,
	}
}

// ID returns the peer's unique identifier
func (p *HTTPPeer) ID() string {
	return p.id
}

// SetNodeID sets our node ID for JWT token requests
func (p *HTTPPeer) SetNodeID(nodeID string) {
	p.nodeID = nodeID
}

// getAuthToken returns a valid auth token, fetching a new JWT if needed
func (p *HTTPPeer) getAuthToken(ctx context.Context) (string, string, error) {
	// If we have a valid JWT token, use it
	if p.jwtToken != "" && time.Now().Before(p.jwtExpiry) {
		return "Bearer", p.jwtToken, nil
	}

	// Try to get a JWT token from the peer
	if p.nodeID != "" {
		token, err := p.fetchJWTToken(ctx)
		if err == nil && token != "" {
			p.jwtToken = token
			p.jwtExpiry = time.Now().Add(23 * time.Hour) // Assume 24h tokens, refresh early
			p.logger.Debug("Got JWT token from peer", "peer_id", p.id)
			return "Bearer", token, nil
		}
		p.logger.Debug("Failed to get JWT token, falling back to API key", "peer_id", p.id, "error", err)
	}

	// Fall back to API key
	if p.apiKey != "" {
		return "X-API-Key", p.apiKey, nil
	}

	return "", "", fmt.Errorf("no authentication method available")
}

// fetchJWTToken requests a JWT token from the peer's auth endpoint
func (p *HTTPPeer) fetchJWTToken(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	payload := struct {
		NodeID      string   `json:"node_id"`
		NodeType    string   `json:"node_type"`
		Permissions []string `json:"permissions"`
	}{
		NodeID:      p.nodeID,
		NodeType:    "daemon",
		Permissions: []string{"sync", "read", "write"},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/auth/token", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return result.Token, nil
}

// setRequestAuth adds authentication to a request
func (p *HTTPPeer) setRequestAuth(ctx context.Context, req *http.Request) error {
	authType, authValue, err := p.getAuthToken(ctx)
	if err != nil {
		return err
	}

	if authType == "Bearer" {
		req.Header.Set("Authorization", "Bearer "+authValue)
	} else {
		req.Header.Set(authType, authValue)
	}
	return nil
}

// GetVectorClock fetches the peer's current vector clock
func (p *HTTPPeer) GetVectorClock(ctx context.Context) (*VectorClock, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/vectorclock", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := p.setRequestAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to set auth: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var vc VectorClock
	if err := json.NewDecoder(resp.Body).Decode(&vc); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	p.logger.Debug("Got vector clock from peer", "peer_id", p.id, "clocks", vc.Clocks)
	return &vc, nil
}

// PushRecords sends records to the peer
func (p *HTTPPeer) PushRecords(ctx context.Context, records []*Record) error {
	if len(records) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	payload := struct {
		Records []*Record `json:"records"`
	}{Records: records}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal records: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/records", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if err := p.setRequestAuth(ctx, req); err != nil {
		return fmt.Errorf("failed to set auth: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	p.logger.Info("Pushed records to peer", "peer_id", p.id, "count", len(records))
	return nil
}

// FetchRecordsSince retrieves records newer than the given vector clock
func (p *HTTPPeer) FetchRecordsSince(ctx context.Context, since *VectorClock) ([]*Record, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reqURL := p.baseURL + "/api/records"
	if since != nil {
		sinceJSON, err := json.Marshal(since)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal since clock: %w", err)
		}
		reqURL += "?since=" + url.QueryEscape(string(sinceJSON))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := p.setRequestAuth(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to set auth: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Records []*Record `json:"records"`
		Count   int       `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	p.logger.Debug("Fetched records from peer", "peer_id", p.id, "count", result.Count)
	return result.Records, nil
}

// IsReachable checks if the peer is accessible
func (p *HTTPPeer) IsReachable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/health", nil)
	if err != nil {
		p.logger.Debug("Failed to create health check request", "peer_id", p.id, "error", err)
		return false
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logger.Debug("Health check failed", "peer_id", p.id, "error", err)
		return false
	}
	defer resp.Body.Close()

	reachable := resp.StatusCode >= 200 && resp.StatusCode < 300
	p.logger.Debug("Peer health check", "peer_id", p.id, "reachable", reachable, "status", resp.StatusCode)
	return reachable
}
