package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFLIP2MDParserBasicParsing tests basic parser functionality
func TestFLIP2MDParserBasicParsing(t *testing.T) {
	content := `# FLIP2.md - Project Configuration

**Project:** TestProject
**Version:** 1.0.0
**Coordinator:** claude-coordinator
**Last Updated:** 2026-01-02

---

## Agents

### Agent Role: Analyst
- **ID Pattern:** ` + "`analyst-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, send-signals`" + `
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** ` + "`access-secrets`" + `
- **Cost Budget (USD/hour):** 2.50
- **Description:** Data analyst role

---

## Commands

### Command: /analyze
- **Aliases:** ` + "`check, run-analysis`" + `
- **Handler:** ` + "`analyst-worker`" + `
- **Args:** ` + "`<dataset> [--format=json|csv]`" + `
- **Description:** Analyze dataset
- **Requires Approval:** no
- **Allowed Roles:** ` + "`analyst, coordinator`" + `

---

## Routing

### Route: Fast Analysis
- **When:** ` + "`task.complexity < 5`" + `
- **Route To:** ` + "`gemini`" + `
- **Reason:** Faster and cheaper for simple tasks
- **Cost Impact:** ` + "`-0.30`" + `

---

## Context

### Auto-Load Files
- ` + "`./README.md`" + ` - Project overview (weight: high)
- ` + "`./ARCHITECTURE.md`" + ` - Architecture guide (weight: medium)
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify metadata
	if config.Project != "TestProject" {
		t.Errorf("expected Project='TestProject', got '%s'", config.Project)
	}
	if config.Version != "1.0.0" {
		t.Errorf("expected Version='1.0.0', got '%s'", config.Version)
	}
	if config.Coordinator != "claude-coordinator" {
		t.Errorf("expected Coordinator='claude-coordinator', got '%s'", config.Coordinator)
	}

	// Verify agents
	if len(config.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(config.Agents))
	} else {
		agent := config.Agents[0]
		if agent.Name != "Analyst" {
			t.Errorf("expected agent name 'Analyst', got '%s'", agent.Name)
		}
		if agent.Model != "gemini" {
			t.Errorf("expected model 'gemini', got '%s'", agent.Model)
		}
		if agent.MaxConcurrentTasks != 5 {
			t.Errorf("expected 5 concurrent tasks, got %d", agent.MaxConcurrentTasks)
		}
		if agent.CostBudgetPerHour != 2.50 {
			t.Errorf("expected cost 2.50, got %f", agent.CostBudgetPerHour)
		}
	}

	// Verify commands
	if len(config.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(config.Commands))
	} else {
		cmd := config.Commands[0]
		if cmd.Name != "/analyze" {
			t.Errorf("expected command '/analyze', got '%s'", cmd.Name)
		}
		if cmd.Handler != "analyst-worker" {
			t.Errorf("expected handler 'analyst-worker', got '%s'", cmd.Handler)
		}
		if len(cmd.Aliases) != 2 {
			t.Errorf("expected 2 aliases, got %d", len(cmd.Aliases))
		}
	}

	// Verify routing
	if len(config.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(config.Routes))
	} else {
		route := config.Routes[0]
		if route.Name != "Fast Analysis" {
			t.Errorf("expected route 'Fast Analysis', got '%s'", route.Name)
		}
		if route.RouteTo != "gemini" {
			t.Errorf("expected route to 'gemini', got '%s'", route.RouteTo)
		}
		if route.CostImpact != -0.30 {
			t.Errorf("expected cost impact -0.30, got %f", route.CostImpact)
		}
	}

	// Verify context
	if len(config.Context.AutoLoadFiles) != 2 {
		t.Errorf("expected 2 context files, got %d", len(config.Context.AutoLoadFiles))
	} else {
		if config.Context.AutoLoadFiles[0].Path != "./README.md" {
			t.Errorf("expected first file path './README.md', got '%s'", config.Context.AutoLoadFiles[0].Path)
		}
		if config.Context.AutoLoadFiles[0].Weight != "high" {
			t.Errorf("expected first file weight 'high', got '%s'", config.Context.AutoLoadFiles[0].Weight)
		}
	}
}

// TestFLIP2MDParserMultipleAgents tests parsing multiple agents
func TestFLIP2MDParserMultipleAgents(t *testing.T) {
	content := `# FLIP2.md

**Project:** MultiAgent
**Version:** 1.0.0

---

## Agents

### Agent Role: Analyst
- **ID Pattern:** ` + "`analyst-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs`" + `
- **Permissions:** ` + "`read-inbox`" + `
- **Max Concurrent Tasks:** 5
- **Cost Budget (USD/hour):** 2.50

### Agent Role: Reviewer
- **ID Pattern:** ` + "`reviewer-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`approve-changes`" + `
- **Permissions:** ` + "`read-inbox, send-signals`" + `
- **Max Concurrent Tasks:** 3
- **Cost Budget (USD/hour):** 4.00

### Agent Role: Monitor
- **ID Pattern:** ` + "`monitor-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, create-tasks`" + `
- **Max Concurrent Tasks:** 10
- **Cost Budget (USD/hour):** 1.50
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(config.Agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(config.Agents))
	}

	// Verify each agent
	agentNames := map[string]bool{}
	for _, agent := range config.Agents {
		agentNames[agent.Name] = true
		if agent.Model == "" {
			t.Errorf("agent %s missing model", agent.Name)
		}
		if agent.IDPattern == "" {
			t.Errorf("agent %s missing ID pattern", agent.Name)
		}
	}

	if !agentNames["Analyst"] || !agentNames["Reviewer"] || !agentNames["Monitor"] {
		t.Error("not all expected agents found")
	}
}

