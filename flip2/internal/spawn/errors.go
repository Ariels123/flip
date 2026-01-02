package spawn

import "errors"

// Validation errors for RoleTemplate
var (
	// ErrRoleNameRequired is returned when a role name is empty
	ErrRoleNameRequired = errors.New("role name is required")

	// ErrRoleDescriptionRequired is returned when a role description is empty
	ErrRoleDescriptionRequired = errors.New("role description is required")

	// ErrRoleSystemPromptRequired is returned when a role system prompt is empty
	ErrRoleSystemPromptRequired = errors.New("role system prompt is required")

	// ErrRoleInvalidMaxTokens is returned when MaxTokens is not positive
	ErrRoleInvalidMaxTokens = errors.New("role max_tokens must be greater than 0")

	// Permission-related errors
	// ErrPermissionDenied is returned when an operation is not allowed by permissions
	ErrPermissionDenied = errors.New("permission denied")

	// ErrNoReadPermission is returned when an agent cannot read a resource
	ErrNoReadPermission = errors.New("insufficient read permissions")

	// ErrNoWritePermission is returned when an agent cannot write to a resource
	ErrNoWritePermission = errors.New("insufficient write permissions")

	// ErrNoExecutePermission is returned when an agent cannot execute a command
	ErrNoExecutePermission = errors.New("insufficient execute permissions")

	// Custom role loading errors
	// ErrCustomRoleInvalid is returned when a custom role definition is invalid
	ErrCustomRoleInvalid = errors.New("invalid custom role definition")

	// ErrCustomRoleNameConflict is returned when custom role name conflicts with builtin
	ErrCustomRoleNameConflict = errors.New("custom role name conflicts with builtin role")

	// ErrCustomRoleParsingFailed is returned when custom roles cannot be parsed from config
	ErrCustomRoleParsingFailed = errors.New("failed to parse custom roles from config")
)
