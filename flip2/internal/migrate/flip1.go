package migrate

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	_ "modernc.org/sqlite" // driver for old DB
)

// MigrateFromFlip1 imports data from FLIP v1 SQLite database
func MigrateFromFlip1(oldDBPath string, pb *pocketbase.PocketBase) error {
	logger := slog.With("component", "migrate")
	logger.Info("Starting migration from FLIP v1", "path", oldDBPath)

	oldDB, err := sql.Open("sqlite", oldDBPath)
	if err != nil {
		return fmt.Errorf("failed to open old database: %w", err)
	}
	defer oldDB.Close()

	if err := migrateAgents(oldDB, pb, logger); err != nil {
		return fmt.Errorf("failed to migrate agents: %w", err)
	}

	if err := migrateTasks(oldDB, pb, logger); err != nil {
		return fmt.Errorf("failed to migrate tasks: %w", err)
	}
	
	// signals and events are less critical but good to have
	if err := migrateSignals(oldDB, pb, logger); err != nil {
		logger.Warn("Failed to migrate signals", "error", err)
	}

	logger.Info("Migration completed successfully")
	return nil
}

func migrateAgents(db *sql.DB, pb *pocketbase.PocketBase, logger *slog.Logger) error {
	rows, err := db.Query("SELECT id, status, capabilities, last_seen, backend FROM agents")
	if err != nil {
		// If table doesn't exist, maybe it's fine or different schema?
		logger.Warn("Could not query agents table", "error", err)
		return nil
	}
	defer rows.Close()

	agentsColl, err := pb.FindCollectionByNameOrId("agents")
	if err != nil {
		return fmt.Errorf("agents collection not found: %w", err)
	}

	count := 0
	for rows.Next() {
		var id, status, backend string
		var caps string // json
		var lastSeenUnix int64

		if err := rows.Scan(&id, &status, &caps, &lastSeenUnix, &backend); err != nil {
			logger.Error("Failed to scan agent row", "error", err)
			continue
		}

		// Check if exists
		if _, err := pb.FindRecordById("agents", id); err == nil {
			logger.Info("Agent already exists, skipping", "id", id)
			continue
		}

		record := core.NewRecord(agentsColl)
		record.Id = id // PocketBase allows setting ID on create
		record.Set("agent_id", id) // Also set our custom field if we have one? Schema says "agent_id" unique
		record.Set("status", status)
		record.Set("backend", backend)
		
		var capsMap map[string]interface{}
		if err := json.Unmarshal([]byte(caps), &capsMap); err == nil {
			record.Set("capabilities", capsMap)
		}

		if lastSeenUnix > 0 {
			record.Set("last_seen", time.Unix(lastSeenUnix, 0))
		}

		if err := pb.Save(record); err != nil {
			logger.Error("Failed to save agent", "id", id, "error", err)
		} else {
			count++
		}
	}
	logger.Info("Migrated agents", "count", count)
	return nil
}

func migrateTasks(db *sql.DB, pb *pocketbase.PocketBase, logger *slog.Logger) error {
	// Schema assumption: id, title, description, status, assignee, priority
	rows, err := db.Query("SELECT id, title, description, status, assignee, priority FROM tasks")
	if err != nil {
		logger.Warn("Could not query tasks table", "error", err)
		return nil
	}
	defer rows.Close()

	tasksColl, err := pb.FindCollectionByNameOrId("tasks")
	if err != nil {
		return fmt.Errorf("tasks collection not found: %w", err)
	}

	count := 0
	for rows.Next() {
		var id, title, description, status, assignee string
		var priority int

		if err := rows.Scan(&id, &title, &description, &status, &assignee, &priority); err != nil {
			logger.Error("Failed to scan task row", "error", err)
			continue
		}

		if _, err := pb.FindRecordById("tasks", id); err == nil {
			continue
		}

		record := core.NewRecord(tasksColl)
		record.Id = id
		record.Set("task_id", id)
		record.Set("title", title)
		record.Set("description", description)
		record.Set("status", status)
		
		// Assignee might need to be resolved to record ID if it's a relation
		// In our schema, 'assignee' is a relation to 'agents'.
		// If flip1 used same IDs for agents, then we can use ID directly if PocketBase allows relation by ID string.
		// Yes, relation stores the record ID.
		if assignee != "" {
			record.Set("assignee", assignee)
		}
		
		record.Set("priority", priority)

		if err := pb.Save(record); err != nil {
			logger.Error("Failed to save task", "id", id, "error", err)
		} else {
			count++
		}
	}
	logger.Info("Migrated tasks", "count", count)
	return nil
}

func migrateSignals(db *sql.DB, pb *pocketbase.PocketBase, logger *slog.Logger) error {
	rows, err := db.Query("SELECT id, from_agent, to_agent, type, content, read FROM signals")
	if err != nil {
		logger.Warn("Could not query signals table", "error", err)
		return nil
	}
	defer rows.Close()

	signalsColl, err := pb.FindCollectionByNameOrId("signals")
	if err != nil {
		return fmt.Errorf("signals collection not found: %w", err)
	}

	count := 0
	for rows.Next() {
		var id, from, to, sigType, content string
		var read bool

		if err := rows.Scan(&id, &from, &to, &sigType, &content, &read); err != nil {
			continue
		}
		
		if _, err := pb.FindRecordById("signals", id); err == nil {
			continue
		}

		record := core.NewRecord(signalsColl)
		record.Id = id
		record.Set("signal_id", id)
		record.Set("from_agent", from)
		record.Set("to_agent", to)
		record.Set("signal_type", sigType)
		record.Set("content", content)
		record.Set("read", read)

		if err := pb.Save(record); err != nil {
			logger.Error("Failed to save signal", "id", id, "error", err)
		} else {
			count++
		}
	}
	logger.Info("Migrated signals", "count", count)
	return nil
}
