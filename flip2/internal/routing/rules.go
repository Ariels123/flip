// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"fmt"
	"math/rand"
	"os"
	"sort"

	"flip2/internal/config"
	"gopkg.in/yaml.v3"
)

// ================================================================================
// RULES ENGINE
// ================================================================================

// ABTestConfig holds configuration for A/B testing routing decisions.
type ABTestConfig struct {
	// Percentage is the percentage of tasks (0-100) to route to the variant model.
	Percentage int

	// VariantModel is the alternative model to route a percentage of tasks to.
	VariantModel Model

	// ControlModel is the standard model used for the remaining tasks.
	ControlModel Model

	// Enabled indicates if A/B testing is active.
	Enabled bool
}

// RulesEngine manages routing rules and makes routing decisions.
type RulesEngine struct {
	// Rules is the list of routing rules, sorted by priority.
	Rules []RoutingRule

	// DefaultModel is the fallback model if no rule matches.
	DefaultModel Model

	// ModelConfigs contains configuration for each model.
	ModelConfigs map[Model]ModelConfig

	// Overrides maps task IDs to their override models.
	// When a task ID has an override, it takes precedence over all rules.
	Overrides map[string]Model

	// ABTest holds A/B testing configuration for routing experimentation.
	ABTest *ABTestConfig
}

// Override represents a routing override for a specific task.
type Override struct {
	TaskID string
	Model  Model
}

// NewRulesEngine creates a new RulesEngine with default rules and configurations.
func NewRulesEngine() *RulesEngine {
	return &RulesEngine{
		Rules:        DefaultRoutingRules(),
		DefaultModel: ModelSonnet,
		ModelConfigs: DefaultModelConfigs(),
		Overrides:    make(map[string]Model),
	}
}

// NewRulesEngineFromFile creates a new RulesEngine by loading rules from a YAML file.
func NewRulesEngineFromFile(yamlPath string) (*RulesEngine, error) {
	engine := &RulesEngine{
		DefaultModel: ModelSonnet,
		ModelConfigs: DefaultModelConfigs(),
		Overrides:    make(map[string]Model),
	}

	if err := engine.LoadRules(yamlPath); err != nil {
		return nil, err
	}

	return engine, nil
}

// RulesConfig represents the top-level YAML structure for routing rules.
type RulesConfig struct {
	// Rules is the list of routing rules.
	Rules []RoutingRuleYAML `yaml:"rules"`

	// DefaultModel is the fallback model if no rule matches.
	DefaultModel *string `yaml:"default_model,omitempty"`

	// Description is a human-readable description of this rules file.
	Description string `yaml:"description,omitempty"`

	// Version indicates the version of the rules schema.
	Version string `yaml:"version,omitempty"`
}

// RoutingRuleYAML represents a routing rule in YAML format.
// This is used for unmarshalling from YAML and then converted to RoutingRule.
type RoutingRuleYAML struct {
	// TaskType this rule applies to (or empty for all).
	TaskType *string `yaml:"task_type"`

	// MinComplexity is the minimum overall complexity score to match.
	MinComplexity *float64 `yaml:"min_complexity"`

	// MaxComplexity is the maximum overall complexity score to match.
	MaxComplexity *float64 `yaml:"max_complexity"`

	// Complexity is a shorthand for matching exact complexity or range.
	// Can be a single number "3" or a range "2-4".
	Complexity *string `yaml:"complexity"`

	// MinRiskLevel is the minimum risk level to match.
	MinRiskLevel *int `yaml:"min_risk_level"`

	// TargetModel is the model to use when this rule matches.
	TargetModel string `yaml:"model"`

	// Priority determines which rule wins on conflicts (higher = wins).
	Priority *int `yaml:"priority"`

	// Description explains the rule for debugging.
	Description string `yaml:"description,omitempty"`
}

// LoadRules loads routing rules from a YAML file.
func (e *RulesEngine) LoadRules(yamlPath string) error {
	// Read file
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("failed to read rules file: %w", err)
	}

	// Unmarshal YAML
	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	// Convert rules from YAML format
	rules, err := convertYAMLRules(config.Rules)
	if err != nil {
		return fmt.Errorf("failed to convert rules: %w", err)
	}

	// Set default model if specified
	if config.DefaultModel != nil && *config.DefaultModel != "" {
		if model := Model(*config.DefaultModel); model.IsValid() {
			e.DefaultModel = model
		}
	}

	// Sort rules by priority (highest first)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	e.Rules = rules
	return nil
}

