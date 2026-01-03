package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Budgets collection
		budgets := core.NewBaseCollection("budgets")
		budgets.Fields.Add(&core.TextField{Name: "name", Required: true})
		budgets.Fields.Add(&core.TextField{Name: "scope", Required: true})
		budgets.Fields.Add(&core.TextField{Name: "scope_id", Required: true})
		budgets.Fields.Add(&core.NumberField{Name: "limit_usd", Required: true, Min: types.Pointer(0.0)})
		budgets.Fields.Add(&core.TextField{Name: "reset_period", Required: true})
		budgets.Fields.Add(&core.NumberField{Name: "warning_pct", Required: true, Min: types.Pointer(0.0), Max: types.Pointer(100.0)})
		budgets.Fields.Add(&core.NumberField{Name: "consumed_usd", Required: true, Min: types.Pointer(0.0)})
		budgets.Fields.Add(&core.TextField{Name: "status", Required: true})
		budgets.Fields.Add(&core.DateField{Name: "last_reset_at", Required: true})
		budgets.Fields.Add(&core.DateField{Name: "next_reset_at", Required: true})
		budgets.Fields.Add(&core.BoolField{Name: "enabled"})

		// Indexes
		budgets.AddIndex("idx_budgets_scope", false, "scope", "scope_id")

		// Permissions (open for now, adjust as needed)
		budgets.ListRule = types.Pointer("")
		budgets.ViewRule = types.Pointer("")
		budgets.CreateRule = types.Pointer("")
		budgets.UpdateRule = types.Pointer("")
		budgets.DeleteRule = types.Pointer("")

		if err := app.Save(budgets); err != nil {
			return err
		}

		// Budget Consumptions collection
		consumptions := core.NewBaseCollection("budget_consumptions")
		consumptions.Fields.Add(&core.RelationField{
			Name: "budget_id", 
			CollectionId: budgets.Id,
			CascadeDelete: true,
			MaxSelect: 1,
			Required: true,
		})
		consumptions.Fields.Add(&core.TextField{Name: "agent_id"})
		consumptions.Fields.Add(&core.TextField{Name: "model"})
		consumptions.Fields.Add(&core.TextField{Name: "task_id"})
		consumptions.Fields.Add(&core.NumberField{Name: "amount_usd", Required: true, Min: types.Pointer(0.0)})
		consumptions.Fields.Add(&core.NumberField{Name: "input_tokens", Min: types.Pointer(0.0)})
		consumptions.Fields.Add(&core.NumberField{Name: "output_tokens", Min: types.Pointer(0.0)})
		consumptions.Fields.Add(&core.DateField{Name: "timestamp", Required: true})
		consumptions.Fields.Add(&core.DateField{Name: "period_start", Required: true})

		// Indexes
		consumptions.AddIndex("idx_consumptions_budget_time", false, "budget_id", "timestamp")
		consumptions.AddIndex("idx_consumptions_budget_period", false, "budget_id", "period_start")

		// Permissions
		consumptions.ListRule = types.Pointer("")
		consumptions.ViewRule = types.Pointer("")
		consumptions.CreateRule = types.Pointer("")
		consumptions.UpdateRule = types.Pointer("")
		consumptions.DeleteRule = types.Pointer("")

		if err := app.Save(consumptions); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Rollback
		consumptions, err := app.FindCollectionByNameOrId("budget_consumptions")
		if err == nil {
			app.Delete(consumptions)
		}
		
		budgets, err := app.FindCollectionByNameOrId("budgets")
		if err == nil {
			app.Delete(budgets)
		}
		return nil
	})
}
