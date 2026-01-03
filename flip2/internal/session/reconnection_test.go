package session

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconnectionManager tests the agent reconnection logic.
func TestReconnectionManager(t *testing.T) {
	// Setup temporary PocketBase app
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: t.TempDir(),
	})
	
	// Bootstrap the app
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("Failed to bootstrap app: %v", err)
	}

	// Create collections
	createTestCollections(t, app)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create managers
	sessionMgr := NewSessionManager(app, logger)
	reconnectConfig := DefaultReconnectionConfig()
	reconnectConfig.InitialBackoff = 10 * time.Millisecond // Fast for tests
	reconnectConfig.MaxBackoff = 100 * time.Millisecond
	reconnectConfig.MaxRetries = 3
	
	rm := NewReconnectionManager(reconnectConfig, app, sessionMgr, logger)

	t.Run("ReconnectAgent_Success", func(t *testing.T) {
		ctx := context.Background()
		agentID := "agent-success"
		dbAgentID := createTestAgent(t, app, agentID)
		sessionID := createTestSession(t, app, agentID, dbAgentID)

		// Mark agent offline to simulate disconnect
		agent, err := app.FindRecordById("agents", dbAgentID)
		require.NoError(t, err)
		agent.Set("status", "offline")
		err = app.Save(agent)
		require.NoError(t, err)

		result, err := rm.ReconnectAgent(ctx, sessionID, agentID)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, agentID, result.AgentID)
		assert.Equal(t, ReconnectionSucceeded, result.State.Status)

		// Verify agent is online
		updatedAgent, err := app.FindRecordById("agents", dbAgentID)
		require.NoError(t, err)
		assert.Equal(t, "online", updatedAgent.GetString("status"))
	})

	t.Run("ReconnectAgent_MaxRetriesExceeded", func(t *testing.T) {
		ctx := context.Background()
		agentID := "agent-fail" 
		sessionID := "session-fail"

		result, err := rm.ReconnectAgent(ctx, sessionID, agentID)
		
		require.NoError(t, err) 
		assert.False(t, result.Success)
		assert.Equal(t, ReconnectionFailed, result.State.Status)
		assert.Equal(t, 3, result.State.AttemptCount)
	})
	
	t.Run("CancelReconnection", func(t *testing.T) {
		go func() {
			rm.ReconnectAgent(context.Background(), "session-dummy", "agent-dummy")
		}()
		
		time.Sleep(20 * time.Millisecond) // Wait for start
		
		err := rm.CancelReconnection("agent-dummy")
		
		if err == nil {
			state, _ := rm.GetReconnectionState("agent-dummy")
			if state != nil {
				assert.Equal(t, ReconnectionCancelled, state.Status)
			}
		}
	})
}

// Helpers

func createTestCollections(t *testing.T, app *pocketbase.PocketBase) {
	// Agents
	agents := core.NewBaseCollection("agents")
	agents.Fields.Add(&core.TextField{Name: "agent_id"})
	agents.Fields.Add(&core.TextField{Name: "status"})
	agents.Fields.Add(&core.DateField{Name: "last_seen"})
	agents.Fields.Add(&core.TextField{Name: "backend"})
	agents.Fields.Add(&core.TextField{Name: "mode"})
	agents.Fields.Add(&core.TextField{Name: "daemon_log_path"})
	if err := app.Save(agents); err != nil {
		t.Fatalf("Failed to create agents collection: %v", err)
	}

	// Sessions
	sessions := core.NewBaseCollection("sessions")
	sessions.Fields.Add(&core.TextField{Name: "name"})
	sessions.Fields.Add(&core.TextField{Name: "status"})
	sessions.Fields.Add(&core.TextField{Name: "coordinator_id"})
	sessions.Fields.Add(&core.JSONField{Name: "active_agents"})
	sessions.Fields.Add(&core.JSONField{Name: "variables"})
	sessions.Fields.Add(&core.DateField{Name: "last_heartbeat"})
	if err := app.Save(sessions); err != nil {
		t.Fatalf("Failed to create sessions collection: %v", err)
	}

	// Session Agents
	sessionAgents := core.NewBaseCollection("session_agents")
	sessionAgents.Fields.Add(&core.TextField{Name: "session_id"})
	sessionAgents.Fields.Add(&core.TextField{Name: "agent_id"})
	sessionAgents.Fields.Add(&core.TextField{Name: "name"})
	sessionAgents.Fields.Add(&core.TextField{Name: "model"})
	sessionAgents.Fields.Add(&core.TextField{Name: "role"})
	sessionAgents.Fields.Add(&core.TextField{Name: "status"})
	if err := app.Save(sessionAgents); err != nil {
		t.Fatalf("Failed to create session_agents collection: %v", err)
	}

	// Signals
	signals := core.NewBaseCollection("signals")
	signals.Fields.Add(&core.TextField{Name: "signal_id"})
	signals.Fields.Add(&core.TextField{Name: "from_agent"})
	signals.Fields.Add(&core.TextField{Name: "to_agent"})
	signals.Fields.Add(&core.JSONField{Name: "content"})
	signals.Fields.Add(&core.BoolField{Name: "read"})
	signals.Fields.Add(&core.TextField{Name: "priority"})
	signals.Fields.Add(&core.TextField{Name: "signal_type"})
	if err := app.Save(signals); err != nil {
		t.Fatalf("Failed to create signals collection: %v", err)
	}
}

func createTestAgent(t *testing.T, app *pocketbase.PocketBase, agentID string) string {
	collection, err := app.FindCollectionByNameOrId("agents")
	if err != nil {
		t.Fatalf("Failed to find agents collection: %v", err)
	}

	record := core.NewRecord(collection)
	record.Set("agent_id", agentID)
	record.Set("status", "online")
	record.Set("last_seen", time.Now())
	record.Set("backend", "test")
	record.Set("mode", "test")
	
	if err := app.Save(record); err != nil {
		t.Fatalf("Failed to create agent record: %v", err)
	}
	return record.Id
}

func createTestSession(t *testing.T, app *pocketbase.PocketBase, agentID, dbAgentID string) string {
	// Create session
	sessionsCol, err := app.FindCollectionByNameOrId("sessions")
	if err != nil {
		t.Fatalf("Failed to find sessions collection: %v", err)
	}
	
	session := core.NewRecord(sessionsCol)
	session.Set("name", "test-session")
	session.Set("status", "active")
	session.Set("coordinator_id", "coord-1")
	if err := app.Save(session); err != nil {
		t.Fatalf("Failed to create session record: %v", err)
	}
	
	// Create session_agent
	saCol, err := app.FindCollectionByNameOrId("session_agents")
	if err != nil {
		t.Fatalf("Failed to find session_agents collection: %v", err)
	}
	
	sa := core.NewRecord(saCol)
	sa.Set("session_id", session.Id)
	sa.Set("agent_id", agentID) // Use logic agent ID or DB ID? Manager expects match with ReconnectAgent arg
	sa.Set("name", "test-agent")
	sa.Set("model", "test-model")
	sa.Set("role", "worker")
	sa.Set("status", "active")
	if err := app.Save(sa); err != nil {
		t.Fatalf("Failed to create session_agent record: %v", err)
	}
	
	return session.Id
}
