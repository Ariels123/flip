-- Pipeline Runs Table
-- Tracks the overall execution of a pipeline instance
CREATE TABLE IF NOT EXISTS pipeline_runs (
    id TEXT PRIMARY KEY,
    pipeline_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    current_stage_index INTEGER NOT NULL DEFAULT 0,
    total_stages INTEGER NOT NULL DEFAULT 0,
    input TEXT,
    final_output TEXT,
    error TEXT,
    error_stage TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    priority INTEGER NOT NULL DEFAULT 0,
    assigned_agent TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_checkpoint_at DATETIME,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_pipeline_runs_status ON pipeline_runs(status);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_pipeline_id ON pipeline_runs(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_priority ON pipeline_runs(priority DESC);
CREATE INDEX IF NOT EXISTS idx_pipeline_runs_assigned_agent ON pipeline_runs(assigned_agent);

-- Stage Runs Table
-- Tracks the execution of each stage within a pipeline
CREATE TABLE IF NOT EXISTS stage_runs (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    stage_name TEXT NOT NULL,
    stage_index INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    backend TEXT NOT NULL,
    model TEXT,
    input TEXT,
    output TEXT,
    error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 2,
    started_at DATETIME,
    completed_at DATETIME,
    task_id TEXT,
    agent_id TEXT,
    metrics TEXT,
    FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_stage_runs_pipeline_run_id ON stage_runs(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_stage_runs_status ON stage_runs(status);
CREATE INDEX IF NOT EXISTS idx_stage_runs_task_id ON stage_runs(task_id);

-- Stage Artifacts Table
-- Stores intermediate outputs that can be referenced by later stages
CREATE TABLE IF NOT EXISTS stage_artifacts (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    stage_run_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'json',
    data TEXT,
    content_type TEXT NOT NULL DEFAULT 'application/json',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    checksum TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    metadata TEXT,
    FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    FOREIGN KEY (stage_run_id) REFERENCES stage_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_stage_artifacts_pipeline_run_id ON stage_artifacts(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_stage_artifacts_stage_run_id ON stage_artifacts(stage_run_id);
CREATE INDEX IF NOT EXISTS idx_stage_artifacts_name ON stage_artifacts(name);

-- Checkpoints Table
-- Stores saved pipeline state for recovery
CREATE TABLE IF NOT EXISTS pipeline_checkpoints (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    pipeline_state TEXT NOT NULL,
    stage_states TEXT NOT NULL,
    artifact_refs TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reason TEXT NOT NULL DEFAULT 'stage_complete',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_pipeline_run_id ON pipeline_checkpoints(pipeline_run_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_version ON pipeline_checkpoints(version DESC);
