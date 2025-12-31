package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"flip2/internal/auth"
	"flip2/internal/sync"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// RESTServer handles HTTPS REST API endpoints
type RESTServer struct {
	logger          *slog.Logger
	jwtManager      *auth.JWTManager
	store           sync.RecordStore
	replicator      *sync.Replicator // Optional: set after sync manager is created
	bootstrapAPIKey string           // Required for JWT token generation
}

// NewRESTServer creates a new REST API server
func NewRESTServer(logger *slog.Logger, jwtManager *auth.JWTManager, store sync.RecordStore, bootstrapAPIKey string) *RESTServer {
	return &RESTServer{
		logger:          logger,
		jwtManager:      jwtManager,
		store:           store,
		replicator:      nil, // Will be set later by SetReplicator
		bootstrapAPIKey: bootstrapAPIKey,
	}
}

// SetReplicator sets the replicator for the REST server (called after sync manager is created)
func (s *RESTServer) SetReplicator(replicator *sync.Replicator) {
	s.replicator = replicator
	s.logger.Info("Replicator configured for REST API")
}

// RegisterRoutes registers all REST API routes with PocketBase router
func (s *RESTServer) RegisterRoutes(r *router.Router[*core.RequestEvent]) {
	// Public endpoints (no auth required)
	// Note: /api/health is already registered by daemon
	r.POST("/api/auth/token", s.handleGetToken)

	// Sync endpoints - protected by API key (PocketBase middleware handles this)
	r.GET("/api/vectorclock", s.handleGetVectorClock)
	r.GET("/api/records", s.handleGetRecords)
	r.POST("/api/records", s.handlePostRecords)
}

// handleGetToken generates a JWT token for a node (PocketBase handler)
// SECURITY FIX: Requires bootstrap API key authentication
func (s *RESTServer) handleGetToken(e *core.RequestEvent) error {
	// SECURITY: Validate bootstrap API key before generating tokens
	apiKey := e.Request.Header.Get("X-API-Key")
	if apiKey == "" {
		s.logger.Warn("Token request rejected: missing X-API-Key header",
			"remote_addr", e.Request.RemoteAddr)
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "X-API-Key header required"})
	}

	if apiKey != s.bootstrapAPIKey {
		s.logger.Warn("Token request rejected: invalid API key",
			"remote_addr", e.Request.RemoteAddr)
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid API key"})
	}

	var req struct {
		NodeID      string   `json:"node_id"`
		NodeType    string   `json:"node_type"`
		Permissions []string `json:"permissions"`
	}

	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// Validate required fields
	if req.NodeID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "node_id required"})
	}
	if req.NodeType == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "node_type required"})
	}

	token, err := s.jwtManager.GenerateToken(req.NodeID, req.NodeType, req.Permissions)
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	s.logger.Info("JWT token generated", "node_id", req.NodeID, "node_type", req.NodeType)
	return e.JSON(http.StatusOK, map[string]string{"token": token})
}

// handleGetVectorClock returns the current vector clock (PocketBase handler)
func (s *RESTServer) handleGetVectorClock(e *core.RequestEvent) error {
	var vc *sync.VectorClock
	if s.replicator != nil {
		vc = s.replicator.GetVectorClock()
		s.logger.Debug("Returning vector clock", "clock", vc.GetAll())
	} else {
		vc = sync.NewVectorClock("unknown")
		s.logger.Warn("Replicator not initialized, returning empty vector clock")
	}

	return e.JSON(http.StatusOK, vc)
}

// handleGetRecords fetches records since a given vector clock (PocketBase handler)
func (s *RESTServer) handleGetRecords(e *core.RequestEvent) error {
	sinceParam := e.Request.URL.Query().Get("since")

	var sinceVC *sync.VectorClock
	if sinceParam != "" {
		sinceVC = &sync.VectorClock{}
		if err := json.Unmarshal([]byte(sinceParam), sinceVC); err != nil {
			s.logger.Warn("Failed to parse 'since' parameter", "error", err)
			sinceVC = nil
		}
	}

	records, err := s.store.GetRecordsSince(sinceVC)
	if err != nil {
		s.logger.Error("Failed to get records", "error", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get records"})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"records": records,
		"count":   len(records),
	})
}

// handlePostRecords accepts and stores incoming records (PocketBase handler)
func (s *RESTServer) handlePostRecords(e *core.RequestEvent) error {
	var req struct {
		Records []*sync.Record `json:"records"`
	}

	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	applied := 0
	for _, record := range req.Records {
		if err := s.store.ApplyRecord(record); err != nil {
			s.logger.Warn("Failed to apply record", "record_id", record.ID, "error", err)
		} else {
			applied++
		}
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"applied": applied,
		"total":   len(req.Records),
	})
}

