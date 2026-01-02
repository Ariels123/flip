package spawn

import (
	"errors"
	"testing"
)

func TestNewPermissionChecker(t *testing.T) {
	perms := &Permissions{
		CanRead:    []string{"**/*.go"},
		CanWrite:   []string{"output/*.txt"},
		CanExecute: []string{"spawn:worker"},
	}

	checker := NewPermissionChecker("test-agent", perms)

	if checker.agentID != "test-agent" {
		t.Errorf("Expected agent ID 'test-agent', got %s", checker.agentID)
	}

	if checker.permissions != perms {
		t.Error("Permissions not properly assigned")
	}
}

func TestCheckReadPermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      *Permissions
		filePath   string
		shouldPass bool
	}{
		{
			name: "exact match",
			perms: &Permissions{
				CanRead: []string{"config/app.json"},
			},
			filePath:   "config/app.json",
			shouldPass: true,
		},
		{
			name: "glob pattern single level",
			perms: &Permissions{
				CanRead: []string{"logs/*.log"},
			},
			filePath:   "logs/app.log",
			shouldPass: true,
		},
		{
			name: "glob pattern does not match",
			perms: &Permissions{
				CanRead: []string{"logs/*.log"},
			},
			filePath:   "logs/app.txt",
			shouldPass: false,
		},
		{
			name: "recursive glob pattern **/*.go",
			perms: &Permissions{
				CanRead: []string{"**/*.go"},
			},
			filePath:   "src/main.go",
			shouldPass: true,
		},
		{
			name: "recursive glob pattern **/*.go nested",
			perms: &Permissions{
				CanRead: []string{"**/*.go"},
			},
			filePath:   "pkg/internal/utils/helper.go",
			shouldPass: true,
		},
		{
			name: "full glob pattern **/*",
			perms: &Permissions{
				CanRead: []string{"**/*"},
			},
			filePath:   "any/path/to/file.txt",
			shouldPass: true,
		},
		{
			name: "no read permissions",
			perms: &Permissions{
				CanRead: []string{},
			},
			filePath:   "file.txt",
			shouldPass: false,
		},
		{
			name: "multiple patterns, matches second",
			perms: &Permissions{
				CanRead: []string{"logs/*.log", "config/*.json"},
			},
			filePath:   "config/app.json",
			shouldPass: true,
		},
		{
			name: "pattern with directory",
			perms: &Permissions{
				CanRead: []string{"public/**"},
			},
			filePath:   "public/assets/image.png",
			shouldPass: false, // public/** does not match due to filepath.Match semantics
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("test-agent", tt.perms)
			err := checker.CheckReadPermission(tt.filePath)

			if tt.shouldPass && err != nil {
				t.Errorf("Expected permission to pass, got error: %v", err)
			}

			if !tt.shouldPass && err == nil {
				t.Errorf("Expected permission to fail, but passed")
			}

			if err != nil {
				var permErr *PermissionError
				if !errors.As(err, &permErr) {
					t.Errorf("Expected PermissionError, got %T", err)
				}
			}
		})
	}
}

func TestCheckWritePermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      *Permissions
		filePath   string
		shouldPass bool
	}{
		{
			name: "write to allowed directory",
			perms: &Permissions{
				CanWrite: []string{"output/*.txt"},
			},
			filePath:   "output/report.txt",
			shouldPass: true,
		},
		{
			name: "write to disallowed directory",
			perms: &Permissions{
				CanWrite: []string{"output/*.txt"},
			},
			filePath:   "protected/secret.txt",
			shouldPass: false,
		},
		{
			name: "write with wildcard all",
			perms: &Permissions{
				CanWrite: []string{"**/*"},
			},
			filePath:   "any/file/path.go",
			shouldPass: true,
		},
		{
			name: "no write permissions defined",
			perms: &Permissions{
				CanWrite: []string{},
			},
			filePath:   "file.txt",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("worker-1", tt.perms)
			err := checker.CheckWritePermission(tt.filePath)

			if tt.shouldPass && err != nil {
				t.Errorf("Expected permission to pass, got error: %v", err)
			}

			if !tt.shouldPass && err == nil {
				t.Errorf("Expected permission to fail, but passed")
			}
		})
	}
}

