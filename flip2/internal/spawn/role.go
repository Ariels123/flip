// Package spawn provides role-based worker instantiation and management.
package spawn

import (
	"encoding/json"
	"fmt"

	"flip2/internal/config"
)

// Permissions defines what actions a role is allowed to perform.
type Permissions struct {
	// CanRead defines which resources or patterns this role can read.
	// Examples: []string{"logs/*", "config/public/*"}
	CanRead []string `json:"can_read,omitempty"`

	// CanWrite defines which resources or patterns this role can modify.
	// Examples: []string{"logs/worker/*", "state/*"}
	CanWrite []string `json:"can_write,omitempty"`

	// CanExecute defines which commands or operations this role can execute.
	// Examples: []string{"spawn:worker", "task:report", "signal:send"}
	CanExecute []string `json:"can_execute,omitempty"`
}

// RoleTemplate defines the configuration and capabilities for a spawnable agent role.
// Roles are used to create worker agents with consistent behavior and constraints.
type RoleTemplate struct {
	// Name is the unique identifier for this role.
	// Must be alphanumeric with hyphens (e.g., "research-worker", "code-reviewer").
	Name string `json:"name"`

	// Description explains the purpose and intended use of this role.
	// This helps coordinators understand what tasks should be assigned to agents with this role.
	Description string `json:"description"`

	// SystemPrompt is the initial system context provided to workers spawned with this role.
	// This establishes the worker's identity, constraints, and behavioral guidelines.
	// Should include: role description, reporting requirements, constraints, available tools.
	SystemPrompt string `json:"system_prompt"`

	// Permissions defines what this role is allowed to do.
	// Controls access to system operations, file system, and inter-agent communication.
	Permissions Permissions `json:"permissions"`

	// Model specifies the default LLM model for agents spawned with this role.
	// Examples: "claude-opus-4-5", "gemini-2.5-pro", "gpt-4"
	// If empty, the system uses its default model.
	Model string `json:"model"`

	// MaxTokens defines the maximum token limit for a single completion by this role.
	// This controls the maximum response size and helps manage costs.
	// Typical values: 2048, 4096, 8192, 16384
	MaxTokens int `json:"max_tokens"`
}

// MarshalJSON implements json.Marshaler for RoleTemplate.
func (rt *RoleTemplate) MarshalJSON() ([]byte, error) {
	type Alias RoleTemplate
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(rt),
	})
}

// UnmarshalJSON implements json.Unmarshaler for RoleTemplate.
func (rt *RoleTemplate) UnmarshalJSON(data []byte) error {
	type Alias RoleTemplate
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(rt),
	}
	return json.Unmarshal(data, &aux)
}

// Validate checks that the RoleTemplate has all required fields and valid values.
// Returns an error if the role definition is incomplete or invalid.
func (rt *RoleTemplate) Validate() error {
	if rt.Name == "" {
		return ErrRoleNameRequired
	}
	if rt.Description == "" {
		return ErrRoleDescriptionRequired
	}
	if rt.SystemPrompt == "" {
		return ErrRoleSystemPromptRequired
	}
	if rt.MaxTokens <= 0 {
		return ErrRoleInvalidMaxTokens
	}
	return nil
}

// LoadCustomRoles loads custom role definitions from a ProjectConfig and returns a map of custom roles.
// Custom roles are extracted from the AgentRole definitions in FLIP2.md configuration.
//
// Parameters:
//   - config: The ProjectConfig containing custom role definitions from FLIP2.md
//
// Returns:
//   - A map[string]*RoleTemplate of custom roles indexed by role name
//   - An error if parsing or validation fails
//
// The function converts AgentRole definitions from FLIP2.md into RoleTemplate objects.
// It validates each custom role to ensure all required fields are present.
func LoadCustomRoles(cfg *config.ProjectConfig) (map[string]*RoleTemplate, error) {
	customRoles := make(map[string]*RoleTemplate)

	if cfg == nil {
		return customRoles, nil
	}

	// Convert AgentRole definitions to RoleTemplate objects
	for _, agentRole := range cfg.Agents {
		role := &RoleTemplate{
			Name:        agentRole.Name,
			Description: agentRole.Description,
			Model:       agentRole.Model,
			MaxTokens:   4096, // Default max tokens if not specified
			// SystemPrompt will be generated from agent capabilities and description
			SystemPrompt: generateSystemPrompt(agentRole),
			// Permissions will be converted from agent capabilities and permissions
			Permissions: convertAgentPermissions(agentRole),
		}

		// Validate the role before adding
		if err := role.Validate(); err != nil {
			return nil, fmt.Errorf("invalid custom role %q: %w", agentRole.Name, ErrCustomRoleInvalid)
		}

		customRoles[role.Name] = role
	}

	return customRoles, nil
}

