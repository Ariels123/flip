package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Agents
		agents := core.NewBaseCollection("agents")
		agents.Fields.Add(&core.TextField{Name: "agent_id", Required: true})
		agents.Fields.Add(&core.SelectField{Name: "status", Values: []string{"online", "offline", "busy", "idle"}})
		agents.Fields.Add(&core.SelectField{Name: "backend", Values: []string{"claude", "gemini", "antigravity", "custom"}})
		agents.Fields.Add(&core.JSONField{Name: "capabilities"})
		agents.Fields.Add(&core.DateField{Name: "last_seen"})
		agents.Fields.Add(&core.JSONField{Name: "metadata"})
		agents.Fields.Add(&core.JSONField{Name: "metadata"})
		agents.AddIndex("idx_agents_agent_id", true, "agent_id", "")
		agents.ListRule = types.Pointer("")
		agents.ViewRule = types.Pointer("")
		agents.CreateRule = types.Pointer("")
		agents.UpdateRule = types.Pointer("")
		if err := app.Save(agents); err != nil {
			return err
		}
        
        // Tasks
        tasks := core.NewBaseCollection("tasks")
        tasks.Fields.Add(&core.TextField{Name: "task_id", Required: true})
        tasks.Fields.Add(&core.TextField{Name: "title", Required: true})
        tasks.Fields.Add(&core.TextField{Name: "description"})
        tasks.Fields.Add(&core.SelectField{Name: "status", Values: []string{"todo", "in_progress", "done", "failed"}})
        tasks.Fields.Add(&core.NumberField{Name: "priority", Min: types.Pointer(1.0), Max: types.Pointer(5.0)})
        tasks.Fields.Add(&core.TextField{Name: "assignee"})
        tasks.Fields.Add(&core.JSONField{Name: "depends_on"})
        tasks.Fields.Add(&core.NumberField{Name: "progress", Min: types.Pointer(0.0), Max: types.Pointer(100.0)})
        tasks.Fields.Add(&core.TextField{Name: "result"})
        tasks.Fields.Add(&core.DateField{Name: "started_at"})
        tasks.Fields.Add(&core.DateField{Name: "completed_at"})
        tasks.Fields.Add(&core.JSONField{Name: "metadata"})
        tasks.Fields.Add(&core.JSONField{Name: "metadata"})
        tasks.AddIndex("idx_tasks_task_id", true, "task_id", "")
        tasks.ListRule = types.Pointer("") // Public list
        tasks.ViewRule = types.Pointer("") // Public view
        tasks.CreateRule = types.Pointer("") // Public create (or admin only?)
        tasks.UpdateRule = types.Pointer("") // Public update (agents need to update status)
        if err := app.Save(tasks); err != nil {
            return err
        }

        // Signals
        signals := core.NewBaseCollection("signals")
        signals.Fields.Add(&core.TextField{Name: "signal_id", Required: true})
        signals.Fields.Add(&core.TextField{Name: "from_agent", Required: true})
        signals.Fields.Add(&core.TextField{Name: "to_agent", Required: true})
        signals.Fields.Add(&core.SelectField{Name: "signal_type", Values: []string{"message", "ping", "task", "alert", "handoff"}})
        signals.Fields.Add(&core.SelectField{Name: "priority", Values: []string{"low", "normal", "high", "critical"}})
        signals.Fields.Add(&core.TextField{Name: "content"})
        signals.Fields.Add(&core.BoolField{Name: "read"})
        signals.Fields.Add(&core.DateField{Name: "read_at"})
        signals.Fields.Add(&core.DateField{Name: "read_at"})
        signals.AddIndex("idx_signals_signal_id", true, "signal_id", "")
        signals.ListRule = types.Pointer("")
        signals.ViewRule = types.Pointer("")
        signals.CreateRule = types.Pointer("")
        signals.UpdateRule = types.Pointer("")
        if err := app.Save(signals); err != nil {
            return err
        }
        
        // Jobs
        jobs := core.NewBaseCollection("jobs")
        jobs.Fields.Add(&core.TextField{Name: "name", Required: true})
        jobs.Fields.Add(&core.TextField{Name: "description"})
        jobs.Fields.Add(&core.TextField{Name: "cron", Required: true})
        jobs.Fields.Add(&core.TextField{Name: "handler", Required: true})
        jobs.Fields.Add(&core.BoolField{Name: "enabled"})
        jobs.Fields.Add(&core.DateField{Name: "last_run"})
        jobs.Fields.Add(&core.DateField{Name: "next_run"})
        jobs.Fields.Add(&core.SelectField{Name: "last_result", Values: []string{"success", "failed", "running"}})
        jobs.Fields.Add(&core.JSONField{Name: "metadata"})
        jobs.Fields.Add(&core.JSONField{Name: "metadata"})
        jobs.AddIndex("idx_jobs_name", true, "name", "")
        jobs.ListRule = types.Pointer("")
        jobs.ViewRule = types.Pointer("")
        jobs.CreateRule = types.Pointer("")
        jobs.UpdateRule = types.Pointer("")
        if err := app.Save(jobs); err != nil {
            return err
        }
        
        // Events
        events := core.NewBaseCollection("events")
        events.Fields.Add(&core.TextField{Name: "event_type", Required: true})
        events.Fields.Add(&core.TextField{Name: "agent_id"})
        events.Fields.Add(&core.TextField{Name: "task_id"})
        events.Fields.Add(&core.JSONField{Name: "details"})
        events.Fields.Add(&core.NumberField{Name: "cost"})
        events.Fields.Add(&core.NumberField{Name: "tokens"})
        events.ListRule = types.Pointer("")
        events.ViewRule = types.Pointer("")
        events.CreateRule = types.Pointer("")
        events.UpdateRule = types.Pointer("")
        return app.Save(events)
        
	}, func(app core.App) error {
        // Drop logic
		for _, name := range []string{"events", "jobs", "signals", "tasks", "agents"} {
            col, _ := app.FindCollectionByNameOrId(name)
            if col != nil {
                app.Delete(col)
            }
        }
		return nil
	})
}
