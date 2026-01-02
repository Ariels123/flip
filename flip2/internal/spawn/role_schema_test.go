package spawn

import (
	"strings"
	"testing"
)

// TestValidateRoleNameFormatValid tests valid role names.
func TestValidateRoleNameFormatValid(t *testing.T) {
	validNames := []string{
		"code-reviewer",
		"security-auditor",
		"data-analyst",
		"implementer",
		"researcher",
		"worker1",
		"test-role-123",
		"a",
		"z",
		"1",
		"a-z-1",
	}

	for _, name := range validNames {
		if err := ValidateRoleNameFormat(name); err != nil {
			t.Errorf("ValidateRoleNameFormat(%q) returned error for valid name: %v", name, err)
		}
	}
}

// TestValidateRoleNameFormatInvalid tests invalid role names.
func TestValidateRoleNameFormatInvalid(t *testing.T) {
	invalidNames := []string{
		"",                    // empty
		"CodeReviewer",        // uppercase
		"Code Reviewer",       // space
		"code_reviewer",       // underscore
		"code.reviewer",       // dot
		"-code-reviewer",      // starts with hyphen
		"code-reviewer-",      // ends with hyphen
		strings.Repeat("a", 129), // too long
	}

	for _, name := range invalidNames {
		if err := ValidateRoleNameFormat(name); err == nil {
			t.Errorf("ValidateRoleNameFormat(%q) should have failed for invalid name", name)
		}
	}
}

// TestValidateSystemPromptValid tests valid system prompts.
func TestValidateSystemPromptValid(t *testing.T) {
	validPrompts := []string{
		strings.Repeat("a", 50),   // minimum length
		strings.Repeat("a", 8192), // maximum length
		"You are a code reviewer. Focus on bugs and style issues.",
		"This is a test prompt that is long enough to pass validation and covers the minimum requirements.",
	}

	minLen := 50
	maxLen := 8192

	for _, prompt := range validPrompts {
		if err := ValidateSystemPrompt(prompt, minLen, maxLen); err != nil {
			t.Errorf("ValidateSystemPrompt() returned error for valid prompt: %v", err)
		}
	}
}

// TestValidateSystemPromptInvalid tests invalid system prompts.
func TestValidateSystemPromptInvalid(t *testing.T) {
	minLen := 50
	maxLen := 8192

	tests := []struct {
		prompt string
		desc   string
	}{
		{"", "empty prompt"},
		{strings.Repeat("a", 49), "too short"},
		{strings.Repeat("a", 8193), "too long"},
	}

	for _, test := range tests {
		if err := ValidateSystemPrompt(test.prompt, minLen, maxLen); err == nil {
			t.Errorf("ValidateSystemPrompt() should fail for %s", test.desc)
		}
	}
}

// TestValidateModelValid tests valid model names.
func TestValidateModelValid(t *testing.T) {
	allowedModels := []string{
		"claude-opus-4-5",
		"claude-sonnet-4",
		"gemini-2.5-pro",
	}

	validModels := []string{
		"",                    // empty is okay (uses default)
		"claude-opus-4-5",     // in allowed list
		"claude-sonnet-4",     // in allowed list
		"gemini-2.5-pro",      // in allowed list
	}

	for _, model := range validModels {
		if err := ValidateModel(model, allowedModels); err != nil {
			t.Errorf("ValidateModel(%q) returned error for valid model: %v", model, err)
		}
	}
}

// TestValidateModelInvalid tests invalid model names.
func TestValidateModelInvalid(t *testing.T) {
	allowedModels := []string{
		"claude-opus-4-5",
		"claude-sonnet-4",
	}

	invalidModels := []string{
		"gpt-4",                     // not in allowed list
		"unknown-model",             // not in allowed list
		strings.Repeat("a", 257),    // too long
	}

	for _, model := range invalidModels {
		if err := ValidateModel(model, allowedModels); err == nil {
			t.Errorf("ValidateModel(%q) should fail for invalid model", model)
		}
	}
}

