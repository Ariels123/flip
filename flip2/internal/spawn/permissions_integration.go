package spawn

import (
	"fmt"
	"os"
)

// FileReadWrapper wraps file read operations with permission checks.
// This shows how to integrate the PermissionChecker with actual file operations.
//
// Example usage:
//
//	checker := NewPermissionChecker("worker-1", permissions)
//	content, err := FileReadWrapper(checker, "config/app.json")
//	if err != nil {
//	    log.Fatal(err) // Could be permission denied or file not found
//	}
func FileReadWrapper(checker *PermissionChecker, filePath string) ([]byte, error) {
	// First check permissions
	if err := checker.CheckReadPermission(filePath); err != nil {
		return nil, err
	}

	// If permission check passes, attempt to read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

// FileWriteWrapper wraps file write operations with permission checks.
// This shows how to integrate the PermissionChecker with actual file operations.
//
// Example usage:
//
//	checker := NewPermissionChecker("worker-2", permissions)
//	err := FileWriteWrapper(checker, "output/results.txt", []byte("data"))
//	if err != nil {
//	    log.Fatal(err) // Could be permission denied or write failed
//	}
func FileWriteWrapper(checker *PermissionChecker, filePath string, content []byte) error {
	// First check permissions
	if err := checker.CheckWritePermission(filePath); err != nil {
		return err
	}

	// If permission check passes, attempt to write the file
	err := os.WriteFile(filePath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// CommandExecutionWrapper wraps command execution with permission checks.
// This is a pattern example showing how to integrate permission checks.
//
// In a real system, this would be used with actual command execution logic.
//
// Example usage:
//
//	checker := NewPermissionChecker("worker-3", permissions)
//	err := CommandExecutionWrapper(checker, "spawn:worker")
//	if err != nil {
//	    log.Fatal(err) // Permission denied or execution failed
//	}
func CommandExecutionWrapper(checker *PermissionChecker, command string) error {
	// First check permissions
	if err := checker.CheckExecutePermission(command); err != nil {
		return err
	}

	// If permission check passes, would execute the command
	// In a real implementation, this would parse and execute the command
	// For example: handleCommand(command)

	return nil
}

// AgentPermissionGate provides a convenient interface for checking all agent operations.
// This wraps the PermissionChecker and provides methods that agents would call.
type AgentPermissionGate struct {
	checker *PermissionChecker
	logger  func(string, ...interface{})
}

// NewAgentPermissionGate creates a new permission gate for an agent.
//
// Parameters:
//   - agentID: The ID of the agent
//   - permissions: The permissions from the agent's role
//   - logger: Optional logging function for permission checks
func NewAgentPermissionGate(agentID string, permissions *Permissions, logger func(string, ...interface{})) *AgentPermissionGate {
	if logger == nil {
		logger = func(string, ...interface{}) {} // No-op logger
	}

	return &AgentPermissionGate{
		checker: NewPermissionChecker(agentID, permissions),
		logger:  logger,
	}
}

// ReadFile attempts to read a file with permission enforcement.
func (gate *AgentPermissionGate) ReadFile(filePath string) ([]byte, error) {
	gate.logger("Attempting to read file: %s by agent %s", filePath, gate.checker.agentID)

	content, err := FileReadWrapper(gate.checker, filePath)
	if err != nil {
		if _, ok := err.(*PermissionError); ok {
			gate.logger("Permission denied: %v", err)
		}
		return nil, err
	}

	gate.logger("Successfully read file: %s", filePath)
	return content, nil
}

// WriteFile attempts to write a file with permission enforcement.
func (gate *AgentPermissionGate) WriteFile(filePath string, content []byte) error {
	gate.logger("Attempting to write file: %s by agent %s", filePath, gate.checker.agentID)

	err := FileWriteWrapper(gate.checker, filePath, content)
	if err != nil {
		if _, ok := err.(*PermissionError); ok {
			gate.logger("Permission denied: %v", err)
		}
		return err
	}

	gate.logger("Successfully wrote file: %s", filePath)
	return nil
}

// ExecuteCommand attempts to execute a command with permission enforcement.
func (gate *AgentPermissionGate) ExecuteCommand(command string) error {
	gate.logger("Attempting to execute command: %s by agent %s", command, gate.checker.agentID)

	err := CommandExecutionWrapper(gate.checker, command)
	if err != nil {
		if _, ok := err.(*PermissionError); ok {
			gate.logger("Permission denied: %v", err)
		}
		return err
	}

	gate.logger("Successfully executed command: %s", command)
	return nil
}

// GetAgentID returns the ID of the agent this gate protects.
func (gate *AgentPermissionGate) GetAgentID() string {
	return gate.checker.agentID
}

// GetPermissions returns the underlying permissions object.
func (gate *AgentPermissionGate) GetPermissions() *Permissions {
	return gate.checker.GetPermissions()
}

// PermissionReport generates a human-readable report of what an agent can and cannot do.
type PermissionReport struct {
	AgentID    string
	CanRead    []string
	CanWrite   []string
	CanExecute []string
}

// GeneratePermissionReport creates a report of an agent's capabilities.
//
// This is useful for debugging and understanding what permissions an agent has.
func GeneratePermissionReport(agentID string, permissions *Permissions) *PermissionReport {
	return &PermissionReport{
		AgentID:    agentID,
		CanRead:    permissions.CanRead,
		CanWrite:   permissions.CanWrite,
		CanExecute: permissions.CanExecute,
	}
}

// String returns a formatted string representation of the permission report.
func (pr *PermissionReport) String() string {
	return fmt.Sprintf(`Permission Report for Agent: %s
Can Read: %v
Can Write: %v
Can Execute: %v`,
		pr.AgentID,
		pr.CanRead,
		pr.CanWrite,
		pr.CanExecute,
	)
}
