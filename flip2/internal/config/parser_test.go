package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseValidFLIP2MD tests parsing a valid FLIP2.md file
func TestParseValidFLIP2MD(t *testing.T) {
	content := `# FLIP2.md - Project Configuration

**Project:** TestProject
**Version:** 1.0
**Coordinator:** claude-coordinator
**Last Updated:** 2026-01-01

---

## Agents

### Agent Role: Analyst
- **ID Pattern:** ` + "`analyst-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks, modify-own-tasks`" + `
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** ` + "`access-secrets, execute-destructive`" + `
- **Cost Budget (USD/hour):** 2.50
- **Description:** Data analyst role

### Agent Role: Reviewer
- **ID Pattern:** ` + "`reviewer-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`approve-changes`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks`" + `
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** ` + "`execute-destructive`" + `
- **Cost Budget (USD/hour):** 4.00
- **Description:** Code reviewer role

---

## Commands

### Command: /analyze
- **Aliases:** ` + "`logs, check`" + `
- **Handler:** ` + "`analyst-worker`" + `
- **Args:** ` + "`<dataset> [--format=json|csv]`" + `
- **Description:** Analyze dataset and generate report
- **Requires Approval:** no
- **Allowed Roles:** ` + "`analyst, coordinator`" + `

### Command: /approve
- **Aliases:** ` + "`accept`" + `
- **Handler:** ` + "`./scripts/approve.sh`" + `
- **Args:** ` + "`<item-id> [--reason=TEXT]`" + `
- **Description:** Approve item for next stage
- **Requires Approval:** yes
- **Allowed Roles:** ` + "`reviewer, coordinator`" + `

---

## Routing

### Route: Fast Analysis
- **When:** ` + "`task.complexity < 5 && task.tokens_estimated < 5000`" + `
- **Route To:** ` + "`gemini`" + `
- **Reason:** Faster and cheaper for simple tasks
- **Cost Impact:** ` + "`-0.30`" + `

### Route: Complex Debugging
- **When:** ` + "`task.type == \"debugging\" && task.complexity > 7`" + `
- **Route To:** ` + "`claude`" + `
- **Reason:** Claude handles complex reasoning better
- **Cost Impact:** ` + "`+0.50`" + `

---

## Context

### Auto-Load Files
- ` + "`./README.md`" + ` - Project overview (weight: high)
- ` + "`./docs/ARCHITECTURE.md`" + ` - System design (weight: high)
- ` + "`./CODING_STANDARDS.md`" + ` - Code style guide (weight: medium)
`

	// Create temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Parse the file
	config, err := ParseFLIP2MD(tmpFile)
	if err != nil {
		t.Fatalf("ParseFLIP2MD failed: %v", err)
	}

	// Verify header metadata
	if config.Project != "TestProject" {
		t.Errorf("Expected Project 'TestProject', got %q", config.Project)
	}
	if config.Version != "1.0" {
		t.Errorf("Expected Version '1.0', got %q", config.Version)
	}
	if config.Coordinator != "claude-coordinator" {
		t.Errorf("Expected Coordinator 'claude-coordinator', got %q", config.Coordinator)
	}

	// Verify agents
	if len(config.Agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(config.Agents))
	}

	if config.Agents[0].Name != "Analyst" {
		t.Errorf("Expected first agent 'Analyst', got %q", config.Agents[0].Name)
	}
	if config.Agents[0].Model != "gemini" {
		t.Errorf("Expected model 'gemini', got %q", config.Agents[0].Model)
	}
	if config.Agents[0].IDPattern != "analyst-*" {
		t.Errorf("Expected ID pattern 'analyst-*', got %q", config.Agents[0].IDPattern)
	}
	if config.Agents[0].MaxConcurrentTasks != 5 {
		t.Errorf("Expected max concurrent tasks 5, got %d", config.Agents[0].MaxConcurrentTasks)
	}
	if config.Agents[0].CostBudgetPerHour != 2.50 {
		t.Errorf("Expected cost budget 2.50, got %f", config.Agents[0].CostBudgetPerHour)
	}

	// Verify commands
	if len(config.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(config.Commands))
	}

	if config.Commands[0].Name != "/analyze" {
		t.Errorf("Expected first command '/analyze', got %q", config.Commands[0].Name)
	}
	if config.Commands[0].Handler != "analyst-worker" {
		t.Errorf("Expected handler 'analyst-worker', got %q", config.Commands[0].Handler)
	}
	if config.Commands[0].RequiresApproval != false {
		t.Errorf("Expected RequiresApproval false, got %v", config.Commands[0].RequiresApproval)
	}

	if config.Commands[1].RequiresApproval != true {
		t.Errorf("Expected second command RequiresApproval true, got %v", config.Commands[1].RequiresApproval)
	}

	// Verify routes
	if len(config.Routes) != 2 {
		t.Fatalf("Expected 2 routes, got %d", len(config.Routes))
	}

	if config.Routes[0].Name != "Fast Analysis" {
		t.Errorf("Expected first route 'Fast Analysis', got %q", config.Routes[0].Name)
	}
	if config.Routes[0].RouteTo != "gemini" {
		t.Errorf("Expected route to 'gemini', got %q", config.Routes[0].RouteTo)
	}
	if config.Routes[0].CostImpact != -0.30 {
		t.Errorf("Expected cost impact -0.30, got %f", config.Routes[0].CostImpact)
	}

	// Verify context
	if len(config.Context.AutoLoadFiles) != 3 {
		t.Fatalf("Expected 3 context files, got %d", len(config.Context.AutoLoadFiles))
	}

	if config.Context.AutoLoadFiles[0].Path != "./README.md" {
		t.Errorf("Expected first file './README.md', got %q", config.Context.AutoLoadFiles[0].Path)
	}
	if config.Context.AutoLoadFiles[0].Weight != "high" {
		t.Errorf("Expected weight 'high', got %q", config.Context.AutoLoadFiles[0].Weight)
	}
}

