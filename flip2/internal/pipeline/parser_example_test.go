package pipeline

import (
	"fmt"
	"os"
	"testing"
)

// TestResearchPipelineExample tests parsing the example research pipeline.
func TestResearchPipelineExample(t *testing.T) {
	// Read the research pipeline YAML
	yamlData, err := os.ReadFile("/Users/arielspivakovsky/src/flip/flip2/examples/research_pipeline.yaml")
	if err != nil {
		t.Fatalf("Error reading research_pipeline.yaml: %v", err)
	}

	// Parse the pipeline
	def, err := ParsePipeline(yamlData)
	if err != nil {
		t.Fatalf("Error parsing research pipeline: %v", err)
	}

	// Verify basic properties
	if def.Name != "research_pipeline" {
		t.Errorf("Expected name 'research_pipeline', got %q", def.Name)
	}

	if len(def.Stages) != 6 {
		t.Errorf("Expected 6 stages, got %d", len(def.Stages))
	}

	// Verify stage IDs
	expectedStages := []string{"gather", "analyze", "deepen_analysis", "cross_reference", "synthesize", "format_report"}
	for i, expectedID := range expectedStages {
		if i >= len(def.Stages) {
			t.Errorf("Missing stage at index %d", i)
			continue
		}
		if def.Stages[i].ID != expectedID {
			t.Errorf("Stage %d: expected ID %q, got %q", i, expectedID, def.Stages[i].ID)
		}
	}

	// Verify dependencies
	analyze := def.GetStage("analyze")
	if analyze == nil || len(analyze.DependsOn) != 1 || analyze.DependsOn[0] != "gather" {
		t.Errorf("analyze stage should depend on gather")
	}

	synthesize := def.GetStage("synthesize")
	if synthesize == nil || len(synthesize.DependsOn) != 2 {
		t.Errorf("synthesize stage should have 2 dependencies")
	}

	// Test execution order
	order, err := def.GetExecutionOrder()
	if err != nil {
		t.Fatalf("Error getting execution order: %v", err)
	}

	if len(order) != 6 {
		t.Errorf("Expected 6 stages in order, got %d", len(order))
	}

	// Verify gather is first (no dependencies)
	if order[0].ID != "gather" {
		t.Errorf("Expected first stage to be 'gather', got %q", order[0].ID)
	}

	// Verify format_report is last
	if order[5].ID != "format_report" {
		t.Errorf("Expected last stage to be 'format_report', got %q", order[5].ID)
	}

	// Verify topological order is respected
	stageIndexMap := make(map[string]int)
	for i, stage := range order {
		stageIndexMap[stage.ID] = i
	}

	for _, stage := range order {
		for _, dep := range stage.DependsOn {
			depIdx := stageIndexMap[dep]
			stageIdx := stageIndexMap[stage.ID]
			if depIdx >= stageIdx {
				t.Errorf("Dependency %q should come before %q", dep, stage.ID)
			}
		}
	}

	// Test stage utilities
	if def.GetStage("gather") == nil {
		t.Error("GetStage should find 'gather'")
	}

	if def.GetStage("nonexistent") != nil {
		t.Error("GetStage should return nil for nonexistent stage")
	}

	if def.GetStageIndex("synthesize") != 4 {
		t.Errorf("Expected synthesize at index 4, got %d", def.GetStageIndex("synthesize"))
	}

	t.Log("Research pipeline example parsed successfully!")
}

// ExamplePipelineDefinition demonstrates how to use the pipeline parser.
func ExamplePipelineDefinition() {
	yaml := `
name: simple_pipeline
description: A simple two-stage pipeline
stages:
  - id: stage_a
    name: First Stage
    backend: gemini
    command: "Gather information"
  - id: stage_b
    name: Second Stage
    backend: claude
    command: "Analyze the information"
    depends_on:
      - stage_a
`

	// Parse the pipeline
	def, err := ParsePipelineFromString(yaml)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Pipeline: %s\n", def.Name)
	fmt.Printf("Stages: %d\n", len(def.Stages))

	// Get execution order
	order, err := def.GetExecutionOrder()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Execution order:")
	for i, stage := range order {
		fmt.Printf("  %d. %s\n", i+1, stage.ID)
	}
	// Output:
	// Pipeline: simple_pipeline
	// Stages: 2
	// Execution order:
	//   1. stage_a
	//   2. stage_b
}
