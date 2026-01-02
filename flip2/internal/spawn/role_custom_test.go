package spawn

import (
	"testing"

	"flip2/internal/config"
)

// TestLoadCustomRolesBasic tests loading a basic custom role from ProjectConfig
func TestLoadCustomRolesBasic(t *testing.T) {
	cfg := &config.ProjectConfig{
		Agents: []config.AgentRole{
			{
				Name:        "security-auditor",
				Description: "Security code reviewer focused on vulnerabilities",
				Model:       "claude-sonnet-4",
				Capabilities: []string{
					"read-code",
					"identify-vulnerabilities",
					"suggest-fixes",
				},
				Permissions: []string{
					"read-inbox",
					"send-signals",
				},
				EscalationRequired: []string{
					"modify-code",
					"execute-destructive",
				},
			},
		},
	}

	customRoles, err := LoadCustomRoles(cfg)
	if err != nil {
		t.Fatalf("LoadCustomRoles failed: %v", err)
	}

	if len(customRoles) != 1 {
		t.Errorf("expected 1 custom role, got %d", len(customRoles))
	}

	role, exists := customRoles["security-auditor"]
	if !exists {
		t.Fatal("security-auditor role not found")
	}

	if role.Name != "security-auditor" {
		t.Errorf("role name mismatch: expected 'security-auditor', got %q", role.Name)
	}

	if role.Description != "Security code reviewer focused on vulnerabilities" {
		t.Errorf("role description mismatch: expected specific description, got %q", role.Description)
	}

	if role.Model != "claude-sonnet-4" {
		t.Errorf("role model mismatch: expected 'claude-sonnet-4', got %q", role.Model)
	}

	if role.SystemPrompt == "" {
		t.Error("role system prompt is empty")
	}

	if role.MaxTokens <= 0 {
		t.Errorf("role max tokens invalid: %d", role.MaxTokens)
	}

	// Check permissions were converted
	if len(role.Permissions.CanRead) == 0 {
		t.Error("role should have read permissions")
	}

	if len(role.Permissions.CanExecute) == 0 {
		t.Error("role should have execute permissions")
	}
}

// TestLoadCustomRolesMultiple tests loading multiple custom roles
func TestLoadCustomRolesMultiple(t *testing.T) {
	cfg := &config.ProjectConfig{
		Agents: []config.AgentRole{
			{
				Name:         "analyzer",
				Description:  "Data analyst role",
				Model:        "gemini-2.5-pro",
				Capabilities: []string{"analyze", "report"},
				Permissions:  []string{"read-inbox"},
			},
			{
				Name:         "executor",
				Description:  "Code executor role",
				Model:        "claude-opus-4",
				Capabilities: []string{"execute"},
				Permissions:  []string{"read-inbox", "send-signals"},
			},
		},
	}

	customRoles, err := LoadCustomRoles(cfg)
	if err != nil {
		t.Fatalf("LoadCustomRoles failed: %v", err)
	}

	if len(customRoles) != 2 {
		t.Errorf("expected 2 custom roles, got %d", len(customRoles))
	}

	// Verify both roles exist
	if _, exists := customRoles["analyzer"]; !exists {
		t.Error("analyzer role not found")
	}
	if _, exists := customRoles["executor"]; !exists {
		t.Error("executor role not found")
	}
}

// TestLoadCustomRolesNilConfig tests handling of nil config
func TestLoadCustomRolesNilConfig(t *testing.T) {
	customRoles, err := LoadCustomRoles(nil)
	if err != nil {
		t.Fatalf("LoadCustomRoles with nil config should not error: %v", err)
	}

	if len(customRoles) != 0 {
		t.Errorf("expected empty map for nil config, got %d roles", len(customRoles))
	}
}

// TestLoadCustomRolesEmptyConfig tests handling of empty config
func TestLoadCustomRolesEmptyConfig(t *testing.T) {
	cfg := &config.ProjectConfig{
		Agents: []config.AgentRole{},
	}

	customRoles, err := LoadCustomRoles(cfg)
	if err != nil {
		t.Fatalf("LoadCustomRoles with empty agents should not error: %v", err)
	}

	if len(customRoles) != 0 {
		t.Errorf("expected empty map for empty agents, got %d roles", len(customRoles))
	}
}

// TestLoadCustomRolesInvalidName tests validation of role name requirement
func TestLoadCustomRolesInvalidName(t *testing.T) {
	cfg := &config.ProjectConfig{
		Agents: []config.AgentRole{
			{
				Name:        "", // Invalid: empty name
				Description: "Role without name",
				Model:       "claude-sonnet-4",
			},
		},
	}

	_, err := LoadCustomRoles(cfg)
	if err == nil {
		t.Error("LoadCustomRoles should fail for empty role name")
	}
}

// TestLoadCustomRolesInvalidDescription tests validation of description requirement
func TestLoadCustomRolesInvalidDescription(t *testing.T) {
	cfg := &config.ProjectConfig{
		Agents: []config.AgentRole{
			{
				Name:        "test-role",
				Description: "", // Invalid: empty description
				Model:       "claude-sonnet-4",
			},
		},
	}

	_, err := LoadCustomRoles(cfg)
	if err == nil {
		t.Error("LoadCustomRoles should fail for empty description")
	}
}

