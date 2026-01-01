package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Alerts collection for tracking system alerts
		alerts := core.NewBaseCollection("alerts")

		alerts.Fields.Add(&core.TextField{Name: "alert_name", Required: true})
		alerts.Fields.Add(&core.TextField{Name: "severity", Required: true})
		alerts.Fields.Add(&core.TextField{Name: "message", Required: true})
		alerts.Fields.Add(&core.NumberField{Name: "metric_value", Required: true})
		alerts.Fields.Add(&core.NumberField{Name: "threshold", Required: true})
		alerts.Fields.Add(&core.TextField{Name: "state", Required: true})
		alerts.Fields.Add(&core.DateField{Name: "fired_at", Required: true})
		alerts.Fields.Add(&core.DateField{Name: "resolved_at"})
		alerts.Fields.Add(&core.BoolField{Name: "notification_sent"})
		alerts.Fields.Add(&core.JSONField{Name: "metadata"})

		// Indexes for efficient queries
		alerts.AddIndex("idx_alerts_name_state", false, "alert_name", "state")
		alerts.AddIndex("idx_alerts_state", false, "state", "")
		alerts.AddIndex("idx_alerts_fired_at", false, "fired_at", "")
		alerts.AddIndex("idx_alerts_resolved_at", false, "resolved_at", "")

		// Public access rules (adjust based on security requirements)
		alerts.ListRule = types.Pointer("")
		alerts.ViewRule = types.Pointer("")
		alerts.CreateRule = types.Pointer("")
		alerts.UpdateRule = types.Pointer("")
		alerts.DeleteRule = types.Pointer("")

		if err := app.Save(alerts); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete alerts collection
		collection, err := app.FindCollectionByNameOrId("alerts")
		if err != nil {
			return nil // Already deleted
		}
		return app.Delete(collection)
	})
}
