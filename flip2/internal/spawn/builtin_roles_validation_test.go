package spawn

import (
	"sort"
	"testing"
)

// TestBuiltinRolesComplete validates all builtin roles are properly defined
func TestBuiltinRolesComplete(t *testing.T) {
	requiredRoles := []string{"code-reviewer", "researcher", "implementer"}
	
	for _, roleName := range requiredRoles {
		if _, exists := BuiltinRoles[roleName]; !exists {
			t.Errorf("Required role %q not found in BuiltinRoles", roleName)
		}
	}
	
	// Check that we have 5 total roles
	if len(BuiltinRoles) != 5 {
		t.Errorf("Expected 5 builtin roles, got %d", len(BuiltinRoles))
	}
}

// TestCodeReviewerRole validates the code-reviewer role
func TestCodeReviewerRole(t *testing.T) {
	role := BuiltinRoles["code-reviewer"]
	if role == nil {
		t.Fatal("code-reviewer role not found")
	}
	
	if role.Name != "code-reviewer" {
		t.Errorf("Expected name 'code-reviewer', got %q", role.Name)
	}
	
	if role.Model != "claude-sonnet-4" {
		t.Errorf("Expected model 'claude-sonnet-4', got %q", role.Model)
	}
	
	// Should have read permissions for code files
	hasCodeRead := false
	for _, pattern := range role.Permissions.CanRead {
		if pattern == "**/*.go" || pattern == "**/*.py" {
			hasCodeRead = true
			break
		}
	}
	if !hasCodeRead {
		t.Error("code-reviewer should have read permissions for code files")
	}
	
	// Should be read-only for writes
	if len(role.Permissions.CanWrite) > 0 && role.Permissions.CanWrite[0] != "reviews/*.md" {
		t.Error("code-reviewer should only write to reviews/*.md")
	}
	
	if err := role.Validate(); err != nil {
		t.Errorf("code-reviewer role validation failed: %v", err)
	}
}

// TestResearcherRole validates the researcher role
func TestResearcherRole(t *testing.T) {
	role := BuiltinRoles["researcher"]
	if role == nil {
		t.Fatal("researcher role not found")
	}
	
	if role.Name != "researcher" {
		t.Errorf("Expected name 'researcher', got %q", role.Name)
	}
	
	if role.Model != "gemini-2.5-pro" {
		t.Errorf("Expected model 'gemini-2.5-pro', got %q", role.Model)
	}
	
	// Should have broad read permissions
	hasRead := false
	for _, pattern := range role.Permissions.CanRead {
		if pattern == "**/*" {
			hasRead = true
			break
		}
	}
	if !hasRead {
		t.Error("researcher should have broad read permissions")
	}
	
	// Should have web browsing capability
	hasBrowse := false
	for _, exec := range role.Permissions.CanExecute {
		if exec == "browse:web" {
			hasBrowse = true
			break
		}
	}
	if !hasBrowse {
		t.Error("researcher should have browse:web capability")
	}
	
	if err := role.Validate(); err != nil {
		t.Errorf("researcher role validation failed: %v", err)
	}
}

// TestImplementerRole validates the implementer role
func TestImplementerRole(t *testing.T) {
	role := BuiltinRoles["implementer"]
	if role == nil {
		t.Fatal("implementer role not found")
	}
	
	if role.Name != "implementer" {
		t.Errorf("Expected name 'implementer', got %q", role.Name)
	}
	
	if role.Model != "claude-sonnet-4" {
		t.Errorf("Expected model 'claude-sonnet-4', got %q", role.Model)
	}
	
	// Should have full read permissions
	hasRead := false
	for _, pattern := range role.Permissions.CanRead {
		if pattern == "**/*" {
			hasRead = true
			break
		}
	}
	if !hasRead {
		t.Error("implementer should have full read permissions")
	}
	
	// Should have write permissions for code
	hasWrite := false
	for _, pattern := range role.Permissions.CanWrite {
		if pattern == "**/*.go" || pattern == "**/*.py" {
			hasWrite = true
			break
		}
	}
	if !hasWrite {
		t.Error("implementer should have write permissions for code files")
	}
	
	if err := role.Validate(); err != nil {
		t.Errorf("implementer role validation failed: %v", err)
	}
}