// convertYAMLRules converts RoutingRuleYAML to RoutingRule.
func convertYAMLRules(yamlRules []RoutingRuleYAML) ([]RoutingRule, error) {
	var rules []RoutingRule

	for i, yr := range yamlRules {
		rule := RoutingRule{
			TargetModel: Model(yr.TargetModel),
			Priority:    500, // default priority
			Description: yr.Description,
		}

		// Validate target model
		if !rule.TargetModel.IsValid() {
			return nil, fmt.Errorf("rule %d: invalid model '%s'", i, yr.TargetModel)
		}

		// Set task type
		if yr.TaskType != nil && *yr.TaskType != "" {
			taskType := TaskType(*yr.TaskType)
			if !taskType.IsValid() {
				return nil, fmt.Errorf("rule %d: invalid task type '%s'", i, *yr.TaskType)
			}
			rule.TaskType = taskType
		}

		// Set priority
		if yr.Priority != nil {
			rule.Priority = *yr.Priority
		}

		// Set min risk level
		if yr.MinRiskLevel != nil {
			rule.MinRiskLevel = *yr.MinRiskLevel
		}

		// Parse complexity rules
		// Support both min/max format and shorthand format
		if yr.MinComplexity != nil {
			rule.MinComplexity = *yr.MinComplexity
		}
		if yr.MaxComplexity != nil {
			rule.MaxComplexity = *yr.MaxComplexity
		}

		// Parse shorthand complexity format (e.g., "2-3" or "4")
		if yr.Complexity != nil {
			min, max, err := parseComplexityRange(*yr.Complexity)
			if err != nil {
				return nil, fmt.Errorf("rule %d: invalid complexity range '%s': %w", i, *yr.Complexity, err)
			}
			if min > 0 {
				rule.MinComplexity = min
			}
			if max > 0 {
				rule.MaxComplexity = max
			}
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// parseComplexityRange parses complexity strings like "2-3" or "4".
// Returns (min, max) as float64. Returns 0 for unbounded limits.
func parseComplexityRange(s string) (float64, float64, error) {
	// Try to parse as range "2-3"
	var n1, n2 float64
	if _, err := fmt.Sscanf(s, "%f-%f", &n1, &n2); err == nil {
		return n1, n2, nil
	}

	// Try to parse as single number
	if _, err := fmt.Sscanf(s, "%f", &n1); err == nil {
		// Single number means exact match (min and max equal)
		return n1, n1, nil
	}

	return 0, 0, fmt.Errorf("invalid complexity format: %s", s)
}

// LoadProjectRules loads routing rules from a ProjectConfig and merges them with default rules.
// Project-specific rules take precedence over default rules when task types match.
// Returns an error if the config is nil or contains invalid routing specifications.
func (e *RulesEngine) LoadProjectRules(projConfig *config.ProjectConfig) error {
	if projConfig == nil {
		return fmt.Errorf("project config cannot be nil")
	}

	// Convert project routing rules to RoutingRule objects
	projectRules, err := convertProjectRoutes(projConfig.Routes)
	if err != nil {
		return fmt.Errorf("failed to convert project routes: %w", err)
	}

	// Merge project rules with default rules
	// Project rules should have higher priority than default rules
	e.mergeRules(projectRules)

	return nil
}

// convertProjectRoutes converts project config routes to RoutingRule objects.
// Each route is converted to a rule with high priority to ensure precedence over defaults.
func convertProjectRoutes(routes []config.Route) ([]RoutingRule, error) {
	var rules []RoutingRule

	for i, route := range routes {
		if route.RouteTo == "" {
			return nil, fmt.Errorf("route %d: 'RouteTo' model is required", i)
		}

		// Validate the target model
		model := Model(route.RouteTo)
		if !model.IsValid() {
			return nil, fmt.Errorf("route %d: invalid model '%s'", i, route.RouteTo)
		}

		// Create a rule from the route
		// We use priority 950 for project rules (higher than default 500, but lower than critical 1000)
		rule := RoutingRule{
			TargetModel: model,
			Priority:    950,
			Description: fmt.Sprintf("Project-specific rule: %s", route.Name),
		}

		// If the route has a condition that looks like a task type, parse it
		if route.Condition != "" {
			taskType := TaskType(route.Condition)
			if taskType.IsValid() {
				rule.TaskType = taskType
			}
			// If condition is not a recognized task type, we treat the rule as applying to all tasks
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

// mergeRules merges project-specific rules with existing default rules.
// For each task type in project rules, we replace or override the corresponding default rule.
// Task types not in project rules retain their default behavior.
func (e *RulesEngine) mergeRules(projectRules []RoutingRule) {
	// Create a map of project rules by task type for quick lookup
	projectRulesByType := make(map[TaskType]*RoutingRule)
	for i := range projectRules {
		if projectRules[i].TaskType != "" {
			projectRulesByType[projectRules[i].TaskType] = &projectRules[i]
		}
	}

	// Update or insert project rules, removing conflicting defaults
	updatedRules := []RoutingRule{}

	// First pass: keep default rules that don't conflict with project rules
	for _, defaultRule := range e.Rules {
		if defaultRule.TaskType != "" {
			if _, exists := projectRulesByType[defaultRule.TaskType]; !exists {
				// This default rule's task type is not overridden by project rules
				updatedRules = append(updatedRules, defaultRule)
			}
		} else {
			// Keep generic rules (no specific task type)
			updatedRules = append(updatedRules, defaultRule)
		}
	}

	// Second pass: add all project rules
	updatedRules = append(updatedRules, projectRules...)

	// Sort by priority (highest first)
	sort.Slice(updatedRules, func(i, j int) bool {
		return updatedRules[i].Priority > updatedRules[j].Priority
	})

	e.Rules = updatedRules
}

// SetOverride sets a routing override for a specific task ID.
// The override takes precedence over all routing rules.
func (e *RulesEngine) SetOverride(taskID string, model Model) error {
	if !model.IsValid() {
		return fmt.Errorf("invalid model: %s", model)
	}
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	e.Overrides[taskID] = model
	return nil
}

// GetOverride retrieves the override model for a task ID, if one exists.
// Returns (model, exists).
func (e *RulesEngine) GetOverride(taskID string) (Model, bool) {
	model, exists := e.Overrides[taskID]
	return model, exists
}

// ClearOverride removes an override for a specific task ID.
func (e *RulesEngine) ClearOverride(taskID string) {
	delete(e.Overrides, taskID)
}

// EnableABTest enables A/B testing with the given configuration.
// The percentage determines what portion of tasks are routed to the variant model.
func (e *RulesEngine) EnableABTest(config *ABTestConfig) error {
	if config == nil {
		return fmt.Errorf("ABTestConfig cannot be nil")
	}
	if config.Percentage < 0 || config.Percentage > 100 {
		return fmt.Errorf("percentage must be between 0 and 100, got %d", config.Percentage)
	}
	if !config.VariantModel.IsValid() {
		return fmt.Errorf("invalid variant model: %s", config.VariantModel)
	}
	if !config.ControlModel.IsValid() {
		return fmt.Errorf("invalid control model: %s", config.ControlModel)
	}
	config.Enabled = true
	e.ABTest = config
	return nil
}

// DisableABTest disables A/B testing.
func (e *RulesEngine) DisableABTest() {
	if e.ABTest != nil {
		e.ABTest.Enabled = false
	}
}

// RouteTask determines which model should handle a task.
// Takes task type and overall complexity score and returns the recommended model.
// Checks overrides first, then falls back to rule-based routing.
// If A/B testing is enabled, may route a percentage to the variant model.
func (e *RulesEngine) RouteTask(taskID string, taskType TaskType, complexity float64) Model {
	// Check for override first (highest priority)
	if override, exists := e.GetOverride(taskID); exists {
		return override
	}

	// Find the highest priority rule that matches
	var primaryModel Model
	for _, rule := range e.Rules {
		if e.ruleMatches(rule, taskType, complexity) {
			primaryModel = rule.TargetModel
			break
		}
	}

	// If no rule matched, use default
	if primaryModel == "" {
		primaryModel = e.DefaultModel
	}

	// Apply A/B testing if enabled
	if e.ABTest != nil && e.ABTest.Enabled {
		if rand.Intn(100) < e.ABTest.Percentage {
			return e.ABTest.VariantModel
		}
		return e.ABTest.ControlModel
	}

	return primaryModel
}

// ruleMatches checks if a rule matches the given task parameters.
func (e *RulesEngine) ruleMatches(rule RoutingRule, taskType TaskType, complexity float64) bool {
	// Check task type (if rule specifies one)
	if rule.TaskType != "" && rule.TaskType != taskType {
		return false
	}

	// Check min complexity
	if rule.MinComplexity > 0 && complexity < rule.MinComplexity {
		return false
	}

	// Check max complexity
	if rule.MaxComplexity > 0 && complexity > rule.MaxComplexity {
		return false
	}

	// Check min risk level (if this rule specifies one)
	// Note: This would require risk level in RouteTask, so for now we skip this check
	// It can be enhanced in RouteTaskWithRisk

	return true
}

// RouteTaskWithFullClassification routes a task based on complete classification.
// This is the primary method that should be used with full task information.
// taskID is required for checking overrides.
func (e *RulesEngine) RouteTaskWithFullClassification(taskID string, classification *TaskClassification) Model {
	return e.RouteTask(taskID, classification.TaskType, classification.Complexity.OverallScore())
}

// RoutingStats contains statistics about routing decisions.
type RoutingStats struct {
	// TotalRouted is the total number of routing decisions made.
	TotalRouted int

	// RoutingByModel counts routing decisions per model.
	RoutingByModel map[Model]int

	// AverageComplexity is the average complexity score of routed tasks.
	AverageComplexity float64

	// CriticalTasksDetected is the number of tasks with risk level 5.
	CriticalTasksDetected int
}

// DisplayName returns the human-readable name of the model.
func (m Model) DisplayName() string {
	configs := DefaultModelConfigs()
	if config, ok := configs[m]; ok {
		return config.DisplayName
	}
	return string(m)
}
