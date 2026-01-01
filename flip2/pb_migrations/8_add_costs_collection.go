package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Costs collection for tracking LLM API costs
		costs := core.NewBaseCollection("costs")
		costs.Fields.Add(&core.TextField{Name: "agent_id", Required: true})
		costs.Fields.Add(&core.TextField{Name: "model", Required: true})
		costs.Fields.Add(&core.NumberField{Name: "input_tokens", Required: true, Min: types.Pointer(0.0)})
		costs.Fields.Add(&core.NumberField{Name: "output_tokens", Required: true, Min: types.Pointer(0.0)})
		costs.Fields.Add(&core.NumberField{Name: "cost_usd", Required: true, Min: types.Pointer(0.0)})
		costs.Fields.Add(&core.TextField{Name: "task_id"})
		costs.Fields.Add(&core.DateField{Name: "timestamp", Required: true})

		// Indexes for efficient queries
		costs.AddIndex("idx_costs_agent_timestamp", false, "agent_id", "timestamp")
		costs.AddIndex("idx_costs_model_timestamp", false, "model", "timestamp")
		costs.AddIndex("idx_costs_timestamp", false, "timestamp", "")

		// Public access rules (adjust based on security requirements)
		costs.ListRule = types.Pointer("")
		costs.ViewRule = types.Pointer("")
		costs.CreateRule = types.Pointer("")
		costs.UpdateRule = types.Pointer("")
		costs.DeleteRule = types.Pointer("")

		if err := app.Save(costs); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete costs collection
		collection, err := app.FindCollectionByNameOrId("costs")
		if err != nil {
			return nil // Already deleted
		}
		return app.Delete(collection)
	})
}