// TestFLIP2MDParserComplexConfiguration tests parsing a complex real-world config
func TestFLIP2MDParserComplexConfiguration(t *testing.T) {
	content := `# FLIP2.md - Data Analytics Platform

**Project:** DataAnalytics Platform
**Version:** 2.1.3
**Coordinator:** research-lead
**Last Updated:** 2026-01-02

---

## Agents

### Agent Role: Data Analyst
- **ID Pattern:** ` + "`analyst-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`" + `
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** ` + "`access-secrets, execute-destructive`" + `
- **Cost Budget (USD/hour):** 2.50
- **Description:** Processes data, generates reports, analyzes metrics from various sources.

### Agent Role: Code Reviewer
- **ID Pattern:** ` + "`reviewer-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`approve-changes`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`" + `
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** ` + "`execute-destructive`" + `
- **Cost Budget (USD/hour):** 4.00
- **Description:** Reviews code changes, validates quality, and approves merges.

### Agent Role: Research Lead
- **ID Pattern:** ` + "`research-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`spawn-workers, read-logs, external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks, modify-all-tasks, escalate, report-status`" + `
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** ` + "`execute-destructive`" + `
- **Cost Budget (USD/hour):** 5.00
- **Description:** Leads research initiatives, spawns worker teams, coordinates with other agents.

---

## Commands

### Command: /analyze
- **Aliases:** ` + "`analyze-data, check, run-analysis`" + `
- **Handler:** ` + "`analyst-worker`" + `
- **Args:** ` + "`<dataset> [--format=json|csv] [--depth=1-5]`" + `
- **Description:** Analyze dataset and generate comprehensive report with metrics, trends, and insights
- **Requires Approval:** no
- **Allowed Roles:** ` + "`analyst, research, coordinator`" + `

### Command: /review-code
- **Aliases:** ` + "`review, code-review, check-pr`" + `
- **Handler:** ` + "`reviewer-worker`" + `
- **Args:** ` + "`<pr-number> [--strict]`" + `
- **Description:** Review code changes for quality, style, architecture, and potential issues
- **Requires Approval:** no
- **Allowed Roles:** ` + "`reviewer, research, coordinator`" + `

### Command: /deploy-pipeline
- **Aliases:** ` + "`deploy, release, push-pipeline`" + `
- **Handler:** ` + "`./scripts/deploy_pipeline.sh`" + `
- **Args:** ` + "`<pipeline-name> <environment> [--dry-run]`" + `
- **Description:** Deploy data pipeline to specified environment with safety checks
- **Requires Approval:** yes
- **Allowed Roles:** ` + "`research, coordinator`" + `

---

## Routing

### Route: Fast Data Analysis
- **When:** ` + "`task.type == \"analysis\" && task.tokens_estimated < 5000 && task.complexity < 5`" + `
- **Route To:** ` + "`gemini`" + `
- **Reason:** Gemini is faster and cheaper for straightforward analysis tasks with clear requirements
- **Cost Impact:** ` + "`-0.30`" + `

### Route: Complex Data Processing
- **When:** ` + "`task.type == \"analysis\" && task.tokens_estimated >= 5000 || task.complexity >= 7`" + `
- **Route To:** ` + "`claude`" + `
- **Reason:** Claude provides superior reasoning for complex data transformations and novel insights
- **Cost Impact:** ` + "`+0.50`" + `

### Route: Code Review Expertise
- **When:** ` + "`task.type == \"review\" && task.requires_accuracy == true`" + `
- **Route To:** ` + "`claude`" + `
- **Reason:** Claude's architectural understanding and code reasoning significantly improves review quality
- **Cost Impact:** ` + "`+0.40`" + `

### Route: Urgent Research Coordination
- **When:** ` + "`task.priority == \"high\" && task.deadline < 60 && task.requires_speed == true`" + `
- **Route To:** ` + "`research`" + `
- **Reason:** Research lead provides rapid coordination and decision-making under time pressure
- **Cost Impact:** ` + "`+0.00`" + `

---

## Context

### Auto-Load Files
- ` + "`./README.md`" + ` - Project overview and quick start guide (weight: high)
- ` + "`./docs/ARCHITECTURE.md`" + ` - System design, data flow, and component relationships (weight: high)
- ` + "`./docs/DATA_MODELS.md`" + ` - Data schema and model definitions (weight: high)
- ` + "`./CODING_STANDARDS.md`" + ` - Code style guide and best practices (weight: medium)
- ` + "`./docs/API_REFERENCE.md`" + ` - API specifications and endpoint documentation (weight: high)
- ` + "`./docs/PIPELINE_GUIDE.md`" + ` - Data pipeline configuration and operations (weight: medium)
- ` + "`./.env.example`" + ` - Environment variables template (weight: low)
- ` + "`./docs/TROUBLESHOOTING.md`" + ` - Common issues and solutions (weight: low)
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify all sections populated
	if len(config.Agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(config.Agents))
	}
	if len(config.Commands) != 3 {
		t.Errorf("expected 3 commands, got %d", len(config.Commands))
	}
	if len(config.Routes) != 4 {
		t.Errorf("expected 4 routes, got %d", len(config.Routes))
	}
	if len(config.Context.AutoLoadFiles) != 8 {
		t.Errorf("expected 8 context files, got %d", len(config.Context.AutoLoadFiles))
	}

	// Check that all agents have correct model assignments
	models := make(map[string]string)
	for _, agent := range config.Agents {
		models[agent.Name] = agent.Model
	}
	if models["Data Analyst"] != "gemini" {
		t.Error("Data Analyst should use gemini model")
	}
	if models["Code Reviewer"] != "claude" {
		t.Error("Code Reviewer should use claude model")
	}
	if models["Research Lead"] != "claude" {
		t.Error("Research Lead should use claude model")
	}

	// Check command approval settings
	approvalCommands := 0
	for _, cmd := range config.Commands {
		if cmd.RequiresApproval {
			approvalCommands++
		}
	}
	if approvalCommands != 1 {
		t.Errorf("expected 1 command requiring approval, got %d", approvalCommands)
	}

	// Check context file weights
	highWeightCount := 0
	mediumWeightCount := 0
	lowWeightCount := 0
	for _, file := range config.Context.AutoLoadFiles {
		switch file.Weight {
		case "high":
			highWeightCount++
		case "medium":
			mediumWeightCount++
		case "low":
			lowWeightCount++
		}
	}
	if highWeightCount < 3 {
		t.Errorf("expected at least 3 high-weight files, got %d", highWeightCount)
	}
}

// TestFLIP2MDParserMissingProject tests error on missing project
func TestFLIP2MDParserMissingProject(t *testing.T) {
	content := `# FLIP2.md

**Version:** 1.0.0

---

## Agents

### Agent Role: Test
- **ID Pattern:** ` + "`test-*`" + `
- **Model:** claude
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	_, err := parser.Parse()
	if err == nil {
		t.Error("expected error for missing Project, got nil")
	}
}

