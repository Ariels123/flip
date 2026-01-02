package spawn

import (
	"fmt"
	"path/filepath"
)

// PermissionError is returned when an action is denied due to insufficient permissions.
type PermissionError struct {
	AgentID    string
	ActionType string
	Target     string
	Message    string
}

// Error implements the error interface for PermissionError.
func (pe *PermissionError) Error() string {
	return fmt.Sprintf("permission denied: agent %s cannot %s %s (%s)",
		pe.AgentID, pe.ActionType, pe.Target, pe.Message)
}

// PermissionChecker validates whether an agent with specific permissions
// is allowed to perform actions on resources.
type PermissionChecker struct {
	agentID     string
	permissions *Permissions
}

// NewPermissionChecker creates a new PermissionChecker for an agent.
//
// Parameters:
//   - agentID: The ID of the agent whose permissions to check
//   - permissions: The permissions object from the agent's role
//
// Returns:
//   - A new PermissionChecker instance
func NewPermissionChecker(agentID string, permissions *Permissions) *PermissionChecker {
	return &PermissionChecker{
		agentID:     agentID,
		permissions: permissions,
	}
}

// CheckReadPermission determines if the agent can read a file or resource.
//
// Parameters:
//   - filePath: The path to the file/resource to read
//
// Returns:
//   - error: PermissionError if read is denied, nil if allowed
func (pc *PermissionChecker) CheckReadPermission(filePath string) error {
	if pc.permissions == nil || len(pc.permissions.CanRead) == 0 {
		return &PermissionError{
			AgentID:    pc.agentID,
			ActionType: "read",
			Target:     filePath,
			Message:    "no read permissions defined",
		}
	}

	if pc.matchesPatterns(filePath, pc.permissions.CanRead) {
		return nil
	}

	return &PermissionError{
		AgentID:    pc.agentID,
		ActionType: "read",
		Target:     filePath,
		Message:    "path does not match any allowed patterns",
	}
}

// CheckWritePermission determines if the agent can write to a file or resource.
//
// Parameters:
//   - filePath: The path to the file/resource to write to
//
// Returns:
//   - error: PermissionError if write is denied, nil if allowed
func (pc *PermissionChecker) CheckWritePermission(filePath string) error {
	if pc.permissions == nil || len(pc.permissions.CanWrite) == 0 {
		return &PermissionError{
			AgentID:    pc.agentID,
			ActionType: "write",
			Target:     filePath,
			Message:    "no write permissions defined",
		}
	}

	if pc.matchesPatterns(filePath, pc.permissions.CanWrite) {
		return nil
	}

	return &PermissionError{
		AgentID:    pc.agentID,
		ActionType: "write",
		Target:     filePath,
		Message:    "path does not match any allowed patterns",
	}
}

// CheckExecutePermission determines if the agent can execute a command or operation.
//
// Parameters:
//   - command: The command or operation to execute (e.g., "spawn:worker", "task:report")
//
// Returns:
//   - error: PermissionError if execution is denied, nil if allowed
func (pc *PermissionChecker) CheckExecutePermission(command string) error {
	if pc.permissions == nil || len(pc.permissions.CanExecute) == 0 {
		return &PermissionError{
			AgentID:    pc.agentID,
			ActionType: "execute",
			Target:     command,
			Message:    "no execute permissions defined",
		}
	}

	if pc.matchesPatterns(command, pc.permissions.CanExecute) {
		return nil
	}

	return &PermissionError{
		AgentID:    pc.agentID,
		ActionType: "execute",
		Target:     command,
		Message:    "command does not match any allowed patterns",
	}
}

// matchesPatterns checks if a target matches any of the glob patterns.
//
// Parameters:
//   - target: The path or command to check
//   - patterns: List of glob patterns to match against
//
// Returns:
//   - bool: true if target matches any pattern, false otherwise
func (pc *PermissionChecker) matchesPatterns(target string, patterns []string) bool {
	for _, pattern := range patterns {
		// filepath.Match is used for file path patterns (e.g., "logs/*", "**/*.go")
		// For command patterns like "spawn:worker", we do exact matching on the pattern

		// Special case: ** at the start matches everything recursively
		if pattern == "**/*" || pattern == "**" {
			return true
		}

		// Try to match as a filepath pattern
		matched, err := filepath.Match(pattern, target)
		if err == nil && matched {
			return true
		}

		// For patterns with "**", convert to a simpler match
		// "**/*.go" -> "*.go" pattern, or "**/*" -> matches all
		if len(pattern) > 2 && pattern[:2] == "**" {
			// For recursive patterns, we do a simple suffix match
			// "**/*.go" matches any file ending in .go
			if len(pattern) > 3 && pattern[2:3] == "/" {
				suffix := pattern[3:] // Remove "**/" prefix
				matched, err := filepath.Match(suffix, filepath.Base(target))
				if err == nil && matched {
					return true
				}
			}
		}

		// For exact matching (useful for commands like "spawn:worker")
		if pattern == target {
			return true
		}
	}

	return false
}

// GetPermissions returns the underlying permissions object for inspection.
//
// Returns:
//   - The Permissions object associated with this checker
func (pc *PermissionChecker) GetPermissions() *Permissions {
	return pc.permissions
}

// CanPerformAction is a convenience method to check all three permission types.
// This is useful for generic permission checks before performing an operation.
//
// Parameters:
//   - actionType: "read", "write", or "execute"
//   - target: The resource or command to check
//
// Returns:
//   - error: PermissionError if the action is denied, nil if allowed
func (pc *PermissionChecker) CanPerformAction(actionType, target string) error {
	switch actionType {
	case "read":
		return pc.CheckReadPermission(target)
	case "write":
		return pc.CheckWritePermission(target)
	case "execute":
		return pc.CheckExecutePermission(target)
	default:
		return &PermissionError{
			AgentID:    pc.agentID,
			ActionType: actionType,
			Target:     target,
			Message:    "unknown action type",
		}
	}
}
