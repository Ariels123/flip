package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Code Reviews collection for automated code review results
		codeReviews := core.NewBaseCollection("code_reviews")

		codeReviews.Fields.Add(&core.TextField{Name: "review_type", Required: true})     // "periodic", "on_demand", "pre_commit"
		codeReviews.Fields.Add(&core.TextField{Name: "reviewer", Required: true})         // "gemini", "claude", etc.
		codeReviews.Fields.Add(&core.TextField{Name: "scope", Required: true})            // "full", "recent_changes", "file"
		codeReviews.Fields.Add(&core.TextField{Name: "target"})                           // File path or commit range
		codeReviews.Fields.Add(&core.SelectField{
			Name:     "status",
			Required: true,
			Values:   []string{"pending", "in_progress", "completed", "failed"},
		})
		codeReviews.Fields.Add(&core.NumberField{Name: "issues_found"})                   // Count of issues
		codeReviews.Fields.Add(&core.NumberField{Name: "files_reviewed"})                 // Count of files
		codeReviews.Fields.Add(&core.JSONField{Name: "issues"})                           // Detailed issues array
		codeReviews.Fields.Add(&core.TextField{Name: "summary"})         // Overall summary
		codeReviews.Fields.Add(&core.DateField{Name: "started_at"})
		codeReviews.Fields.Add(&core.DateField{Name: "completed_at"})
		codeReviews.Fields.Add(&core.NumberField{Name: "duration_seconds"})
		codeReviews.Fields.Add(&core.JSONField{Name: "metadata"})                         // Additional context

		// Indexes for efficient queries
		codeReviews.AddIndex("idx_code_reviews_status", false, "status", "")
		codeReviews.AddIndex("idx_code_reviews_type", false, "review_type", "")
		codeReviews.AddIndex("idx_code_reviews_completed", false, "completed_at", "")

		// Public access rules (adjust based on security requirements)
		codeReviews.ListRule = types.Pointer("")
		codeReviews.ViewRule = types.Pointer("")
		codeReviews.CreateRule = types.Pointer("")
		codeReviews.UpdateRule = types.Pointer("")
		codeReviews.DeleteRule = types.Pointer("")

		if err := app.Save(codeReviews); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete code_reviews collection
		collection, err := app.FindCollectionByNameOrId("code_reviews")
		if err != nil {
			return nil // Already deleted
		}
		return app.Delete(collection)
	})
}
