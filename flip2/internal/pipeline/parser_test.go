// Package pipeline provides pipeline definition parsing and validation.
package pipeline

import (
	"testing"
	"time"
)

// TestParseValidPipeline tests parsing a valid pipeline definition.
func TestParseValidPipeline(t *testing.T) {
	yaml := `
name: test_pipeline
description: A simple test pipeline
version: "1.0"
stages:
  - id: gather
    name: Gather Information
    backend: gemini
    command: "Search for information about AI"
  - id: analyze
    name: Analyze Results
    backend: claude
    command: "Analyze and summarize the gathered information"
    depends_on:
      - gather
`

	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		t.Fatalf("ParsePipeline failed: %v", err)
	}

	if def.Name != "test_pipeline" {
		t.Errorf("Expected name 'test_pipeline', got %q", def.Name)
	}

	if len(def.Stages) != 2 {
		t.Errorf("Expected 2 stages, got %d", len(def.Stages))
	}

	if def.Stages[0].ID != "gather" {
		t.Errorf("Expected first stage ID 'gather', got %q", def.Stages[0].ID)
	}

	if def.Stages[1].ID != "analyze" {
		t.Errorf("Expected second stage ID 'analyze', got %q", def.Stages[1].ID)
	}

	if def.Stages[1].Backend != "claude" {
		t.Errorf("Expected backend 'claude', got %q", def.Stages[1].Backend)
	}
}

// TestParseMultiStagesPipeline tests parsing a pipeline with multiple stages.
func TestParseMultiStagesPipeline(t *testing.T) {
	yaml := `
name: research_pipeline
description: Multi-stage research and analysis pipeline
version: "1.0"
max_retries: 3
global_timeout: "1h"
variables:
  research_domain: "machine learning"
stages:
  - id: gather
    name: Gather Information
    backend: gemini
    command: "Research {{research_domain}}"
    timeout: "10m"
    retries: 2
  - id: analyze
    name: Analyze Results
    backend: claude
    command: "Analyze the gathered data"
    depends_on:
      - gather
  - id: format
    name: Format Output
    backend: claude
    command: "Format the analysis as a report"
    depends_on:
      - analyze
`

	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		t.Fatalf("ParsePipeline failed: %v", err)
	}

	if def.Name != "research_pipeline" {
		t.Errorf("Expected name 'research_pipeline', got %q", def.Name)
	}

	if len(def.Stages) != 3 {
		t.Errorf("Expected 3 stages, got %d", len(def.Stages))
	}

	if def.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", def.MaxRetries)
	}

	if def.GlobalTimeout == nil || def.GlobalTimeout.Duration != time.Hour {
		t.Errorf("Expected global timeout 1h")
	}

	// Check execution order
	order, err := def.GetExecutionOrder()
	if err != nil {
		t.Fatalf("GetExecutionOrder failed: %v", err)
	}

	if len(order) != 3 {
		t.Errorf("Expected 3 stages in order, got %d", len(order))
	}

	if order[0].ID != "gather" || order[1].ID != "analyze" || order[2].ID != "format" {
		t.Errorf("Unexpected execution order: %v", []string{order[0].ID, order[1].ID, order[2].ID})
	}
}