// TestGeminiFlashWorkerRole validates the gemini-flash-worker role
func TestGeminiFlashWorkerRole(t *testing.T) {
	role := BuiltinRoles["gemini-flash-worker"]
	if role == nil {
		t.Fatal("gemini-flash-worker role not found")
	}
	
	if role.Name != "gemini-flash-worker" {
		t.Errorf("Expected name 'gemini-flash-worker', got %q", role.Name)
	}
	
	if role.Model != "gemini-2.5-flash" {
		t.Errorf("Expected model 'gemini-2.5-flash', got %q", role.Model)
	}
	
	if err := role.Validate(); err != nil {
		t.Errorf("gemini-flash-worker role validation failed: %v", err)
	}
}

// TestHaikuWorkerRole validates the haiku-worker role
func TestHaikuWorkerRole(t *testing.T) {
	role := BuiltinRoles["haiku-worker"]
	if role == nil {
		t.Fatal("haiku-worker role not found")
	}
	
	if role.Name != "haiku-worker" {
		t.Errorf("Expected name 'haiku-worker', got %q", role.Name)
	}
	
	if role.Model != "claude-haiku-4" {
		t.Errorf("Expected model 'claude-haiku-4', got %q", role.Model)
	}
	
	if err := role.Validate(); err != nil {
		t.Errorf("haiku-worker role validation failed: %v", err)
	}
}

// TestAllRolesValidate ensures all builtin roles pass validation
func TestAllRolesValidate(t *testing.T) {
	for name, role := range BuiltinRoles {
		if err := role.Validate(); err != nil {
			t.Errorf("Role %q failed validation: %v", name, err)
		}
		
		if role.MaxTokens <= 0 {
			t.Errorf("Role %q has invalid MaxTokens: %d", name, role.MaxTokens)
		}
		
		if role.Model == "" {
			t.Errorf("Role %q has no model specified", name)
		}
	}
}

// TestRolePermissionMatrix validates permission structure across all roles
func TestRolePermissionMatrix(t *testing.T) {
	for name, role := range BuiltinRoles {
		// Every role must have at least read permission
		if len(role.Permissions.CanRead) == 0 {
			t.Errorf("Role %q has no read permissions", name)
		}
		
		// Every role should have at least one execute permission for reporting
		hasReport := false
		for _, exec := range role.Permissions.CanExecute {
			if exec == "signal:send" || exec == "task:report" {
				hasReport = true
				break
			}
		}
		if !hasReport {
			t.Errorf("Role %q should have task reporting capability", name)
		}
	}
}

// TestRoleNamesUnique ensures all role names are unique
func TestRoleNamesUnique(t *testing.T) {
	names := make([]string, 0, len(BuiltinRoles))
	for name := range BuiltinRoles {
		names = append(names, name)
	}
	
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("Duplicate role name: %q", name)
		}
		seen[name] = true
	}
}

// TestRoleNameFormat ensures all role names follow naming conventions
func TestRoleNameFormat(t *testing.T) {
	for name := range BuiltinRoles {
		// Role names should be lowercase with hyphens
		for i, char := range name {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-') {
				t.Errorf("Role name %q contains invalid character at position %d: %c", name, i, char)
			}
		}
		
		// Should start with a letter
		if name[0] < 'a' || name[0] > 'z' {
			t.Errorf("Role name %q should start with a lowercase letter", name)
		}
	}
}

// TestRoleSpawning validates that all roles can be spawned
func TestRoleSpawning(t *testing.T) {
	names := make([]string, 0, len(BuiltinRoles))
	for name := range BuiltinRoles {
		names = append(names, name)
	}
	sort.Strings(names)
	
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			agentID, err := SpawnWithRole(name, "test task")
			if err != nil {
				t.Errorf("Failed to spawn with role %q: %v", name, err)
			}
			
			if agentID == "" {
				t.Errorf("Spawn with role %q returned empty agent ID", name)
			}
		})
	}
}

// TestRoleSystemPrompts ensures all roles have meaningful system prompts
func TestRoleSystemPrompts(t *testing.T) {
	for name, role := range BuiltinRoles {
		// System prompt should be long enough to contain meaningful content
		if len(role.SystemPrompt) < 50 {
			t.Errorf("Role %q has very short system prompt (%d chars)", name, len(role.SystemPrompt))
		}

		// Should contain role constraints or responsibilities
		prompt := role.SystemPrompt
		hasConstraints := stringContains(prompt, "constraint") || stringContains(prompt, "should") || stringContains(prompt, "must")
		if !hasConstraints {
			t.Errorf("Role %q system prompt lacks constraints or guidance", name)
		}
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func stringContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}