// TestMergeRolesBasic tests merging custom roles with builtins
func TestMergeRolesBasic(t *testing.T) {
	customRoles := map[string]*RoleTemplate{
		"security-auditor": {
			Name:         "security-auditor",
			Description:  "Security reviewer",
			SystemPrompt: "You are a security reviewer",
			Model:        "claude-sonnet-4",
			MaxTokens:    8192,
			Permissions: Permissions{
				CanRead:    []string{"**/*.go"},
				CanWrite:   []string{"security/*"},
				CanExecute: []string{"signal:send"},
			},
		},
	}

	merged := MergeRoles(customRoles)

	// Check builtins are present
	expectedBuiltins := []string{"code-reviewer", "researcher", "implementer", "gemini-flash-worker", "haiku-worker"}
	for _, builtin := range expectedBuiltins {
		if _, exists := merged[builtin]; !exists {
			t.Errorf("builtin role %q not found in merged roles", builtin)
		}
	}

	// Check custom role is present
	if _, exists := merged["security-auditor"]; !exists {
		t.Error("custom role security-auditor not found in merged roles")
	}

	// Verify total count
	expectedCount := len(expectedBuiltins) + 1 // 5 builtins + 1 custom
	if len(merged) != expectedCount {
		t.Errorf("expected %d merged roles, got %d", expectedCount, len(merged))
	}
}

// TestMergeRolesOverride tests that custom roles override builtins
func TestMergeRolesOverride(t *testing.T) {
	customRoles := map[string]*RoleTemplate{
		"code-reviewer": { // Override builtin
			Name:         "code-reviewer",
			Description:  "Custom security-focused code reviewer",
			SystemPrompt: "You are a custom code reviewer",
			Model:        "claude-opus-4",
			MaxTokens:    16384,
			Permissions: Permissions{
				CanRead:    []string{"**/*.go", "**/*.py"},
				CanWrite:   []string{"security-reviews/*"},
				CanExecute: []string{"signal:send", "task:report"},
			},
		},
	}

	merged := MergeRoles(customRoles)

	// Check that custom version is used
	reviewerRole := merged["code-reviewer"]
	if reviewerRole.MaxTokens != 16384 {
		t.Errorf("custom role override failed: expected MaxTokens 16384, got %d", reviewerRole.MaxTokens)
	}

	if reviewerRole.Model != "claude-opus-4" {
		t.Errorf("custom role override failed: expected model claude-opus-4, got %q", reviewerRole.Model)
	}

	if reviewerRole.Description != "Custom security-focused code reviewer" {
		t.Errorf("custom role override failed: description not overridden")
	}
}

// TestMergeRolesEmpty tests merging empty custom roles
func TestMergeRolesEmpty(t *testing.T) {
	customRoles := make(map[string]*RoleTemplate)
	merged := MergeRoles(customRoles)

	// Should have all builtins
	expectedBuiltins := []string{"code-reviewer", "researcher", "implementer", "gemini-flash-worker", "haiku-worker"}
	if len(merged) != len(expectedBuiltins) {
		t.Errorf("expected %d builtins, got %d", len(expectedBuiltins), len(merged))
	}

	for _, builtin := range expectedBuiltins {
		if _, exists := merged[builtin]; !exists {
			t.Errorf("builtin role %q not found", builtin)
		}
	}
}

// TestGenerateSystemPrompt tests system prompt generation from AgentRole
func TestGenerateSystemPrompt(t *testing.T) {
	agentRole := config.AgentRole{
		Name:        "auditor",
		Description: "Security auditor",
		Capabilities: []string{
			"code-review",
			"vulnerability-detection",
		},
		EscalationRequired: []string{
			"execute-fixes",
			"modify-production",
		},
	}

	prompt := generateSystemPrompt(agentRole)

	// Verify prompt contains key elements
	if !contains(prompt, "auditor") {
		t.Error("prompt should contain role name")
	}

	if !contains(prompt, "Security auditor") {
		t.Error("prompt should contain role description")
	}

	if !contains(prompt, "code-review") {
		t.Error("prompt should contain capabilities")
	}

	if !contains(prompt, "execute-fixes") {
		t.Error("prompt should contain escalation requirements")
	}

	if !contains(prompt, "coordinator") {
		t.Error("prompt should reference coordinator")
	}

	if !contains(prompt, "Constraints") && !contains(prompt, "constraints") {
		t.Error("prompt should include constraints section")
	}
}

// TestGenerateSystemPromptEmpty tests system prompt generation with empty AgentRole
func TestGenerateSystemPromptEmpty(t *testing.T) {
	agentRole := config.AgentRole{}
	prompt := generateSystemPrompt(agentRole)

	if prompt == "" {
		t.Error("prompt should not be empty even for empty role")
	}

	if !contains(prompt, "FLIP2") {
		t.Error("prompt should reference FLIP2 system")
	}
}

