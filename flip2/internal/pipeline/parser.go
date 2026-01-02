// Package pipeline provides pipeline definition parsing and validation.
package pipeline

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// PIPELINE DEFINITION STRUCTURES
// =============================================================================

// PipelineDefinition represents the complete definition of a pipeline read from YAML.
// It serves as the blueprint for creating pipeline runs.
type PipelineDefinition struct {
	// Name is the unique identifier for this pipeline (e.g., "research", "data-analyze").
	Name string `yaml:"name" json:"name"`

	// Description explains what this pipeline does.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Version indicates the pipeline schema version (e.g., "1.0").
	Version string `yaml:"version,omitempty" json:"version,omitempty"`

	// Stages is an ordered list of stages to execute in sequence.
	Stages []Stage `yaml:"stages" json:"stages"`

	// GlobalDependencies are dependencies shared across all stages.
	GlobalDependencies []string `yaml:"global_dependencies,omitempty" json:"global_dependencies,omitempty"`

	// GlobalTimeout is the maximum duration for the entire pipeline (0 = no limit).
	GlobalTimeout *Duration `yaml:"global_timeout,omitempty" json:"global_timeout,omitempty"`

	// MaxRetries is the maximum number of total retries across the pipeline.
	MaxRetries int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`

	// Variables are pipeline-level variables that can be substituted in stage commands.
	Variables map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`

	// OnFailure defines behavior when the pipeline fails.
	OnFailure string `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`

	// Metadata stores arbitrary key-value pairs.
	Metadata map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// Stage represents a single stage in a pipeline.
type Stage struct {
	// ID is the unique identifier for this stage within the pipeline.
	ID string `yaml:"id" json:"id"`

	// Name is a human-readable name for the stage.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Description explains what this stage does.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Backend is the LLM backend to use: "claude", "gemini", or "antigravity".
	Backend string `yaml:"backend" json:"backend"`

	// Model is the specific model to use (optional, uses backend default if empty).
	// Examples: "claude-opus-4-5", "gemini-2.5-pro", "gemini-2.5-pro-exp-0801".
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// Command is the task/prompt to execute in this stage.
	Command string `yaml:"command" json:"command"`

	// Input specifies where to get input for this stage.
	// Can reference previous stage outputs using "${stages.previous_stage_id.output}".
	Input *Input `yaml:"input,omitempty" json:"input,omitempty"`

	// Output specifies where to store or name the output of this stage.
	Output *Output `yaml:"output,omitempty" json:"output,omitempty"`

	// Timeout is the maximum duration for this stage (0 = inherit from pipeline).
	Timeout *Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Retries is the number of times to retry this stage on failure (0 = no retries).
	Retries int `yaml:"retries,omitempty" json:"retries,omitempty"`

	// DependsOn specifies stage IDs that must complete before this stage runs.
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`

	// Skip indicates whether this stage should be skipped (conditional execution).
	Skip *Condition `yaml:"skip,omitempty" json:"skip,omitempty"`

	// Parallel indicates if multiple instances of this stage can run in parallel.
	Parallel *Parallel `yaml:"parallel,omitempty" json:"parallel,omitempty"`

	// Transform specifies a transformation to apply to the stage output.
	Transform string `yaml:"transform,omitempty" json:"transform,omitempty"`

	// Metadata stores arbitrary key-value pairs for this stage.
	Metadata map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// Input specifies where to get input for a stage.
type Input struct {
	// Type is "pipeline_input", "previous_stage", "artifact", or "literal".
	Type string `yaml:"type" json:"type"`

	// Source specifies the source: stage ID, artifact name, or literal value.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	// Path is a JSONPath to a specific field within the input.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}

// Output specifies where to store the stage output.
type Output struct {
	// Name is the artifact name to store this output as.
	Name string `yaml:"name" json:"name"`

	// Type is the artifact type: "json", "text", "file", or "url".
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Persist indicates if the output should be persisted.
	Persist bool `yaml:"persist,omitempty" json:"persist,omitempty"`
}

// Condition represents a conditional expression.
type Condition struct {
	// When is the condition expression (e.g., "input == null", "env.SKIP_ANALYSIS == true").
	When string `yaml:"when" json:"when"`
}

// Parallel specifies parallel execution configuration.
type Parallel struct {
	// Enabled indicates if parallel execution is enabled for this stage.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Count is the number of parallel instances to run.
	Count int `yaml:"count,omitempty" json:"count,omitempty"`

	// BatchSize is the number of items to process per parallel instance.
	BatchSize int `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`
}

// Duration is a custom type for parsing YAML duration strings.
type Duration struct {
	Duration time.Duration
}

// UnmarshalYAML parses a duration from YAML (e.g., "5m", "30s", "2h").
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format: %q, expected format like 5m, 30s, 2h: %w", s, err)
	}
	d.Duration = duration
	return nil
}

// MarshalYAML marshals a duration to YAML.
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// =============================================================================
// PARSER
// =============================================================================

// ParsePipeline parses a pipeline definition from YAML bytes.
// It validates the pipeline structure, including DAG correctness.
func ParsePipeline(yamlBytes []byte) (*PipelineDefinition, error) {
	var def PipelineDefinition
	if err := yaml.Unmarshal(yamlBytes, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate the definition
	if err := def.Validate(); err != nil {
		return nil, err
	}

	return &def, nil
}

// ParsePipelineFromString parses a pipeline definition from a YAML string.
func ParsePipelineFromString(yamlStr string) (*PipelineDefinition, error) {
	return ParsePipeline([]byte(yamlStr))
}

// =============================================================================
// VALIDATION
// =============================================================================

// Validate checks if the pipeline definition is valid.
// It checks for:
// - Required fields
// - Valid stage backends
// - Circular dependencies (DAG validation)
// - Missing dependencies
// - Duplicate stage IDs
func (p *PipelineDefinition) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(p.Stages) == 0 {
		return fmt.Errorf("pipeline must have at least one stage")
	}

	// Check for duplicate stage IDs and valid stage structure
	stageIDs := make(map[string]bool)
	for i, stage := range p.Stages {
		if stage.ID == "" {
			return fmt.Errorf("stage %d: ID is required", i)
		}
		if stageIDs[stage.ID] {
			return fmt.Errorf("duplicate stage ID: %q", stage.ID)
		}
		stageIDs[stage.ID] = true

		if err := stage.Validate(); err != nil {
			return fmt.Errorf("stage %q: %w", stage.ID, err)
		}
	}

	// Check for missing dependencies
	if err := p.ValidateDependencies(); err != nil {
		return err
	}

	// Check for cycles in the dependency graph
	if err := p.ValidateNoCycles(); err != nil {
		return err
	}

	return nil
}

// Validate checks if the stage is valid.
func (s *Stage) Validate() error {
	if s.Backend == "" {
		return fmt.Errorf("backend is required")
	}

	validBackends := map[string]bool{
		"claude":      true,
		"gemini":      true,
		"antigravity": true,
	}
	if !validBackends[s.Backend] {
		return fmt.Errorf("invalid backend: %q (valid: claude, gemini, antigravity)", s.Backend)
	}

	if s.Command == "" {
		return fmt.Errorf("command is required")
	}

	if s.Input != nil {
		if err := s.Input.Validate(); err != nil {
			return fmt.Errorf("invalid input: %w", err)
		}
	}

	if s.Output != nil {
		if err := s.Output.Validate(); err != nil {
			return fmt.Errorf("invalid output: %w", err)
		}
	}

	return nil
}

// Validate checks if the input specification is valid.
func (i *Input) Validate() error {
	validTypes := map[string]bool{
		"pipeline_input": true,
		"previous_stage": true,
		"artifact":       true,
		"literal":        true,
	}
	if !validTypes[i.Type] {
		return fmt.Errorf("invalid input type: %q", i.Type)
	}

	// Source is required for non-literal inputs
	if i.Type != "literal" && i.Source == "" {
		return fmt.Errorf("source is required for input type %q", i.Type)
	}

	return nil
}

// Validate checks if the output specification is valid.
func (o *Output) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("output name is required")
	}

	validTypes := map[string]bool{
		"json": true,
		"text": true,
		"file": true,
		"url":  true,
	}
	if o.Type != "" && !validTypes[o.Type] {
		return fmt.Errorf("invalid output type: %q", o.Type)
	}

	return nil
}

// ValidateDependencies checks that all referenced dependencies exist.
func (p *PipelineDefinition) ValidateDependencies() error {
	stageIDs := make(map[string]bool)
	for _, stage := range p.Stages {
		stageIDs[stage.ID] = true
	}

	for _, stage := range p.Stages {
		for _, dep := range stage.DependsOn {
			if !stageIDs[dep] {
				return fmt.Errorf("stage %q depends on non-existent stage %q", stage.ID, dep)
			}
		}
	}

	return nil
}

// ValidateNoCycles checks that the dependency graph has no cycles.
// Uses DFS to detect cycles in the DAG.
func (p *PipelineDefinition) ValidateNoCycles() error {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, stage := range p.Stages {
		if _, exists := adj[stage.ID]; !exists {
			adj[stage.ID] = make([]string, 0)
		}
		for _, dep := range stage.DependsOn {
			adj[stage.ID] = append(adj[stage.ID], dep)
		}
	}

	// Color-based DFS for cycle detection
	// 0 = white (unvisited), 1 = gray (visiting), 2 = black (visited)
	colors := make(map[string]int)
	for _, stage := range p.Stages {
		colors[stage.ID] = 0
	}

	var detectCycle func(stageID string) error
	detectCycle = func(stageID string) error {
		colors[stageID] = 1 // Mark as visiting

		for _, neighbor := range adj[stageID] {
			if colors[neighbor] == 1 {
				return fmt.Errorf("cycle detected: stage %q -> %q", stageID, neighbor)
			}
			if colors[neighbor] == 0 {
				if err := detectCycle(neighbor); err != nil {
					return err
				}
			}
		}

		colors[stageID] = 2 // Mark as visited
		return nil
	}

	for _, stage := range p.Stages {
		if colors[stage.ID] == 0 {
			if err := detectCycle(stage.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// ToJSON converts the pipeline definition to JSON bytes.
func (p *PipelineDefinition) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// ToYAML converts the pipeline definition back to YAML bytes.
func (p *PipelineDefinition) ToYAML() ([]byte, error) {
	return yaml.Marshal(p)
}

// GetStage returns a stage by ID, or nil if not found.
func (p *PipelineDefinition) GetStage(id string) *Stage {
	for i := range p.Stages {
		if p.Stages[i].ID == id {
			return &p.Stages[i]
		}
	}
	return nil
}

// GetStageIndex returns the index of a stage by ID, or -1 if not found.
func (p *PipelineDefinition) GetStageIndex(id string) int {
	for i, stage := range p.Stages {
		if stage.ID == id {
			return i
		}
	}
	return -1
}

// GetExecutionOrder returns the stages in execution order (topological sort).
// For pipelines with dependencies, stages are ordered so that all dependencies
// come before the dependent stage.
func (p *PipelineDefinition) GetExecutionOrder() ([]Stage, error) {
	// Kahn's algorithm for topological sort
	inDegree := make(map[string]int)
	adj := make(map[string][]string)

	// Initialize
	for _, stage := range p.Stages {
		inDegree[stage.ID] = 0
		adj[stage.ID] = make([]string, 0)
	}

	// Build adjacency list (reverse direction: stage -> dependents)
	for _, stage := range p.Stages {
		for _, dep := range stage.DependsOn {
			adj[dep] = append(adj[dep], stage.ID)
			inDegree[stage.ID]++
		}
	}

	// Find all vertices with in-degree 0
	queue := make([]string, 0)
	for _, stage := range p.Stages {
		if inDegree[stage.ID] == 0 {
			queue = append(queue, stage.ID)
		}
	}

	result := make([]Stage, 0, len(p.Stages))
	for len(queue) > 0 {
		stageID := queue[0]
		queue = queue[1:]

		result = append(result, *p.GetStage(stageID))

		// Reduce in-degree for all neighbors
		for _, neighbor := range adj[stageID] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If not all stages were processed, there's a cycle (shouldn't happen if ValidateNoCycles passed)
	if len(result) != len(p.Stages) {
		return nil, fmt.Errorf("cycle detected in pipeline dependencies")
	}

	return result, nil
}