func TestCheckExecutePermission(t *testing.T) {
	tests := []struct {
		name       string
		perms      *Permissions
		command    string
		shouldPass bool
	}{
		{
			name: "exact command match",
			perms: &Permissions{
				CanExecute: []string{"spawn:worker"},
			},
			command:    "spawn:worker",
			shouldPass: true,
		},
		{
			name: "command not in list",
			perms: &Permissions{
				CanExecute: []string{"spawn:worker"},
			},
			command:    "spawn:coordinator",
			shouldPass: false,
		},
		{
			name: "wildcard execute",
			perms: &Permissions{
				CanExecute: []string{"spawn:*"},
			},
			command:    "spawn:worker",
			shouldPass: true,
		},
		{
			name: "multiple commands, second matches",
			perms: &Permissions{
				CanExecute: []string{"signal:send", "task:report", "spawn:worker"},
			},
			command:    "task:report",
			shouldPass: true,
		},
		{
			name: "no execute permissions",
			perms: &Permissions{
				CanExecute: []string{},
			},
			command:    "spawn:worker",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("agent-x", tt.perms)
			err := checker.CheckExecutePermission(tt.command)

			if tt.shouldPass && err != nil {
				t.Errorf("Expected permission to pass, got error: %v", err)
			}

			if !tt.shouldPass && err == nil {
				t.Errorf("Expected permission to fail, but passed")
			}
		})
	}
}

func TestPermissionError(t *testing.T) {
	permErr := &PermissionError{
		AgentID:    "test-agent",
		ActionType: "read",
		Target:     "/etc/passwd",
		Message:    "path does not match any allowed patterns",
	}

	errorMsg := permErr.Error()
	if errorMsg == "" {
		t.Error("Error message should not be empty")
	}

	if !contains(errorMsg, "permission denied") {
		t.Errorf("Error message should contain 'permission denied', got: %s", errorMsg)
	}

	if !contains(errorMsg, "test-agent") {
		t.Errorf("Error message should contain agent ID, got: %s", errorMsg)
	}
}

func TestMatchesPatterns(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		patterns  []string
		shouldMatch bool
	}{
		{
			name:      "simple wildcard",
			target:    "app.log",
			patterns:  []string{"*.log"},
			shouldMatch: true,
		},
		{
			name:      "simple wildcard no match",
			target:    "app.txt",
			patterns:  []string{"*.log"},
			shouldMatch: false,
		},
		{
			name:      "double star recursive",
			target:    "src/pkg/utils.go",
			patterns:  []string{"**/*.go"},
			shouldMatch: true,
		},
		{
			name:      "double star all files",
			target:    "any/path/file.anything",
			patterns:  []string{"**/*"},
			shouldMatch: true,
		},
		{
			name:      "exact match",
			target:    "spawn:worker",
			patterns:  []string{"spawn:worker"},
			shouldMatch: true,
		},
		{
			name:      "command wildcard",
			target:    "spawn:worker",
			patterns:  []string{"spawn:*"},
			shouldMatch: true,
		},
		{
			name:      "multiple patterns first match",
			target:    "config.json",
			patterns:  []string{"*.json", "*.yaml"},
			shouldMatch: true,
		},
		{
			name:      "multiple patterns second match",
			target:    "app.yaml",
			patterns:  []string{"*.json", "*.yaml"},
			shouldMatch: true,
		},
		{
			name:      "multiple patterns no match",
			target:    "app.txt",
			patterns:  []string{"*.json", "*.yaml"},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("test", &Permissions{})
			matches := checker.matchesPatterns(tt.target, tt.patterns)

			if matches != tt.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v for target=%s with patterns=%v",
					tt.shouldMatch, matches, tt.target, tt.patterns)
			}
		})
	}
}