// TestValidateResourcePatternValid tests valid resource patterns.
func TestValidateResourcePatternValid(t *testing.T) {
	validPatterns := []string{
		"**/*",
		"**/*.go",
		"config/*",
		"logs/worker/*",
		"src/main/**/*.py",
		".eslintrc",
		"package-lock.json",
		"work:folder",
		"data/*.csv",
	}

	for _, pattern := range validPatterns {
		if err := ValidateResourcePattern(pattern); err != nil {
			t.Errorf("ValidateResourcePattern(%q) returned error: %v", pattern, err)
		}
	}
}

// TestValidateResourcePatternInvalid tests invalid resource patterns.
func TestValidateResourcePatternInvalid(t *testing.T) {
	invalidPatterns := []string{
		"",              // empty
		"file@name",     // invalid char @
		"file#name",     // invalid char #
		"file$name",     // invalid char $
		"file(name)",    // invalid char ()
	}

	for _, pattern := range invalidPatterns {
		if err := ValidateResourcePattern(pattern); err == nil {
			t.Errorf("ValidateResourcePattern(%q) should fail for invalid pattern", pattern)
		}
	}
}

// TestValidatePermissionsValid tests valid permission sets.
func TestValidatePermissionsValid(t *testing.T) {
	validPermissions := []Permissions{
		{
			CanRead:    []string{"**/*"},
			CanWrite:   []string{"logs/*"},
			CanExecute: []string{"signal:send"},
		},
		{
			CanRead:    []string{"**/*.go", "**/*.py"},
			CanWrite:   []string{"src/**/*"},
			CanExecute: []string{"task:create", "signal:send"},
		},
		{
			CanRead:    []string{},
			CanWrite:   []string{},
			CanExecute: []string{},
		},
	}

	for _, perms := range validPermissions {
		if err := ValidatePermissions(perms); err != nil {
			t.Errorf("ValidatePermissions() returned error: %v", err)
		}
	}
}

// TestValidatePermissionsInvalid tests invalid permission sets.
func TestValidatePermissionsInvalid(t *testing.T) {
	invalidPermissions := []struct {
		perms Permissions
		desc  string
	}{
		{
			Permissions{
				CanRead: []string{"file@name"},
			},
			"invalid read pattern",
		},
		{
			Permissions{
				CanWrite: []string{"file#name"},
			},
			"invalid write pattern",
		},
		{
			Permissions{
				CanExecute: []string{"invalid"},  // missing colon
			},
			"invalid execute permission format",
		},
	}

	for _, test := range invalidPermissions {
		if err := ValidatePermissions(test.perms); err == nil {
			t.Errorf("ValidatePermissions() should fail for %s", test.desc)
		}
	}
}

// TestValidateResourceLimitsValid tests valid resource limits.
func TestValidateResourceLimitsValid(t *testing.T) {
	validLimits := []ResourceLimit{
		{
			TimeoutSeconds:       300,
			TokenBudget:          4096,
			ConcurrentExecutions: 1,
			MaxRetries:           3,
		},
		{
			TimeoutSeconds:       3600,
			TokenBudget:          65536,
			ConcurrentExecutions: 100,
			MaxRetries:           5,
		},
		{
			TimeoutSeconds:       0,
			TokenBudget:          0,
			ConcurrentExecutions: 1,
			MaxRetries:           0,
		},
	}

	maxTokens := 65536
	for _, limits := range validLimits {
		if err := ValidateResourceLimits(limits, maxTokens); err != nil {
			t.Errorf("ValidateResourceLimits() returned error: %v", err)
		}
	}
}

