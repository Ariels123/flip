package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// 1. Rename existing 'agents' to 'agents_old' if it exists
		oldAgents, err := app.FindCollectionByNameOrId("agents")
		if err == nil && oldAgents.Name == "agents" {
			oldAgents.Name = "agents_old"
			if err := app.Save(oldAgents); err != nil {
				return err
			}
		}

		// 2. Create new 'agents' Auth Collection
		// Check if it already exists (idempotency)
		_, err = app.FindCollectionByNameOrId("agents")
		if err != nil {
			newAgents := core.NewAuthCollection("agents")

			// Re-add fields from original schema + daemon log path
			newAgents.Fields.Add(&core.TextField{Name: "agent_id", Required: true})
			newAgents.Fields.Add(&core.SelectField{Name: "status", Values: []string{"online", "offline", "busy", "idle"}})
			newAgents.Fields.Add(&core.SelectField{Name: "backend", Values: []string{"claude", "gemini", "antigravity", "custom"}})
			newAgents.Fields.Add(&core.JSONField{Name: "capabilities"})
			newAgents.Fields.Add(&core.DateField{Name: "last_seen"})
			newAgents.Fields.Add(&core.JSONField{Name: "metadata"})
			newAgents.Fields.Add(&core.TextField{Name: "daemon_log_path"})

			// Add Index
			newAgents.AddIndex("idx_agents_agent_id", true, "agent_id", "")

			// Rules - allow authenticated agents to list/view each other
			newAgents.ListRule = types.Pointer("@request.auth.id != ''")
			newAgents.ViewRule = types.Pointer("@request.auth.id != ''")
			// Self-update only
			newAgents.UpdateRule = types.Pointer("id = @request.auth.id")

			if err := app.Save(newAgents); err != nil {
				return err
			}

			// 3. Migrate Data from agents_old
			oldAgentsRenamed, _ := app.FindCollectionByNameOrId("agents_old")
			if oldAgentsRenamed != nil {
				records, err := app.FindRecordsByFilter("agents_old", "id != ''", "", 0, 0)
				if err == nil {
					newAgentsColl, _ := app.FindCollectionByNameOrId("agents")
					for _, r := range records {
						newRecord := core.NewRecord(newAgentsColl)

						// Copy fields
						newRecord.Set("agent_id", r.GetString("agent_id"))
						newRecord.Set("status", r.GetString("status"))
						newRecord.Set("backend", r.GetString("backend"))
						newRecord.Set("capabilities", r.Get("capabilities"))
						newRecord.Set("last_seen", r.GetDateTime("last_seen"))
						newRecord.Set("metadata", r.Get("metadata"))
						newRecord.Set("daemon_log_path", r.GetString("daemon_log_path"))

						// Set Auth Defaults - use agent_id as username and initial password
						agentId := r.GetString("agent_id")
						if agentId != "" {
							newRecord.Set("username", agentId)
							newRecord.Set("password", agentId)
							newRecord.Set("verified", true)
						}

						// Save (ignore error to continue best-effort)
						_ = app.Save(newRecord)
					}
				}
			}
		}

		// Get the agents collection for relations
		newAgents, err := app.FindCollectionByNameOrId("agents")
		if err != nil {
			return err
		}

		// 4. Update 'tasks' rules
		tasks, err := app.FindCollectionByNameOrId("tasks")
		if err == nil {
			// Add created_by if missing
			if tasks.Fields.GetByName("created_by") == nil {
				tasks.Fields.Add(&core.RelationField{
					Name:         "created_by",
					CollectionId: newAgents.Id,
					MaxSelect:    1,
				})
			}

			// Update Rules
			tasks.ListRule = types.Pointer("@request.auth.id != '' && (assignee = @request.auth.agent_id || created_by = @request.auth.id)")
			tasks.ViewRule = types.Pointer("@request.auth.id != '' && (assignee = @request.auth.agent_id || created_by = @request.auth.id)")
			tasks.CreateRule = types.Pointer("@request.auth.id != ''")
			tasks.UpdateRule = types.Pointer("@request.auth.id != '' && (assignee = @request.auth.agent_id || created_by = @request.auth.id)")

			if err := app.Save(tasks); err != nil {
				return err
			}
		}

		// 5. Update 'signals' rules
		signals, err := app.FindCollectionByNameOrId("signals")
		if err == nil {
			signals.ListRule = types.Pointer("@request.auth.id != '' && (to_agent = @request.auth.agent_id || from_agent = @request.auth.agent_id)")
			signals.ViewRule = types.Pointer("@request.auth.id != '' && (to_agent = @request.auth.agent_id || from_agent = @request.auth.agent_id)")
			signals.CreateRule = types.Pointer("@request.auth.id != ''")
			signals.UpdateRule = types.Pointer("@request.auth.id != '' && to_agent = @request.auth.agent_id")

			if err := app.Save(signals); err != nil {
				return err
			}
		}

		return nil
	}, func(app core.App) error {
		return nil
	})
}
