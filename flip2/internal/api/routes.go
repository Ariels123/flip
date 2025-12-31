// Package api provides REST API routes for FLIP2.
package api

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// RegisterAPIRoutes registers all FLIP2 API routes with the PocketBase router
func (h *APIHandlers) RegisterAPIRoutes(r *router.Router[*core.RequestEvent]) {
	// === Agent Endpoints ===
	r.GET("/api/agents", h.HandleListAgents)
	r.GET("/api/agents/{id}", h.HandleGetAgent)
	r.POST("/api/agents", h.HandleRegisterAgent)
	r.POST("/api/agents/{id}/heartbeat", h.HandleAgentHeartbeat)

	// === Task Endpoints ===
	r.POST("/api/tasks", h.HandleSubmitTask)
	r.GET("/api/tasks", h.HandleListTasks)
	r.GET("/api/tasks/{id}", h.HandleGetTask)
	r.DELETE("/api/tasks/{id}", h.HandleCancelTask)

	// === LLM Endpoints ===
	r.POST("/api/llm/invoke", h.HandleInvokeLLM)
	r.GET("/api/llm/backends", h.HandleListBackends)

	// === Signal Endpoints ===
	r.POST("/api/signals", h.HandleSendSignal)
	r.GET("/api/signals", h.HandleGetSignals)

	// === Health Endpoint ===
	r.GET("/api/flip/health", h.HandleHealth)
}