// TestCycleDetection tests that circular dependencies are detected.
func TestCycleDetection(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "simple cycle",
			yaml: `
name: cycle_test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
    depends_on:
      - stage_b
  - id: stage_b
    backend: claude
    command: "Task B"
    depends_on:
      - stage_a
`,
			wantErr: true,
			errMsg:  "cycle detected",
		},
		{
			name: "self cycle",
			yaml: `
name: self_cycle_test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
    depends_on:
      - stage_a
`,
			wantErr: true,
			errMsg:  "cycle detected",
		},
		{
			name: "complex cycle",
			yaml: `
name: complex_cycle_test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
    depends_on:
      - stage_d
  - id: stage_b
    backend: claude
    command: "Task B"
    depends_on:
      - stage_a
  - id: stage_c
    backend: claude
    command: "Task C"
    depends_on:
      - stage_b
  - id: stage_d
    backend: claude
    command: "Task D"
    depends_on:
      - stage_c
`,
			wantErr: true,
			errMsg:  "cycle detected",
		},
		{
			name: "no cycle - valid DAG",
			yaml: `
name: valid_dag_test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
  - id: stage_b
    backend: claude
    command: "Task B"
    depends_on:
      - stage_a
  - id: stage_c
    backend: claude
    command: "Task C"
    depends_on:
      - stage_a
  - id: stage_d
    backend: claude
    command: "Task D"
    depends_on:
      - stage_b
      - stage_c
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePipelineFromString(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePipeline error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestMissingDependency tests that missing dependencies are detected.
func TestMissingDependency(t *testing.T) {
	yaml := `
name: missing_dep_test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
    depends_on:
      - non_existent_stage
  - id: stage_b
    backend: claude
    command: "Task B"
`

	_, err := ParsePipelineFromString(yaml)
	if err == nil {
		t.Fatal("Expected error for missing dependency, got nil")
	}

	if !contains(err.Error(), "non_existent_stage") {
		t.Errorf("Expected error mentioning 'non_existent_stage', got %q", err.Error())
	}
}

// TestDuplicateStageID tests that duplicate stage IDs are detected.
func TestDuplicateStageID(t *testing.T) {
	yaml := `
name: duplicate_test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
  - id: stage_a
    backend: claude
    command: "Task A duplicate"
`

	_, err := ParsePipelineFromString(yaml)
	if err == nil {
		t.Fatal("Expected error for duplicate stage ID, got nil")
	}

	if !contains(err.Error(), "duplicate") {
		t.Errorf("Expected error containing 'duplicate', got %q", err.Error())
	}
}

// TestInvalidBackend tests that invalid backends are rejected.
func TestInvalidBackend(t *testing.T) {
	yaml := `
name: invalid_backend_test
stages:
  - id: stage_a
    backend: invalid_backend
    command: "Task A"
`

	_, err := ParsePipelineFromString(yaml)
	if err == nil {
		t.Fatal("Expected error for invalid backend, got nil")
	}

	if !contains(err.Error(), "invalid backend") {
		t.Errorf("Expected error containing 'invalid backend', got %q", err.Error())
	}
}

// TestRequiredFields tests that required fields are validated.
func TestRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing pipeline name",
			yaml: `
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
`,
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "missing stages",
			yaml: `
name: test
`,
			wantErr: true,
			errMsg:  "at least one stage",
		},
		{
			name: "missing stage ID",
			yaml: `
name: test
stages:
  - backend: claude
    command: "Task A"
`,
			wantErr: true,
			errMsg:  "ID is required",
		},
		{
			name: "missing stage backend",
			yaml: `
name: test
stages:
  - id: stage_a
    command: "Task A"
`,
			wantErr: true,
			errMsg:  "backend is required",
		},
		{
			name: "missing stage command",
			yaml: `
name: test
stages:
  - id: stage_a
    backend: claude
`,
			wantErr: true,
			errMsg:  "command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePipelineFromString(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePipeline error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestDurationParsing tests YAML duration parsing.
func TestDurationParsing(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectedDur time.Duration
		wantErr     bool
	}{
		{
			name: "parse 30 seconds",
			yaml: `
name: test
global_timeout: "30s"
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
`,
			expectedDur: 30 * time.Second,
		},
		{
			name: "parse 5 minutes",
			yaml: `
name: test
global_timeout: "5m"
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
`,
			expectedDur: 5 * time.Minute,
		},
		{
			name: "parse 2 hours",
			yaml: `
name: test
global_timeout: "2h"
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
`,
			expectedDur: 2 * time.Hour,
		},
		{
			name: "stage timeout",
			yaml: `
name: test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
    timeout: "10m"
`,
			expectedDur: 10 * time.Minute,
		},
		{
			name: "invalid duration",
			yaml: `
name: test
global_timeout: "invalid"
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParsePipelineFromString(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePipeline error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if tt.name == "stage timeout" {
				if def.Stages[0].Timeout == nil || def.Stages[0].Timeout.Duration != tt.expectedDur {
					t.Errorf("Expected duration %v, got %v", tt.expectedDur, def.Stages[0].Timeout)
				}
			} else {
				if def.GlobalTimeout == nil || def.GlobalTimeout.Duration != tt.expectedDur {
					t.Errorf("Expected duration %v, got %v", tt.expectedDur, def.GlobalTimeout)
				}
			}
		})
	}
}