// TestValidateResourceLimitsInvalid tests invalid resource limits.
func TestValidateResourceLimitsInvalid(t *testing.T) {
	maxTokens := 65536

	invalidLimits := []struct {
		limits ResourceLimit
		desc   string
	}{
		{
			ResourceLimit{
				TimeoutSeconds:       -1,
				ConcurrentExecutions: 1,
			},
			"negative timeout",
		},
		{
			ResourceLimit{
				TimeoutSeconds:       3601,
				ConcurrentExecutions: 1,
			},
			"timeout exceeds max",
		},
		{
			ResourceLimit{
				TokenBudget:          -1,
				ConcurrentExecutions: 1,
			},
			"negative token budget",
		},
		{
			ResourceLimit{
				TokenBudget:          65537,
				ConcurrentExecutions: 1,
			},
			"token budget exceeds limit",
		},
		{
			ResourceLimit{
				TimeoutSeconds:       300,
				TokenBudget:          4096,
				ConcurrentExecutions: 0,
			},
			"zero concurrent executions",
		},
		{
			ResourceLimit{
				TimeoutSeconds:       300,
				TokenBudget:          4096,
				ConcurrentExecutions: 101,
			},
			"concurrent executions exceed max",
		},
		{
			ResourceLimit{
				TimeoutSeconds:       300,
				TokenBudget:          4096,
				ConcurrentExecutions: 1,
				MaxRetries:           -1,
			},
			"negative max retries",
		},
		{
			ResourceLimit{
				TimeoutSeconds:       300,
				TokenBudget:          4096,
				ConcurrentExecutions: 1,
				MaxRetries:           6,
			},
			"max retries exceed recommended",
		},
	}

	for _, test := range invalidLimits {
		if err := ValidateResourceLimits(test.limits, maxTokens); err == nil {
			t.Errorf("ValidateResourceLimits() should fail for %s", test.desc)
		}
	}
}

// TestValidateRoleContextValid tests valid context definitions.
func TestValidateRoleContextValid(t *testing.T) {
	validContexts := []RoleContext{
		{
			Files:                []string{"CLAUDE.md", ".eslintrc"},
			DataSources:          []string{"git:history"},
			EnvironmentVariables: []string{"API_KEY"},
		},
		{
			Files: []string{},
		},
		{
			Files:       []string{strings.Repeat("a", 10)},
			DataSources: []string{},
		},
	}

	contextLimit := 50
	for _, ctx := range validContexts {
		if err := ValidateRoleContext(ctx, contextLimit); err != nil {
			t.Errorf("ValidateRoleContext() returned error: %v", err)
		}
	}
}

// TestValidateRoleContextInvalid tests invalid context definitions.
func TestValidateRoleContextInvalid(t *testing.T) {
	contextLimit := 50

	invalidContexts := []struct {
		ctx  RoleContext
		desc string
	}{
		{
			RoleContext{
				Files: make([]string, 51),
			},
			"exceeds file limit",
		},
		{
			RoleContext{
				Files: []string{"file@name"},
			},
			"invalid file pattern",
		},
	}

	for _, test := range invalidContexts {
		if err := ValidateRoleContext(test.ctx, contextLimit); err == nil {
			t.Errorf("ValidateRoleContext() should fail for %s", test.desc)
		}
	}
}

// TestValidateRoleWithSchema tests full role validation against schema.
func TestValidateRoleWithSchema(t *testing.T) {
	validator := DefaultRoleSchemaValidator()

	validRole := &RoleTemplate{
		Name:        "test-role",
		Description: "This is a test role for validation purposes",
		SystemPrompt: "You are a test role. Follow all instructions and report back to the coordinator. " +
			strings.Repeat("a", 50),
		Model:     "claude-sonnet-4",
		MaxTokens: 4096,
		Permissions: Permissions{
			CanRead:    []string{"**/*"},
			CanWrite:   []string{"test/*"},
			CanExecute: []string{"signal:send"},
		},
	}

	if err := ValidateRoleWithSchema(validRole, validator); err != nil {
		t.Errorf("ValidateRoleWithSchema() failed for valid role: %v", err)
	}
}