// MergeRoles combines builtin roles with custom roles, allowing custom roles to override builtins.
// Custom roles take precedence over builtin roles with the same name.
//
// Parameters:
//   - customRoles: Map of custom role definitions from LoadCustomRoles
//
// Returns:
//   - A merged map[string]*RoleTemplate containing both builtin and custom roles
//   - An error if a custom role has an invalid name that matches a builtin
//
// The merge strategy:
// 1. Start with all builtin roles
// 2. Add all custom roles, overriding any builtins with the same name
// 3. Return the merged set
//
// Note: Custom role names should not conflict with builtin roles unless intended as overrides.
func MergeRoles(customRoles map[string]*RoleTemplate) map[string]*RoleTemplate {
	// Start with a copy of builtin roles
	merged := make(map[string]*RoleTemplate)
	for name, role := range BuiltinRoles {
		roleCopy := *role
		merged[name] = &roleCopy
	}

	// Add or override with custom roles
	for name, role := range customRoles {
		roleCopy := *role
		merged[name] = &roleCopy
	}

	return merged
}

// generateSystemPrompt creates a system prompt from an AgentRole definition.
// This establishes the agent's identity, responsibilities, and constraints.
func generateSystemPrompt(agentRole config.AgentRole) string {
	if agentRole.Name == "" {
		return "You are a FLIP2 agent. Report findings to the coordinator."
	}

	prompt := fmt.Sprintf(`You are a %s agent in the FLIP2 system. Your coordinator manages task assignments and approvals.

**Role:** %s
**Description:** %s

**Responsibilities:**
- Complete assigned tasks according to specifications
- Report progress and findings to the coordinator
- Escalate blockers or ambiguities immediately
- Do not make autonomous decisions beyond your scope

**Capabilities:**
`, agentRole.Name, agentRole.Name, agentRole.Description)

	// Add capabilities
	if len(agentRole.Capabilities) > 0 {
		prompt += "You have the following capabilities:\n"
		for _, cap := range agentRole.Capabilities {
			prompt += fmt.Sprintf("- %s\n", cap)
		}
	}

	// Add escalation requirements
	if len(agentRole.EscalationRequired) > 0 {
		prompt += "\n**Actions Requiring Coordinator Approval:**\n"
		for _, esc := range agentRole.EscalationRequired {
			prompt += fmt.Sprintf("- %s\n", esc)
		}
	}

	prompt += `
**Important Constraints:**
- Do not spawn additional agents without explicit coordinator approval
- Do not make decisions about resource allocation or budgeting
- Do not execute operations marked as requiring escalation
- Signal the coordinator if you encounter blockers
- Report all results in structured format
- Do not make autonomous commits or deployments`

	return prompt
}

// convertAgentPermissions converts FLIP2.md AgentRole permissions to spawn.Permissions.
// This maps the string-based permission lists to pattern-based read/write/execute permissions.
func convertAgentPermissions(agentRole config.AgentRole) Permissions {
	permissions := Permissions{
		CanRead:    []string{"**/*"},
		CanWrite:   []string{},
		CanExecute: []string{},
	}

	// Map FLIP2.md permissions to spawn permissions
	for _, perm := range agentRole.Permissions {
		switch perm {
		case "read-inbox":
			// Already included in default CanRead
		case "send-signals":
			addIfNotPresent(&permissions.CanExecute, "signal:send")
		case "create-tasks":
			addIfNotPresent(&permissions.CanExecute, "task:create")
		case "modify-own-tasks":
			addIfNotPresent(&permissions.CanExecute, "task:modify-own")
		case "execute-code":
			addIfNotPresent(&permissions.CanWrite, "**/*.go")
			addIfNotPresent(&permissions.CanWrite, "**/*.py")
		case "approve-changes":
			addIfNotPresent(&permissions.CanExecute, "task:approve")
		}
	}

	// If CanWrite is still empty, restrict to safe paths
	if len(permissions.CanWrite) == 0 {
		permissions.CanWrite = []string{"work/*"}
	}

	return permissions
}

// addIfNotPresent adds a string to a slice if it's not already present.
func addIfNotPresent(slice *[]string, item string) {
	for _, existing := range *slice {
		if existing == item {
			return
		}
	}
	*slice = append(*slice, item)
}