// TestGetStage tests the GetStage utility function.
func TestGetStage(t *testing.T) {
	yaml := `
name: test
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
  - id: stage_b
    backend: gemini
    command: "Task B"
`

	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		t.Fatalf("ParsePipeline failed: %v", err)
	}

	stage := def.GetStage("stage_a")
	if stage == nil {
		t.Fatal("GetStage returned nil for existing stage")
	}
	if stage.ID != "stage_a" || stage.Backend != "claude" {
		t.Errorf("Unexpected stage: %+v", stage)
	}

	stage = def.GetStage("non_existent")
	if stage != nil {
		t.Errorf("GetStage should return nil for non-existent stage, got %+v", stage)
	}
}

// TestGetExecutionOrder tests the topological sort for execution order.
func TestGetExecutionOrder(t *testing.T) {
	yaml := `
name: test
stages:
  - id: stage_d
    backend: claude
    command: "Task D"
    depends_on:
      - stage_b
      - stage_c
  - id: stage_a
    backend: claude
    command: "Task A"
  - id: stage_b
    backend: claude
    command: "Task B"
    depends_on:
      - stage_a
  - id: stage_c
    backend: claude
    command: "Task C"
    depends_on:
      - stage_a
`

	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		t.Fatalf("ParsePipeline failed: %v", err)
	}

	order, err := def.GetExecutionOrder()
	if err != nil {
		t.Fatalf("GetExecutionOrder failed: %v", err)
	}

	if len(order) != 4 {
		t.Errorf("Expected 4 stages, got %d", len(order))
	}

	// stage_a should be first (no dependencies)
	if order[0].ID != "stage_a" {
		t.Errorf("Expected first stage to be 'stage_a', got %q", order[0].ID)
	}

	// stage_d should be last (depends on b and c)
	if order[3].ID != "stage_d" {
		t.Errorf("Expected last stage to be 'stage_d', got %q", order[3].ID)
	}

	// Verify that dependencies come before dependents
	stageIndexMap := make(map[string]int)
	for i, stage := range order {
		stageIndexMap[stage.ID] = i
	}

	for _, stage := range order {
		for _, dep := range stage.DependsOn {
			depIdx := stageIndexMap[dep]
			stageIdx := stageIndexMap[stage.ID]
			if depIdx >= stageIdx {
				t.Errorf("Dependency %q should come before %q in execution order", dep, stage.ID)
			}
		}
	}
}

// TestInputOutput tests input and output specifications.
func TestInputOutput(t *testing.T) {
	yaml := `
name: test
stages:
  - id: gather
    backend: gemini
    command: "Gather data"
    output:
      name: raw_data
      type: json
      persist: true
  - id: analyze
    backend: claude
    command: "Analyze data"
    input:
      type: previous_stage
      source: gather
      path: ".results"
    output:
      name: analysis
      type: json
`

	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		t.Fatalf("ParsePipeline failed: %v", err)
	}

	gather := def.GetStage("gather")
	if gather.Output == nil || gather.Output.Name != "raw_data" {
		t.Errorf("Unexpected gather output: %+v", gather.Output)
	}

	analyze := def.GetStage("analyze")
	if analyze.Input == nil || analyze.Input.Type != "previous_stage" || analyze.Input.Source != "gather" {
		t.Errorf("Unexpected analyze input: %+v", analyze.Input)
	}
}

// TestMetadata tests pipeline and stage metadata.
func TestMetadata(t *testing.T) {
	yaml := `
name: test
metadata:
  author: test_user
  version: "1.0"
  tags:
    - experimental
    - fast
variables:
  domain: "ML"
  model: "advanced"
stages:
  - id: stage_a
    backend: claude
    command: "Task A"
    metadata:
      priority: high
      owner: alice
`

	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		t.Fatalf("ParsePipeline failed: %v", err)
	}

	if def.Metadata["author"] != "test_user" {
		t.Errorf("Expected metadata author 'test_user', got %v", def.Metadata["author"])
	}

	if def.Variables["domain"] != "ML" {
		t.Errorf("Expected variable domain 'ML', got %v", def.Variables["domain"])
	}

	stage := def.GetStage("stage_a")
	if stage.Metadata["priority"] != "high" {
		t.Errorf("Expected stage priority 'high', got %v", stage.Metadata["priority"])
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || len(substr) > 0 && len(s) > 0 && contains(s[1:], substr)
}

// Simple contains implementation
func contains2(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