// TestValidateRoleWithSchemaNilRole tests validation with nil role.
func TestValidateRoleWithSchemaNilRole(t *testing.T) {
	validator := DefaultRoleSchemaValidator()
	if err := ValidateRoleWithSchema(nil, validator); err == nil {
		t.Error("ValidateRoleWithSchema() should fail for nil role")
	}
}

// TestValidateRoleWithSchemaReservedName tests that reserved names are rejected.
func TestValidateRoleWithSchemaReservedName(t *testing.T) {
	validator := DefaultRoleSchemaValidator()

	role := &RoleTemplate{
		Name:        "coordinator",  // reserved
		Description: "This is a test role",
		SystemPrompt: "You are a test role. " + strings.Repeat("a", 50),
		Model:       "claude-sonnet-4",
		MaxTokens:   4096,
	}

	if err := ValidateRoleWithSchema(role, validator); err == nil {
		t.Error("ValidateRoleWithSchema() should reject reserved role name")
	}
}

// TestValidateRoleWithSchemaInvalidMaxTokens tests token validation.
func TestValidateRoleWithSchemaInvalidMaxTokens(t *testing.T) {
	validator := DefaultRoleSchemaValidator()

	tests := []struct {
		maxTokens int
		shouldErr bool
	}{
		{0, true},           // zero
		{-1, true},          // negative
		{1, false},          // minimum valid
		{65536, false},      // at limit
		{65537, true},       // over limit
	}

	for _, test := range tests {
		role := &RoleTemplate{
			Name:        "test",
			Description: "This is a test role",
			SystemPrompt: "You are a test role. " + strings.Repeat("a", 50),
			MaxTokens:   test.maxTokens,
		}

		err := ValidateRoleWithSchema(role, validator)
		if (err != nil) != test.shouldErr {
			t.Errorf("ValidateRoleWithSchema() with MaxTokens=%d: expected error=%v, got error=%v",
				test.maxTokens, test.shouldErr, err != nil)
		}
	}
}

// TestGetSchemaDefinition tests schema definition retrieval.
func TestGetSchemaDefinition(t *testing.T) {
	schema := GetSchemaDefinition()

	if schema.Version != RoleSchemaVersion {
		t.Errorf("schema version mismatch: expected %s, got %s", RoleSchemaVersion, schema.Version)
	}

	if len(schema.RequiredFields) == 0 {
		t.Error("schema should have required fields")
	}

	if len(schema.Fields) == 0 {
		t.Error("schema should define fields")
	}

	// Check that name, description, system_prompt, and max_tokens are defined
	requiredFields := map[string]bool{
		"name":             false,
		"description":      false,
		"system_prompt":    false,
		"max_tokens":       false,
	}

	for field := range schema.Fields {
		if _, exists := requiredFields[field]; exists {
			requiredFields[field] = true
		}
	}

	for field, found := range requiredFields {
		if !found {
			t.Errorf("required field %q not found in schema", field)
		}
	}
}

// TestDefaultRoleSchemaValidator tests default validator creation.
func TestDefaultRoleSchemaValidator(t *testing.T) {
	validator := DefaultRoleSchemaValidator()

	if validator == nil {
		t.Fatal("DefaultRoleSchemaValidator() returned nil")
	}

	if len(validator.AllowedModels) == 0 {
		t.Error("default validator should have allowed models")
	}

	if validator.MaxTokensLimit <= 0 {
		t.Error("default validator should have positive max tokens limit")
	}

	if len(validator.ReservedRoleNames) == 0 {
		t.Error("default validator should have reserved role names")
	}
}