// TestConvertAgentPermissions tests permission conversion
func TestConvertAgentPermissions(t *testing.T) {
	agentRole := config.AgentRole{
		Name: "developer",
		Permissions: []string{
			"read-inbox",
			"send-signals",
			"create-tasks",
			"execute-code",
		},
	}

	perms := convertAgentPermissions(agentRole)

	// Check read permissions
	if len(perms.CanRead) == 0 {
		t.Error("should have read permissions")
	}

	// Check execute permissions
	hasSignalSend := false
	hasTaskCreate := false
	for _, perm := range perms.CanExecute {
		if perm == "signal:send" {
			hasSignalSend = true
		}
		if perm == "task:create" {
			hasTaskCreate = true
		}
	}
	if !hasSignalSend {
		t.Error("should have signal:send permission")
	}
	if !hasTaskCreate {
		t.Error("should have task:create permission")
	}

	// Check write permissions for code execution
	hasGoWrite := false
	for _, perm := range perms.CanWrite {
		if perm == "**/*.go" {
			hasGoWrite = true
			break
		}
	}
	if !hasGoWrite {
		t.Error("should have **/*.go write permission for execute-code")
	}
}

// TestConvertAgentPermissionsNoExecuteCode tests permission conversion without execute-code
func TestConvertAgentPermissionsNoExecuteCode(t *testing.T) {
	agentRole := config.AgentRole{
		Name: "reader",
		Permissions: []string{
			"read-inbox",
		},
	}

	perms := convertAgentPermissions(agentRole)

	// Should have safe default for write
	if len(perms.CanWrite) == 0 {
		t.Error("should have default write permissions")
	}

	// Should be restricted to work/*
	hasWorkWrite := false
	for _, perm := range perms.CanWrite {
		if perm == "work/*" {
			hasWorkWrite = true
			break
		}
	}
	if !hasWorkWrite {
		t.Error("should restrict write to work/* by default")
	}
}

// TestLoadAndMergeCustomRoles tests full workflow
func TestLoadAndMergeCustomRoles(t *testing.T) {
	cfg := &config.ProjectConfig{
		Agents: []config.AgentRole{
			{
				Name:        "analyzer",
				Description: "Data analyzer",
				Model:       "gemini-2.5-pro",
				Permissions: []string{"read-inbox", "send-signals"},
			},
			{
				Name:        "implementer",
				Description: "Code implementer (custom)",
				Model:       "claude-opus-4",
				Permissions: []string{"read-inbox", "send-signals", "execute-code"},
			},
		},
	}

	// Load custom roles
	customRoles, err := LoadCustomRoles(cfg)
	if err != nil {
		t.Fatalf("LoadCustomRoles failed: %v", err)
	}

	if len(customRoles) != 2 {
		t.Errorf("expected 2 custom roles, got %d", len(customRoles))
	}

	// Merge with builtins
	merged := MergeRoles(customRoles)

	// Should have builtins + custom
	// 5 builtins (code-reviewer, researcher, implementer, gemini-flash-worker, haiku-worker) + 2 custom (analyzer, implementer override)
	// Total: 6 roles (gemini-flash-worker, haiku-worker, code-reviewer, analyzer, implementer (overridden), researcher)
	expectedCount := 6 // 5 builtins + 2 custom - 1 override
	if len(merged) != expectedCount {
		t.Errorf("expected %d merged roles, got %d", expectedCount, len(merged))
	}

	// Check that custom implementer overrides builtin
	impl := merged["implementer"]
	if impl.Description != "Code implementer (custom)" {
		t.Error("custom implementer should override builtin")
	}

	// Check that analyzer is present
	if _, exists := merged["analyzer"]; !exists {
		t.Error("custom analyzer role should be in merged set")
	}
}


// TestRoleTemplateValidateRequiredFields tests that all required fields are validated
func TestRoleTemplateValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		role        *RoleTemplate
		shouldError bool
	}{
		{
			name: "valid role",
			role: &RoleTemplate{
				Name:         "test",
				Description:  "test role",
				SystemPrompt: "you are test",
				MaxTokens:    4096,
			},
			shouldError: false,
		},
		{
			name: "missing name",
			role: &RoleTemplate{
				Description:  "test",
				SystemPrompt: "you are test",
				MaxTokens:    4096,
			},
			shouldError: true,
		},
		{
			name: "missing description",
			role: &RoleTemplate{
				Name:         "test",
				SystemPrompt: "you are test",
				MaxTokens:    4096,
			},
			shouldError: true,
		},
		{
			name: "missing system prompt",
			role: &RoleTemplate{
				Name:        "test",
				Description: "test",
				MaxTokens:   4096,
			},
			shouldError: true,
		},
		{
			name: "invalid max tokens",
			role: &RoleTemplate{
				Name:         "test",
				Description:  "test",
				SystemPrompt: "you are test",
				MaxTokens:    0,
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if (err != nil) != tt.shouldError {
				t.Errorf("validation error mismatch: expected error=%v, got error=%v", tt.shouldError, err != nil)
			}
		})
	}
}