// TestFLIP2MDParserFileNotFound tests error on missing file
func TestFLIP2MDParserFileNotFound(t *testing.T) {
	parser := NewFLIP2MDParser("/nonexistent/path/FLIP2.md")
	_, err := parser.Parse()
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// TestFLIP2MDParserEmptySections tests parsing with empty sections
func TestFLIP2MDParserEmptySections(t *testing.T) {
	content := `# FLIP2.md

**Project:** MinimalProject
**Version:** 1.0.0

---

## Agents

(No agents defined)

---

## Commands

(No commands defined)
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if config.Project != "MinimalProject" {
		t.Errorf("expected MinimalProject, got %s", config.Project)
	}
	if len(config.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(config.Agents))
	}
	if len(config.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(config.Commands))
	}
}

// TestFLIP2MDParserValidation tests schema validation integration
func TestFLIP2MDParserValidation(t *testing.T) {
	content := `# FLIP2.md

**Project:** ValidationTest
**Version:** 1.0.0

---

## Agents

### Agent Role: Test
- **ID Pattern:** ` + "`test-*`" + `
- **Model:** claude
- **Description:** Test agent

---

## Commands

### Command: /test
- **Handler:** ` + "`test-handler`" + `
- **Description:** Test command

---

## Routing

### Route: TestRoute
- **When:** ` + "`task.type == \"test\"`" + `
- **Route To:** ` + "`claude`" + `
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Config should be valid (all required fields present)
	if config == nil {
		t.Error("config is nil")
	}
}

// TestFLIP2MDParserCostImpactParsing tests cost impact with +/- prefixes
func TestFLIP2MDParserCostImpactParsing(t *testing.T) {
	content := `# FLIP2.md

**Project:** CostTest
**Version:** 1.0.0

---

## Routing

### Route: Positive Impact
- **When:** ` + "`task.type == \"expensive\"`" + `
- **Route To:** ` + "`claude`" + `
- **Cost Impact:** ` + "`+0.50`" + `

### Route: Negative Impact
- **When:** ` + "`task.type == \"cheap\"`" + `
- **Route To:** ` + "`gemini`" + `
- **Cost Impact:** ` + "`-0.30`" + `

### Route: Zero Impact
- **When:** ` + "`task.type == \"neutral\"`" + `
- **Route To:** ` + "`gpt-4`" + `
- **Cost Impact:** ` + "`0.00`" + `
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(config.Routes) != 3 {
		t.Errorf("expected 3 routes, got %d", len(config.Routes))
	}

	// Find each route and verify cost impact
	for _, route := range config.Routes {
		switch route.Name {
		case "Positive Impact":
			if route.CostImpact != 0.50 {
				t.Errorf("positive impact: expected 0.50, got %f", route.CostImpact)
			}
		case "Negative Impact":
			if route.CostImpact != -0.30 {
				t.Errorf("negative impact: expected -0.30, got %f", route.CostImpact)
			}
		case "Zero Impact":
			if route.CostImpact != 0.00 {
				t.Errorf("zero impact: expected 0.00, got %f", route.CostImpact)
			}
		}
	}
}

// TestFLIP2MDParserContextWeights tests context file weight parsing
func TestFLIP2MDParserContextWeights(t *testing.T) {
	content := `# FLIP2.md

**Project:** WeightTest
**Version:** 1.0.0

---

## Context

### Auto-Load Files
- ` + "`./critical.md`" + ` - Critical file (weight: high)
- ` + "`./standard.md`" + ` - Standard file (weight: medium)
- ` + "`./optional.md`" + ` - Optional file (weight: low)
- ` + "`./default.md`" + ` - Default weight file
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	parser := NewFLIP2MDParser(tmpFile)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(config.Context.AutoLoadFiles) != 4 {
		t.Errorf("expected 4 context files, got %d", len(config.Context.AutoLoadFiles))
	}

	weights := make(map[string]string)
	for _, file := range config.Context.AutoLoadFiles {
		weights[file.Path] = file.Weight
	}

	if weights["./critical.md"] != "high" {
		t.Errorf("critical.md: expected weight 'high', got '%s'", weights["./critical.md"])
	}
	if weights["./standard.md"] != "medium" {
		t.Errorf("standard.md: expected weight 'medium', got '%s'", weights["./standard.md"])
	}
	if weights["./optional.md"] != "low" {
		t.Errorf("optional.md: expected weight 'low', got '%s'", weights["./optional.md"])
	}
	if weights["./default.md"] != "medium" {
		t.Errorf("default.md: expected weight 'medium', got '%s'", weights["./default.md"])
	}
}

// TestFLIP2MDParserIntegration tests the parser with example FLIP2.md
func TestFLIP2MDParserIntegration(t *testing.T) {
	// Try multiple possible paths
	examplePath := "/Users/arielspivakovsky/src/flip/flip2/examples/example.FLIP2.md"
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Skipf("example file not found at %s", examplePath)
	}

	parser := NewFLIP2MDParser(examplePath)
	config, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Verify basic structure
	if config.Project == "" {
		t.Error("Project not parsed")
	}
	if len(config.Agents) == 0 {
		t.Error("No agents parsed")
	}
	if len(config.Commands) == 0 {
		t.Error("No commands parsed")
	}
	if len(config.Routes) == 0 {
		t.Error("No routes parsed")
	}
}