// TestParseMalformedMarkdown tests handling of malformed markdown
func TestParseMalformedMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no sections",
			content: `# FLIP2.md\n\n**Project:** Test`,
			wantErr: true,
			errMsg:  "no sections found",
		},
		{
			name: "missing agent model",
			content: `# FLIP2.md

**Project:** Test
**Version:** 1.0

## Agents

### Agent Role: BadAgent
- **ID Pattern:** ` + "`bad-*`" + `
- **Description:** Missing model
`,
			wantErr: true,
			errMsg:  "is missing Model",
		},
		{
			name: "duplicate agent ID",
			content: `# FLIP2.md

**Project:** Test
**Version:** 1.0

## Agents

### Agent Role: Agent1
- **ID Pattern:** ` + "`same-*`" + `
- **Model:** gemini

### Agent Role: Agent2
- **ID Pattern:** ` + "`same-*`" + `
- **Model:** claude
`,
			wantErr: true,
			errMsg:  "duplicate agent ID",
		},
		{
			name: "missing required agent field",
			content: `# FLIP2.md

**Project:** Test
**Version:** 1.0

## Agents

### Agent Role: BadAgent
- **ID Pattern:** ` + "`bad-*`" + `
- **Description:** Missing model

## Commands

### Command: /test
- **Handler:** ` + "`./script.sh`" + `
`,
			wantErr: true,
			errMsg:  "is missing Model",
		},
		{
			name: "invalid context weight",
			content: `# FLIP2.md

**Project:** Test
**Version:** 1.0

## Context

### Auto-Load Files
- ` + "`./file.md`" + ` - Description (weight: invalid)
`,
			wantErr: true,
			errMsg:  "invalid weight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "FLIP2.md")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			_, err := ParseFLIP2MD(tmpFile)
			if (err == nil) != !tt.wantErr {
				t.Errorf("ParseFLIP2MD() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseFLIP2MD() error message = %q, want containing %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestParseYAMLExtraction tests YAML block extraction capability
func TestParseYAMLExtraction(t *testing.T) {
	content := `# FLIP2.md

**Project:** YAMLTest
**Version:** 1.0

## Agents

### Agent Role: Worker
- **ID Pattern:** ` + "`worker-*`" + `
- **Model:** gemini
- **Capabilities:** ` + "`read-logs, external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`" + `
- **Max Concurrent Tasks:** 10
- **Escalation Required For:** ` + "`access-secrets`" + `
- **Cost Budget (USD/hour):** 1.50
- **Description:** General purpose worker
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	config, err := ParseFLIP2MD(tmpFile)
	if err != nil {
		t.Fatalf("ParseFLIP2MD failed: %v", err)
	}

	// Verify capabilities were parsed as list
	if len(config.Agents[0].Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(config.Agents[0].Capabilities))
	}
	if config.Agents[0].Capabilities[0] != "read-logs" {
		t.Errorf("Expected first capability 'read-logs', got %q", config.Agents[0].Capabilities[0])
	}

	// Verify permissions were parsed as list
	if len(config.Agents[0].Permissions) != 5 {
		t.Errorf("Expected 5 permissions, got %d", len(config.Agents[0].Permissions))
	}
}

// TestParseMarkdownTables tests markdown table extraction
func TestParseMarkdownTables(t *testing.T) {
	content := `# FLIP2.md

**Project:** TableTest
**Version:** 1.0

## Agents

### Agent Role: Analyzer
- **ID Pattern:** ` + "`analyzer-*`" + `
- **Model:** claude
- **Capabilities:** ` + "`external-api-calls`" + `
- **Permissions:** ` + "`read-inbox, send-signals`" + `
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** ` + "`execute-destructive`" + `
- **Cost Budget (USD/hour):** 3.75
- **Description:** Analysis specialist

## Commands

### Command: /report
- **Aliases:** ` + "`generate, create, build`" + `
- **Handler:** ` + "`analyzer-worker`" + `
- **Args:** ` + "`<report-type> [--detailed]`" + `
- **Description:** Generate analysis report
- **Requires Approval:** no
- **Allowed Roles:** ` + "`analyzer`" + `
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	config, err := ParseFLIP2MD(tmpFile)
	if err != nil {
		t.Fatalf("ParseFLIP2MD failed: %v", err)
	}

	// Verify command aliases were parsed correctly
	if len(config.Commands[0].Aliases) != 3 {
		t.Errorf("Expected 3 aliases, got %d: %v", len(config.Commands[0].Aliases), config.Commands[0].Aliases)
	}
	if len(config.Commands[0].Aliases) > 0 && config.Commands[0].Aliases[0] != "generate" {
		t.Errorf("Expected first alias 'generate', got %q", config.Commands[0].Aliases[0])
	}
}

// TestParseEmptyOptionalSections tests handling of optional sections
func TestParseEmptyOptionalSections(t *testing.T) {
	content := `# FLIP2.md

**Project:** MinimalProject
**Version:** 1.0

## Agents

### Agent Role: Worker
- **ID Pattern:** ` + "`worker-*`" + `
- **Model:** gemini
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "FLIP2.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	config, err := ParseFLIP2MD(tmpFile)
	if err != nil {
		t.Fatalf("ParseFLIP2MD failed: %v", err)
	}

	// Verify empty optional sections are handled gracefully
	if len(config.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(config.Agents))
	}
	if len(config.Commands) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(config.Commands))
	}
	if len(config.Routes) != 0 {
		t.Errorf("Expected 0 routes, got %d", len(config.Routes))
	}
	if len(config.Context.AutoLoadFiles) != 0 {
		t.Errorf("Expected 0 context files, got %d", len(config.Context.AutoLoadFiles))
	}
}

// TestSchemaValidation tests schema validation rules
func TestSchemaValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *ProjectConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid config",
			config: &ProjectConfig{
				Agents: []AgentRole{
					{Name: "Agent1", IDPattern: "agent1-*", Model: "claude"},
				},
				Commands: []Command{
					{Name: "/cmd1", Handler: "agent1-worker"},
				},
				Routes: []Route{
					{Name: "Route1", Condition: "task.type == 'test'", RouteTo: "claude"},
				},
				Context: ContextConfig{
					AutoLoadFiles: []ContextFile{
						{Path: "./README.md", Weight: "high"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "agent missing ID pattern",
			config: &ProjectConfig{
				Agents: []AgentRole{
					{Name: "BadAgent", Model: "claude"},
				},
			},
			wantErr:   true,
			errSubstr: "missing ID Pattern",
		},
		{
			name: "command missing handler",
			config: &ProjectConfig{
				Commands: []Command{
					{Name: "/cmd1"},
				},
			},
			wantErr:   true,
			errSubstr: "missing Handler",
		},
		{
			name: "route missing condition",
			config: &ProjectConfig{
				Routes: []Route{
					{Name: "Route1", RouteTo: "claude"},
				},
			},
			wantErr:   true,
			errSubstr: "missing When",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchema(tt.config)
			if (err == nil) != !tt.wantErr {
				t.Errorf("validateSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && !contains(err.Error(), tt.errSubstr) {
				t.Errorf("validateSchema() error message = %q, want containing %q", err.Error(), tt.errSubstr)
			}
		})
	}
}
