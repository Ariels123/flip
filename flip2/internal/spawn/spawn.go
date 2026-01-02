// Package spawn provides role-based worker instantiation and management.
package spawn

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"flip2/internal/config"
)

var (
	// ErrRoleNotFound is returned when a requested role does not exist
	ErrRoleNotFound = fmt.Errorf("role not found")

	// ErrInvalidAgentID is returned when an agent ID fails validation
	ErrInvalidAgentID = fmt.Errorf("invalid agent ID format")

	// ErrTaskDescriptionRequired is returned when task description is empty
	ErrTaskDescriptionRequired = fmt.Errorf("task description is required")

	// validAgentID matches alphanumeric characters, hyphens, and underscores
	// Agent IDs must be safe for use in file systems and APIs
	validAgentID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// SpawnConfig holds configuration for spawning a worker agent with a role
type SpawnConfig struct {
	// RoleName is the name of the role to spawn with (e.g., "code-reviewer", "researcher")
	RoleName string

	// Task is the task description or instructions for the worker
	Task string

	// AgentIDPrefix is an optional prefix for the generated agent ID
	// If provided, the generated ID will be "{AgentIDPrefix}-{random}"
	// If empty, the generated ID will be "{RoleName}-{random}"
	AgentIDPrefix string

	// Logger is optional; if nil, a no-op logger will be used
	Logger *slog.Logger

	// UseBuiltinRole determines if we should look for the role in built-in roles
	// If false, only custom roles are searched
	UseBuiltinRole bool
}

// SpawnWithRole spawns a new worker agent with a specified role.
//
// The function:
// 1. Loads the role template from BuiltinRoles or custom storage
// 2. Validates the role
// 3. Generates a unique agent ID
// 4. Sets up the agent with the role's system prompt and permissions
// 5. Selects the model specified in the role template
// 6. Returns the agent ID for tracking
//
// Parameters:
//   - roleName: Name of the role to use (e.g., "code-reviewer")
//   - task: Description of the task the agent should complete
//
// Returns:
//   - agentID: Unique identifier for the spawned agent (e.g., "code-reviewer-abc123def456")
//   - error: Any error that occurred during spawning
func SpawnWithRole(roleName, task string) (agentID string, err error) {
	cfg := SpawnConfig{
		RoleName:       roleName,
		Task:           task,
		UseBuiltinRole: true,
		Logger:         nil, // Use no-op logger if not provided
	}
	return SpawnWithRoleConfig(cfg)
}

// SpawnWithRoleConfig spawns a new worker agent with a specified role and custom configuration.
//
// This is the extended version of SpawnWithRole that allows customization through SpawnConfig.
//
// Enhancement (SPW-005): Injects project context from FLIP2.md and auto-loaded files
// into the spawned agent's system prompt. The context includes:
// - The FLIP2.md configuration itself
// - All files specified in the Context.AutoLoadFiles section
// - Limited to 100KB to maintain reasonable context size
//
// Parameters:
//   - cfg: SpawnConfig with detailed spawn parameters
//
// Returns:
//   - agentID: Unique identifier for the spawned agent
//   - error: Any error that occurred during spawning
func SpawnWithRoleConfig(cfg SpawnConfig) (agentID string, err error) {
	// Get no-op logger if not provided
	if cfg.Logger == nil {
		cfg.Logger = slog.New(&noopHandler{})
	}

	// Validate inputs
	if cfg.RoleName == "" {
		return "", fmt.Errorf("role name is required")
	}
	if cfg.Task == "" {
		return "", ErrTaskDescriptionRequired
	}

	// Load role template
	var role *RoleTemplate
	if cfg.UseBuiltinRole {
		role = GetBuiltinRole(cfg.RoleName)
	}

	if role == nil {
		cfg.Logger.Debug("role not found", "role_name", cfg.RoleName)
		return "", fmt.Errorf("role '%s': %w", cfg.RoleName, ErrRoleNotFound)
	}

	// Validate the role
	if err := role.Validate(); err != nil {
		cfg.Logger.Debug("invalid role", "role_name", cfg.RoleName, "error", err)
		return "", fmt.Errorf("invalid role '%s': %w", cfg.RoleName, err)
	}

	// Generate agent ID
	idPrefix := cfg.AgentIDPrefix
	if idPrefix == "" {
		idPrefix = cfg.RoleName
	}

	agentID, err = generateAgentID(idPrefix)
	if err != nil {
		cfg.Logger.Debug("failed to generate agent ID", "prefix", idPrefix, "error", err)
		return "", fmt.Errorf("failed to generate agent ID: %w", err)
	}

	// Enhance system prompt with project context (SPW-005)
	enhancedSystemPrompt := role.SystemPrompt
	contextInjection := loadAndInjectProjectContext(cfg.Logger)
	if contextInjection != "" {
		enhancedSystemPrompt = enhancedSystemPrompt + "\n\n" + contextInjection
	}

	// Update the role's system prompt with the enhanced version
	role.SystemPrompt = enhancedSystemPrompt

	// Log spawn attempt
	cfg.Logger.Info("spawning worker agent",
		"agent_id", agentID,
		"role", cfg.RoleName,
		"model", role.Model,
		"max_tokens", role.MaxTokens,
	)

	// The agent initialization would typically happen here in a real system.
	// In the context of FLIP2, this would interact with the database to create
	// an agent record with the role configuration.
	//
	// For now, we return the agent ID. The CLI command will create the actual
	// agent record in the PocketBase database.
	//
	// In a full implementation, this function would:
	// 1. Create an agent record in the database with:
	//    - agent_id: generated ID
	//    - system_prompt: role.SystemPrompt (now enhanced with project context)
	//    - permissions: role.Permissions (JSON)
	//    - model: role.Model
	//    - max_tokens: role.MaxTokens
	//    - task: cfg.Task
	//    - status: "spawning" (will be updated when agent connects)
	// 2. Write configuration to a file
	// 3. Return any error from database operations

	cfg.Logger.Info("worker agent spawned successfully",
		"agent_id", agentID,
		"role", cfg.RoleName,
	)

	return agentID, nil
}

// generateAgentID creates a unique agent ID with a prefix and random suffix.
//
// Format: {prefix}-{random_hex}
// Example: "code-reviewer-a1b2c3d4e5f6"
//
// Parameters:
//   - prefix: The prefix for the agent ID (typically the role name)
//
// Returns:
//   - agentID: Unique agent ID
//   - error: Error if ID generation or validation fails
func generateAgentID(prefix string) (string, error) {
	// Validate prefix
	if !validAgentID.MatchString(prefix) {
		return "", fmt.Errorf("prefix '%s' contains invalid characters: %w", prefix, ErrInvalidAgentID)
	}

	// Generate 6 random bytes (12 hex characters)
	randomBytes := make([]byte, 6)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	randomSuffix := hex.EncodeToString(randomBytes)
	agentID := fmt.Sprintf("%s-%s", prefix, randomSuffix)

	// Validate the final ID
	if !validAgentID.MatchString(agentID) {
		return "", fmt.Errorf("generated agent ID '%s' is invalid: %w", agentID, ErrInvalidAgentID)
	}

	return agentID, nil
}

// SpawnInfo contains information about a spawned agent for tracking and debugging
type SpawnInfo struct {
	// AgentID is the unique identifier for the spawned agent
	AgentID string `json:"agent_id"`

	// RoleName is the name of the role the agent was spawned with
	RoleName string `json:"role_name"`

	// Model is the LLM model selected for this agent
	Model string `json:"model"`

	// SystemPrompt is the system context provided to the agent
	SystemPrompt string `json:"system_prompt"`

	// Permissions define what the agent is allowed to do
	Permissions Permissions `json:"permissions"`

	// Task is the task description assigned to the agent
	Task string `json:"task"`

	// SpawnedAt is the timestamp when the agent was spawned
	SpawnedAt time.Time `json:"spawned_at"`

	// Status is the current status of the spawned agent
	// Typical values: "spawning", "initializing", "ready", "error"
	Status string `json:"status"`
}

// GetSpawnInfoForRole returns a SpawnInfo object for a role with a generated agent ID.
//
// This is useful for displaying information about what will be spawned before actually
// creating the agent.
//
// Parameters:
//   - roleName: Name of the role
//   - task: Task description
//
// Returns:
//   - info: SpawnInfo with generated agent ID and role details
//   - error: Error if role is not found or validation fails
func GetSpawnInfoForRole(roleName, task string) (*SpawnInfo, error) {
	if roleName == "" {
		return nil, fmt.Errorf("role name is required")
	}
	if task == "" {
		return nil, ErrTaskDescriptionRequired
	}

	role := GetBuiltinRole(roleName)
	if role == nil {
		return nil, fmt.Errorf("role '%s': %w", roleName, ErrRoleNotFound)
	}

	if err := role.Validate(); err != nil {
		return nil, fmt.Errorf("invalid role '%s': %w", roleName, err)
	}

	agentID, err := generateAgentID(roleName)
	if err != nil {
		return nil, err
	}

	return &SpawnInfo{
		AgentID:      agentID,
		RoleName:     roleName,
		Model:        role.Model,
		SystemPrompt: role.SystemPrompt,
		Permissions:  role.Permissions,
		Task:         task,
		SpawnedAt:    time.Now(),
		Status:       "spawning",
	}, nil
}

// loadAndInjectProjectContext loads the FLIP2.md configuration and related context files,
// then formats them for injection into the agent's system prompt.
//
// SPW-005 Implementation:
// - Searches for FLIP2.md in the current working directory and parent directories
// - Parses the FLIP2.md to extract Context.AutoLoadFiles
// - Reads and includes the FLIP2.md file itself
// - Reads all auto-loaded files from config.Context section
// - Limits total context to 100KB to maintain reasonable prompt size
// - Returns formatted context string ready for injection into system prompt
func loadAndInjectProjectContext(logger *slog.Logger) string {
	const maxContextSize = 100 * 1024 // 100KB limit

	contextBuffer := &bytes.Buffer{}

	// Step 1: Find and load FLIP2.md
	flip2Path := findFLIP2MD()
	if flip2Path == "" {
		logger.Debug("no FLIP2.md found, context injection skipped")
		return ""
	}

	// Step 2: Read FLIP2.md content
	flip2Content, err := os.ReadFile(flip2Path)
	if err != nil {
		logger.Debug("failed to read FLIP2.md", "path", flip2Path, "error", err)
		return ""
	}

	// Step 3: Write context header
	contextBuffer.WriteString("# Project Context\n\n")
	contextBuffer.WriteString("## FLIP2.md Config\n")
	contextBuffer.WriteString("```\n")

	// Write FLIP2.md with size tracking
	flip2Bytes := int64(len(flip2Content))
	if contextBuffer.Len()+int(flip2Bytes) > maxContextSize {
		// Truncate FLIP2.md if needed to fit in context
		availableSpace := maxContextSize - contextBuffer.Len()
		truncated := flip2Content[:availableSpace-100] // -100 for safety margin
		contextBuffer.Write(truncated)
		contextBuffer.WriteString("\n... [truncated]\n")
		logger.Debug("FLIP2.md truncated to fit in context budget")
		return contextBuffer.String()
	}

	contextBuffer.Write(flip2Content)
	contextBuffer.WriteString("```\n\n")

	// Step 4: Parse FLIP2.md to get context files
	projectConfig, err := config.ParseFLIP2MD(flip2Path)
	if err != nil {
		logger.Debug("failed to parse FLIP2.md", "path", flip2Path, "error", err)
		// Continue without parsed context if parsing fails
		return contextBuffer.String()
	}

	// Step 5: Load and include relevant files
	if len(projectConfig.Context.AutoLoadFiles) > 0 {
		contextBuffer.WriteString("## Relevant Files\n\n")

		baseDir := filepath.Dir(flip2Path)

		for _, contextFile := range projectConfig.Context.AutoLoadFiles {
			// Check if we've exceeded the context size limit
			if contextBuffer.Len() >= maxContextSize {
				contextBuffer.WriteString("\n... [context size limit reached]\n")
				break
			}

			// Resolve file path relative to FLIP2.md location
			filePath := filepath.Join(baseDir, contextFile.Path)

			// Try to read the file
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				logger.Debug("failed to read context file",
					"path", contextFile.Path,
					"full_path", filePath,
					"error", err)
				continue
			}

			// Calculate remaining space
			remainingSpace := maxContextSize - contextBuffer.Len()
			fileSize := len(fileContent)

			// Ensure we have space for file header and some content
			if remainingSpace < 200 {
				contextBuffer.WriteString("\n... [context size limit reached]\n")
				break
			}

			// Write file header with description
			contextBuffer.WriteString(fmt.Sprintf("### %s\n", contextFile.Path))
			if contextFile.Description != "" {
				contextBuffer.WriteString(fmt.Sprintf("_Description: %s_\n", contextFile.Description))
			}
			if contextFile.Weight != "" {
				contextBuffer.WriteString(fmt.Sprintf("_Weight: %s_\n\n", contextFile.Weight))
			} else {
				contextBuffer.WriteString("\n")
			}

			// Write file content, truncating if necessary
			if fileSize > remainingSpace-200 {
				// File too large, truncate
				truncatedSize := remainingSpace - 300 // Leave margin for markers
				contextBuffer.WriteString("```\n")
				contextBuffer.Write(fileContent[:truncatedSize])
				contextBuffer.WriteString("\n... [truncated]\n")
				contextBuffer.WriteString("```\n\n")
				logger.Debug("context file truncated",
					"path", contextFile.Path,
					"original_size", fileSize,
					"truncated_size", truncatedSize)
				contextBuffer.WriteString("\n... [context size limit reached]\n")
				break
			} else {
				// File fits, include completely
				contextBuffer.WriteString("```\n")
				contextBuffer.Write(fileContent)
				contextBuffer.WriteString("\n```\n\n")
			}
		}
	}

	// Return formatted context
	result := contextBuffer.String()
	if len(result) > maxContextSize {
		// Final safety check - truncate if we somehow exceeded the limit
		result = result[:maxContextSize] + "\n\n... [context truncated to size limit]"
	}

	logger.Debug("project context loaded for injection",
		"context_size", len(result),
		"files_included", len(projectConfig.Context.AutoLoadFiles))

	return result
}

// findFLIP2MD searches for FLIP2.md starting from the current working directory
// and walking up the directory tree. Returns the first FLIP2.md path found,
// or empty string if none is found.
func findFLIP2MD() string {
	// Try current working directory first
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Check up to 5 directory levels
	for i := 0; i < 5; i++ {
		flip2Path := filepath.Join(cwd, "FLIP2.md")
		if _, err := os.Stat(flip2Path); err == nil {
			return flip2Path
		}

		// Move to parent directory
		parent := filepath.Dir(cwd)
		if parent == cwd {
			// Reached filesystem root
			break
		}
		cwd = parent
	}

	return ""
}

// noopHandler is a slog.Handler that discards all log records.
// Used when no logger is provided.
type noopHandler struct{}

// Enabled returns false since we discard all records
func (h *noopHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

// Handle is a no-op
func (h *noopHandler) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

// WithAttrs returns the same handler
func (h *noopHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

// WithGroup returns the same handler
func (h *noopHandler) WithGroup(_ string) slog.Handler {
	return h
}
