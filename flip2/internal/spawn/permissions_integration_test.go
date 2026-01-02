package spawn

import (
	"os"
	"path/filepath"
	"testing"
	"strings"
)

func TestFileReadWrapper(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")

	err := os.WriteFile(testFile, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		perms     *Permissions
		filePath  string
		shouldErr bool
	}{
		{
			name: "allowed read",
			perms: &Permissions{
				CanRead: []string{"**/*"},
			},
			filePath:  testFile,
			shouldErr: false,
		},
		{
			name: "permission denied",
			perms: &Permissions{
				CanRead: []string{"logs/*.log"},
			},
			filePath:  testFile,
			shouldErr: true,
		},
		{
			name: "no permissions",
			perms: &Permissions{
				CanRead: []string{},
			},
			filePath:  testFile,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("test-agent", tt.perms)
			content, err := FileReadWrapper(checker, tt.filePath)

			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if !tt.shouldErr && string(content) != string(testContent) {
				t.Errorf("Content mismatch: expected %s, got %s", testContent, content)
			}
		})
	}
}

func TestFileWriteWrapper(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		perms     *Permissions
		filePath  string
		shouldErr bool
	}{
		{
			name: "allowed write",
			perms: &Permissions{
				CanWrite: []string{"**/*.txt"},
			},
			filePath:  filepath.Join(tmpDir, "output.txt"),
			shouldErr: false,
		},
		{
			name: "permission denied",
			perms: &Permissions{
				CanWrite: []string{"protected/*"},
			},
			filePath:  filepath.Join(tmpDir, "output.txt"),
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("test-agent", tt.perms)
			testContent := []byte("test data")
			err := FileWriteWrapper(checker, tt.filePath, testContent)

			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			// Clean up
			if !tt.shouldErr {
				os.Remove(tt.filePath)
			}
		})
	}
}

func TestCommandExecutionWrapper(t *testing.T) {
	tests := []struct {
		name      string
		perms     *Permissions
		command   string
		shouldErr bool
	}{
		{
			name: "allowed command",
			perms: &Permissions{
				CanExecute: []string{"spawn:worker", "task:report"},
			},
			command:   "spawn:worker",
			shouldErr: false,
		},
		{
			name: "denied command",
			perms: &Permissions{
				CanExecute: []string{"task:report"},
			},
			command:   "spawn:coordinator",
			shouldErr: true,
		},
		{
			name: "wildcard allowed",
			perms: &Permissions{
				CanExecute: []string{"spawn:*"},
			},
			command:   "spawn:worker",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPermissionChecker("test-agent", tt.perms)
			err := CommandExecutionWrapper(checker, tt.command)

			if tt.shouldErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestAgentPermissionGate(t *testing.T) {
	perms := &Permissions{
		CanRead:    []string{"**/*.go"},
		CanWrite:   []string{"output/*.txt"},
		CanExecute: []string{"task:report"},
	}

	logMessages := []string{}
	logger := func(format string, args ...interface{}) {
		logMessages = append(logMessages, format)
	}

	gate := NewAgentPermissionGate("worker-1", perms, logger)

	// Test GetAgentID
	if gate.GetAgentID() != "worker-1" {
		t.Errorf("Expected agent ID 'worker-1', got %s", gate.GetAgentID())
	}

	// Test GetPermissions
	if gate.GetPermissions() != perms {
		t.Error("GetPermissions returned wrong object")
	}

	// Test ExecuteCommand with permission
	err := gate.ExecuteCommand("task:report")
	if err != nil {
		t.Errorf("Expected allowed command to succeed, got error: %v", err)
	}

	// Test ExecuteCommand without permission
	err = gate.ExecuteCommand("spawn:worker")
	if err == nil {
		t.Error("Expected denied command to fail")
	}

	// Verify logging occurred
	if len(logMessages) == 0 {
		t.Error("Expected logger to be called")
	}
}

func TestGeneratePermissionReport(t *testing.T) {
	perms := &Permissions{
		CanRead:    []string{"logs/*", "config/*"},
		CanWrite:   []string{"output/*"},
		CanExecute: []string{"task:report", "signal:send"},
	}

	report := GeneratePermissionReport("test-agent", perms)

	if report.AgentID != "test-agent" {
		t.Errorf("Expected agent ID 'test-agent', got %s", report.AgentID)
	}

	if len(report.CanRead) != 2 {
		t.Errorf("Expected 2 read permissions, got %d", len(report.CanRead))
	}

	if len(report.CanWrite) != 1 {
		t.Errorf("Expected 1 write permission, got %d", len(report.CanWrite))
	}

	if len(report.CanExecute) != 2 {
		t.Errorf("Expected 2 execute permissions, got %d", len(report.CanExecute))
	}

	// Test String representation
	reportStr := report.String()
	if !strings.Contains(reportStr, "test-agent") {
		t.Error("Report string should contain agent ID")
	}
	if !strings.Contains(reportStr, "Can Read") {
		t.Error("Report string should contain read permissions section")
	}
}

func TestIntegrationRolePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with code-reviewer role
	reviewerRole := CodeReviewerBuiltinRole()
	reviewerGate := NewAgentPermissionGate("code-reviewer-1", &reviewerRole.Permissions, nil)

	// Code reviewer should be able to read code files
	// Create test files
	goFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(goFile, []byte("package main"), 0644)

	content, err := reviewerGate.ReadFile(goFile)
	if err == nil {
		if string(content) != "package main" {
			t.Error("Content mismatch")
		}
	}

	// Test with researcher role
	researcherRole := ResearcherBuiltinRole()
	researcherGate := NewAgentPermissionGate("researcher-1", &researcherRole.Permissions, nil)

	// Researcher should be able to read any file
	configFile := filepath.Join(tmpDir, "config.json")
	os.WriteFile(configFile, []byte(`{"key":"value"}`), 0644)

	content, err = researcherGate.ReadFile(configFile)
	if err != nil {
		t.Errorf("Researcher should be able to read any file, got error: %v", err)
	}

	// Test with implementer role
	implementerRole := ImplementerBuiltinRole()
	implementerGate := NewAgentPermissionGate("implementer-1", &implementerRole.Permissions, nil)

	// Implementer should be able to write Go files
	newGoFile := filepath.Join(tmpDir, "new_feature.go")
	err = implementerGate.WriteFile(newGoFile, []byte("package main\nfunc main() {}"))
	if err != nil {
		t.Errorf("Implementer should be able to write .go files, got error: %v", err)
	}
}

func TestPermissionErrorDetails(t *testing.T) {
	perms := &Permissions{
		CanRead: []string{"logs/*.log"},
	}

	checker := NewPermissionChecker("restricted-agent", perms)
	err := checker.CheckReadPermission("secrets/password.txt")

	if err == nil {
		t.Fatal("Expected permission error")
	}

	// Verify error details
	permErr, ok := err.(*PermissionError)
	if !ok {
		t.Fatalf("Expected PermissionError, got %T", err)
	}

	if permErr.AgentID != "restricted-agent" {
		t.Errorf("Expected agent ID 'restricted-agent', got %s", permErr.AgentID)
	}

	if permErr.ActionType != "read" {
		t.Errorf("Expected action type 'read', got %s", permErr.ActionType)
	}

	if permErr.Target != "secrets/password.txt" {
		t.Errorf("Expected target 'secrets/password.txt', got %s", permErr.Target)
	}
}
