package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// Pipeline Runs Table
		// Tracks the overall execution of a pipeline instance
		pipelineRuns := core.NewBaseCollection("pipeline_runs")
		pipelineRuns.Fields.Add(&core.TextField{Name: "pipeline_id", Required: true})
		pipelineRuns.Fields.Add(&core.TextField{Name: "status", Required: true})
		pipelineRuns.Fields.Add(&core.NumberField{Name: "current_stage_index", Required: true, Min: types.Pointer(0.0)})
		pipelineRuns.Fields.Add(&core.NumberField{Name: "total_stages", Required: true, Min: types.Pointer(1.0)})
		pipelineRuns.Fields.Add(&core.TextField{Name: "input"})                         // JSON
		pipelineRuns.Fields.Add(&core.TextField{Name: "final_output"})                  // JSON
		pipelineRuns.Fields.Add(&core.TextField{Name: "error"})                         // error message
		pipelineRuns.Fields.Add(&core.TextField{Name: "error_stage"})                   // stage name
		pipelineRuns.Fields.Add(&core.NumberField{Name: "retry_count", Required: true}) // 0
		pipelineRuns.Fields.Add(&core.NumberField{Name: "max_retries", Required: true}) // 3
		pipelineRuns.Fields.Add(&core.NumberField{Name: "priority", Required: true})    // 0
		pipelineRuns.Fields.Add(&core.TextField{Name: "assigned_agent"})                // coordinator agent
		pipelineRuns.Fields.Add(&core.DateField{Name: "started_at"})                    // when execution started
		pipelineRuns.Fields.Add(&core.DateField{Name: "completed_at"})                  // when it finished
		pipelineRuns.Fields.Add(&core.DateField{Name: "last_checkpoint_at"})            // last checkpoint time
		pipelineRuns.Fields.Add(&core.TextField{Name: "metadata"})                      // JSON

		// Indexes for efficient queries
		pipelineRuns.AddIndex("idx_pipeline_runs_status", false, "status", "")
		pipelineRuns.AddIndex("idx_pipeline_runs_pipeline_id", false, "pipeline_id", "")
		pipelineRuns.AddIndex("idx_pipeline_runs_priority", false, "priority", "")
		pipelineRuns.AddIndex("idx_pipeline_runs_assigned_agent", false, "assigned_agent", "")

		// Public access rules
		pipelineRuns.ListRule = types.Pointer("")
		pipelineRuns.ViewRule = types.Pointer("")
		pipelineRuns.CreateRule = types.Pointer("")
		pipelineRuns.UpdateRule = types.Pointer("")
		pipelineRuns.DeleteRule = types.Pointer("")

		if err := app.Save(pipelineRuns); err != nil {
			return err
		}

		// Stage Runs Table
		// Tracks the execution of each stage within a pipeline
		stageRuns := core.NewBaseCollection("stage_runs")
		stageRuns.Fields.Add(&core.TextField{Name: "pipeline_run_id", Required: true})
		stageRuns.Fields.Add(&core.TextField{Name: "stage_name", Required: true})
		stageRuns.Fields.Add(&core.NumberField{Name: "stage_index", Required: true, Min: types.Pointer(0.0)})
		stageRuns.Fields.Add(&core.TextField{Name: "status", Required: true})
		stageRuns.Fields.Add(&core.TextField{Name: "backend", Required: true})
		stageRuns.Fields.Add(&core.TextField{Name: "model"})  // optional
		stageRuns.Fields.Add(&core.TextField{Name: "input"})  // JSON
		stageRuns.Fields.Add(&core.TextField{Name: "output"}) // JSON
		stageRuns.Fields.Add(&core.TextField{Name: "error"})  // error message
		stageRuns.Fields.Add(&core.NumberField{Name: "retry_count", Required: true})
		stageRuns.Fields.Add(&core.NumberField{Name: "max_retries", Required: true})
		stageRuns.Fields.Add(&core.DateField{Name: "started_at"})
		stageRuns.Fields.Add(&core.DateField{Name: "completed_at"})
		stageRuns.Fields.Add(&core.TextField{Name: "task_id"})  // optional
		stageRuns.Fields.Add(&core.TextField{Name: "agent_id"}) // optional
		stageRuns.Fields.Add(&core.TextField{Name: "metrics"})  // JSON

		// Indexes for efficient queries
		stageRuns.AddIndex("idx_stage_runs_pipeline_run_id", false, "pipeline_run_id", "")
		stageRuns.AddIndex("idx_stage_runs_status", false, "status", "")
		stageRuns.AddIndex("idx_stage_runs_task_id", false, "task_id", "")

		// Public access rules
		stageRuns.ListRule = types.Pointer("")
		stageRuns.ViewRule = types.Pointer("")
		stageRuns.CreateRule = types.Pointer("")
		stageRuns.UpdateRule = types.Pointer("")
		stageRuns.DeleteRule = types.Pointer("")

		if err := app.Save(stageRuns); err != nil {
			return err
		}

		// Stage Artifacts Table
		// Stores intermediate outputs that can be referenced by later stages
		stageArtifacts := core.NewBaseCollection("stage_artifacts")
		stageArtifacts.Fields.Add(&core.TextField{Name: "pipeline_run_id", Required: true})
		stageArtifacts.Fields.Add(&core.TextField{Name: "stage_run_id", Required: true})
		stageArtifacts.Fields.Add(&core.TextField{Name: "name", Required: true})
		stageArtifacts.Fields.Add(&core.TextField{Name: "type", Required: true})         // json, text, file, url, binary
		stageArtifacts.Fields.Add(&core.TextField{Name: "data"})                         // JSON or data
		stageArtifacts.Fields.Add(&core.TextField{Name: "content_type", Required: true}) // MIME type
		stageArtifacts.Fields.Add(&core.NumberField{Name: "size_bytes", Required: true}) // 0
		stageArtifacts.Fields.Add(&core.TextField{Name: "checksum"})                     // SHA256
		stageArtifacts.Fields.Add(&core.DateField{Name: "expires_at"})                   // optional GC
		stageArtifacts.Fields.Add(&core.TextField{Name: "metadata"})                     // JSON

		// Indexes for efficient queries
		stageArtifacts.AddIndex("idx_stage_artifacts_pipeline_run_id", false, "pipeline_run_id", "")
		stageArtifacts.AddIndex("idx_stage_artifacts_stage_run_id", false, "stage_run_id", "")
		stageArtifacts.AddIndex("idx_stage_artifacts_name", false, "name", "")

		// Public access rules
		stageArtifacts.ListRule = types.Pointer("")
		stageArtifacts.ViewRule = types.Pointer("")
		stageArtifacts.CreateRule = types.Pointer("")
		stageArtifacts.UpdateRule = types.Pointer("")
		stageArtifacts.DeleteRule = types.Pointer("")

		if err := app.Save(stageArtifacts); err != nil {
			return err
		}

		// Pipeline Checkpoints Table
		// Stores saved pipeline state for recovery
		pipelineCheckpoints := core.NewBaseCollection("pipeline_checkpoints")
		pipelineCheckpoints.Fields.Add(&core.TextField{Name: "pipeline_run_id", Required: true})
		pipelineCheckpoints.Fields.Add(&core.NumberField{Name: "version", Required: true, Min: types.Pointer(0.0)})
		pipelineCheckpoints.Fields.Add(&core.TextField{Name: "pipeline_state", Required: true}) // JSON
		pipelineCheckpoints.Fields.Add(&core.TextField{Name: "stage_states", Required: true})   // JSON
		pipelineCheckpoints.Fields.Add(&core.TextField{Name: "artifact_refs"})                  // JSON refs
		pipelineCheckpoints.Fields.Add(&core.TextField{Name: "reason", Required: true})         // stage_complete, periodic, manual, pre_shutdown, error
		pipelineCheckpoints.Fields.Add(&core.NumberField{Name: "size_bytes", Required: true})   // 0

		// Indexes for efficient queries
		pipelineCheckpoints.AddIndex("idx_pipeline_checkpoints_pipeline_run_id", false, "pipeline_run_id", "")
		pipelineCheckpoints.AddIndex("idx_pipeline_checkpoints_version", false, "version", "")

		// Public access rules
		pipelineCheckpoints.ListRule = types.Pointer("")
		pipelineCheckpoints.ViewRule = types.Pointer("")
		pipelineCheckpoints.CreateRule = types.Pointer("")
		pipelineCheckpoints.UpdateRule = types.Pointer("")
		pipelineCheckpoints.DeleteRule = types.Pointer("")

		if err := app.Save(pipelineCheckpoints); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Rollback: delete all pipeline collections
		collections := []string{"pipeline_runs", "stage_runs", "stage_artifacts", "pipeline_checkpoints"}
		for _, collName := range collections {
			collection, err := app.FindCollectionByNameOrId(collName)
			if err != nil {
				continue // Already deleted
			}
			if err := app.Delete(collection); err != nil {
				return err
			}
		}
		return nil
	})
}