func TestCanPerformAction(t *testing.T) {
	perms := &Permissions{
		CanRead:    []string{"**/*.go"},
		CanWrite:   []string{"output/*.txt"},
		CanExecute: []string{"spawn:worker", "task:report"},
	}

	checker := NewPermissionChecker("test-agent", perms)

	tests := []struct {
		actionType string
		target     string
		shouldPass bool
	}{
		{"read", "main.go", true},
		{"read", "pkg/utils.go", true},
		{"read", "config.json", false},
		{"write", "output/report.txt", true},
		{"write", "protected/file.txt", false},
		{"execute", "spawn:worker", true},
		{"execute", "task:report", true},
		{"execute", "spawn:coordinator", false},
		{"invalid", "target", false},
	}

	for _, tt := range tests {
		t.Run(tt.actionType+"_"+tt.target, func(t *testing.T) {
			err := checker.CanPerformAction(tt.actionType, tt.target)

			if tt.shouldPass && err != nil {
				t.Errorf("Expected action to pass, got error: %v", err)
			}

			if !tt.shouldPass && err == nil {
				t.Errorf("Expected action to fail, but passed")
			}
		})
	}
}

func TestGetPermissions(t *testing.T) {
	perms := &Permissions{
		CanRead:    []string{"logs/*"},
		CanWrite:   []string{"output/*"},
		CanExecute: []string{"spawn:worker"},
	}

	checker := NewPermissionChecker("test-agent", perms)
	retrieved := checker.GetPermissions()

	if retrieved != perms {
		t.Error("GetPermissions should return the same permissions object")
	}
}

func TestCodeReviewerPermissions(t *testing.T) {
	role := CodeReviewerBuiltinRole()
	checker := NewPermissionChecker("code-reviewer-abc", &role.Permissions)

	// Should be able to read Go and Python files
	if err := checker.CheckReadPermission("main.go"); err != nil {
		t.Errorf("Code reviewer should read .go files: %v", err)
	}

	if err := checker.CheckReadPermission("script.py"); err != nil {
		t.Errorf("Code reviewer should read .py files: %v", err)
	}

	// Should not be able to read other files
	if err := checker.CheckReadPermission("config.json"); err == nil {
		t.Error("Code reviewer should not read .json files")
	}

	// Should be able to write reviews
	if err := checker.CheckWritePermission("reviews/pr-123.md"); err != nil {
		t.Errorf("Code reviewer should write to reviews: %v", err)
	}

	// Should be able to execute allowed operations
	if err := checker.CheckExecutePermission("signal:send"); err != nil {
		t.Errorf("Code reviewer should execute signal:send: %v", err)
	}
}

func TestResearcherPermissions(t *testing.T) {
	role := ResearcherBuiltinRole()
	checker := NewPermissionChecker("researcher-def", &role.Permissions)

	// Researcher can read anything
	if err := checker.CheckReadPermission("any/path/file.txt"); err != nil {
		t.Errorf("Researcher should read any file: %v", err)
	}

	// Researcher can write research files
	if err := checker.CheckWritePermission("research/findings.md"); err != nil {
		t.Errorf("Researcher should write to research directory: %v", err)
	}

	// Researcher can browse web
	if err := checker.CheckExecutePermission("browse:web"); err != nil {
		t.Errorf("Researcher should execute browse:web: %v", err)
	}
}

func TestImplementerPermissions(t *testing.T) {
	role := ImplementerBuiltinRole()
	checker := NewPermissionChecker("implementer-ghi", &role.Permissions)

	// Implementer can read any file
	if err := checker.CheckReadPermission("pkg/core/logic.go"); err != nil {
		t.Errorf("Implementer should read any file: %v", err)
	}

	// Implementer can write Go and Python files
	if err := checker.CheckWritePermission("pkg/new_module.go"); err != nil {
		t.Errorf("Implementer should write .go files: %v", err)
	}

	if err := checker.CheckWritePermission("scripts/helper.py"); err != nil {
		t.Errorf("Implementer should write .py files: %v", err)
	}

	// Implementer can report and signal
	if err := checker.CheckExecutePermission("task:report"); err != nil {
		t.Errorf("Implementer should execute task:report: %v", err)
	}
}

// Helper function for string containment
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
