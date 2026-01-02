package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Vibe Scorecard collection for quality evaluation of task results
		vibescore := core.NewBaseCollection("vibescore")

		// Core evaluation dimensions (0-10 scale)
		vibescore.Fields.Add(&core.NumberField{
			Name:     "correctness",
			Required: true,
			Min:      types.Pointer(0.0),
			Max:      types.Pointer(10.0),
		})
		vibescore.Fields.Add(&core.NumberField{
			Name:     "efficiency",
			Required: true,
			Min:      types.Pointer(0.0),
			Max:      types.Pointer(10.0),
		})
		vibescore.Fields.Add(&core.NumberField{
			Name:     "maintainability",
			Required: true,
			Min:      types.Pointer(0.0),
			Max:      types.Pointer(10.0),
		})
		vibescore.Fields.Add(&core.NumberField{
			Name:     "security",
			Required: true,
			Min:      types.Pointer(0.0),
			Max:      types.Pointer(10.0),
		})

		// Calculated overall score (0-10, average of dimensions)
		vibescore.Fields.Add(&core.NumberField{
			Name:     "overall_score",
			Required: true,
			Min:      types.Pointer(0.0),
			Max:      types.Pointer(10.0),
		})

		// Metadata: task and evaluator information
		vibescore.Fields.Add(&core.TextField{
			Name:     "task_id",
			Required: true,
		})
		vibescore.Fields.Add(&core.TextField{
			Name:     "evaluator",
			Required: true,
		}) // "claude", "gemini", "antigravity", etc.
		vibescore.Fields.Add(&core.TextField{
			Name: "evaluator_model",
		}) // e.g., "claude-opus-4-5", "gemini-2.5-pro"
		vibescore.Fields.Add(&core.DateField{
			Name:     "evaluated_at",
			Required: true,
		})

		// Detailed feedback
		vibescore.Fields.Add(&core.TextField{
			Name: "correctness_feedback",
		})
		vibescore.Fields.Add(&core.TextField{
			Name: "efficiency_feedback",
		})
		vibescore.Fields.Add(&core.TextField{
			Name: "maintainability_feedback",
		})
		vibescore.Fields.Add(&core.TextField{
			Name: "security_feedback",
		})

		// Overall summary feedback
		vibescore.Fields.Add(&core.TextField{
			Name: "summary_feedback",
		})

		// Improvement suggestions
		vibescore.Fields.Add(&core.JSONField{
			Name: "improvement_suggestions",
		}) // Array of suggested improvements

		// Pass/Fail status based on threshold
		vibescore.Fields.Add(&core.SelectField{
			Name:   "status",
			Values: []string{"pass", "fail", "needs_review"},
		}) // "pass" (>= threshold), "fail" (< threshold), "needs_review" (requires human evaluation)

		// Quality threshold used for this evaluation
		vibescore.Fields.Add(&core.NumberField{
			Name: "quality_threshold",
			Min:  types.Pointer(0.0),
			Max:  types.Pointer(10.0),
		})

		// Retry information (if this is a retry evaluation)
		vibescore.Fields.Add(&core.NumberField{
			Name: "retry_count",
			Min:  types.Pointer(0.0),
		})
		vibescore.Fields.Add(&core.TextField{
			Name: "previous_score_id",
		})

		// Additional metadata
		vibescore.Fields.Add(&core.JSONField{
			Name: "metadata",
		})

		// Indexes for efficient queries
		vibescore.AddIndex("idx_vibescore_task_id", false, "task_id", "")
		vibescore.AddIndex("idx_vibescore_evaluator", false, "evaluator", "")
		vibescore.AddIndex("idx_vibescore_evaluated_at", false, "evaluated_at", "")
		vibescore.AddIndex("idx_vibescore_overall_score", false, "overall_score", "")
		vibescore.AddIndex("idx_vibescore_status", false, "status", "")
		vibescore.AddIndex("idx_vibescore_task_evaluated", false, "task_id", "evaluated_at")

		// Access rules (adjust based on security requirements)
		vibescore.ListRule = types.Pointer("")
		vibescore.ViewRule = types.Pointer("")
		vibescore.CreateRule = types.Pointer("")
		vibescore.UpdateRule = types.Pointer("")
		vibescore.DeleteRule = types.Pointer("")

		if err := app.Save(vibescore); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete vibescore collection
		collection, err := app.FindCollectionByNameOrId("vibescore")
		if err != nil {
			return nil // Already deleted
		}
		return app.Delete(collection)
	})
}