// TestExampleSecurityAuditorRole tests the security auditor example role.
func TestExampleSecurityAuditorRole(t *testing.T) {
	role := ExampleSecurityAuditorRole()

	if role.Name != "security-auditor" {
		t.Errorf("expected name 'security-auditor', got %q", role.Name)
	}

	if role.Description == "" {
		t.Error("role should have description")
	}

	if role.SystemPrompt == "" {
		t.Error("role should have system prompt")
	}

	if role.MaxTokens <= 0 {
		t.Error("role should have positive max tokens")
	}

	// Verify it passes validation
	validator := DefaultRoleSchemaValidator()
	if err := ValidateRoleWithSchema(role, validator); err != nil {
		t.Errorf("example role should pass validation: %v", err)
	}
}

// TestExampleDataAnalystRole tests the data analyst example role.
func TestExampleDataAnalystRole(t *testing.T) {
	role := ExampleDataAnalystRole()

	if role.Name != "data-analyst" {
		t.Errorf("expected name 'data-analyst', got %q", role.Name)
	}

	validator := DefaultRoleSchemaValidator()
	if err := ValidateRoleWithSchema(role, validator); err != nil {
		t.Errorf("example role should pass validation: %v", err)
	}
}

// TestExamplePerformanceOptimizerRole tests the performance optimizer example role.
func TestExamplePerformanceOptimizerRole(t *testing.T) {
	role := ExamplePerformanceOptimizerRole()

	if role.Name != "performance-optimizer" {
		t.Errorf("expected name 'performance-optimizer', got %q", role.Name)
	}

	validator := DefaultRoleSchemaValidator()
	if err := ValidateRoleWithSchema(role, validator); err != nil {
		t.Errorf("example role should pass validation: %v", err)
	}
}

// TestSchemaVersionConsistency tests that schema version is consistent.
func TestSchemaVersionConsistency(t *testing.T) {
	schema := GetSchemaDefinition()
	if schema.Version == "" {
		t.Error("schema version should not be empty")
	}

	if RoleSchemaVersion == "" {
		t.Error("RoleSchemaVersion constant should not be empty")
	}

	if schema.Version != RoleSchemaVersion {
		t.Errorf("schema version %q should match constant %q", schema.Version, RoleSchemaVersion)
	}
}

// TestPermissionLevelConstants tests permission level enum values.
func TestPermissionLevelConstants(t *testing.T) {
	strictLevel := PermissionLevelStrict
	globLevel := PermissionLevelGlob

	if strictLevel != "strict" {
		t.Errorf("PermissionLevelStrict should be 'strict', got %q", strictLevel)
	}

	if globLevel != "glob" {
		t.Errorf("PermissionLevelGlob should be 'glob', got %q", globLevel)
	}
}

// TestRoleNameLengthBoundaries tests role name length boundaries.
func TestRoleNameLengthBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		shouldErr bool
	}{
		{"a", false},                                       // minimum
		{strings.Repeat("a", 128), false},                 // maximum
		{strings.Repeat("a", 129), true},                  // over maximum
		{"test-role-that-is-very-long-but-still-valid", false},
	}

	for _, test := range tests {
		err := ValidateRoleNameFormat(test.name)
		if (err != nil) != test.shouldErr {
			t.Errorf("ValidateRoleNameFormat(%q): expected error=%v, got error=%v",
				test.name, test.shouldErr, err != nil)
		}
	}
}

// TestSystemPromptLengthBoundaries tests system prompt length boundaries.
func TestSystemPromptLengthBoundaries(t *testing.T) {
	minLen := 50
	maxLen := 8192

	tests := []struct {
		prompt    string
		shouldErr bool
	}{
		{strings.Repeat("a", 49), true},   // below minimum
		{strings.Repeat("a", 50), false},  // at minimum
		{strings.Repeat("a", 8192), false}, // at maximum
		{strings.Repeat("a", 8193), true},  // over maximum
	}

	for _, test := range tests {
		err := ValidateSystemPrompt(test.prompt, minLen, maxLen)
		if (err != nil) != test.shouldErr {
			t.Errorf("ValidateSystemPrompt(): expected error=%v, got error=%v", test.shouldErr, err != nil)
		}
	}
}
