package spawn

import (
	"strings"
	"testing"
)

// TestSpawnWithRole tests the basic role-based spawning functionality
func TestSpawnWithRole(t *testing.T) {
	tests := []struct {
		name      string
		roleName  string
		task      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "Valid code-reviewer role",
			roleName: "code-reviewer",
			task:     "Review the authentication module for security issues",
			wantErr:  false,
		},
		{
			name:     "Valid researcher role",
			roleName: "researcher",
			task:     "Research current best practices for Go error handling",
			wantErr:  false,
		},
		{
			name:     "Valid implementer role",
			roleName: "implementer",
			task:     "Implement the database migration system",
			wantErr:  false,
		},
		{
			name:      "Invalid role name",
			roleName:  "nonexistent-role",
			task:      "Some task",
			wantErr:   true,
			errSubstr: "not found",
		},
		{
			name:      "Empty task",
			roleName:  "code-reviewer",
			task:      "",
			wantErr:   true,
			errSubstr: "task",
		},
		{
			name:      "Empty role name",
			roleName:  "",
			task:      "Some task",
			wantErr:   true,
			errSubstr: "role name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentID, err := SpawnWithRole(tt.roleName, tt.task)

			if (err != nil) != tt.wantErr {
				t.Errorf("SpawnWithRole() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("SpawnWithRole() error = %v, want substring %q", err, tt.errSubstr)
				}
			}

			if !tt.wantErr && agentID == "" {
				t.Errorf("SpawnWithRole() returned empty agent ID")
			}

			// Verify agent ID format
			if !tt.wantErr && agentID != "" {
				if !strings.Contains(agentID, "-") {
					t.Errorf("SpawnWithRole() agent ID %q doesn't contain expected format", agentID)
				}
			}
		})
	}
}

// TestGenerateAgentID tests the agent ID generation
func TestGenerateAgentID(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "Valid prefix",
			prefix:  "code-reviewer",
			wantErr: false,
		},
		{
			name:    "Valid prefix with underscores",
			prefix:  "code_reviewer",
			wantErr: false,
		},
		{
			name:      "Invalid prefix with spaces",
			prefix:    "code reviewer",
			wantErr:   true,
			errSubstr: "invalid",
		},
		{
			name:      "Invalid prefix with special chars",
			prefix:    "code@reviewer",
			wantErr:   true,
			errSubstr: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentID, err := generateAgentID(tt.prefix)

			if (err != nil) != tt.wantErr {
				t.Errorf("generateAgentID() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("generateAgentID() error = %v, want substring %q", err, tt.errSubstr)
				}
			}

			if !tt.wantErr && agentID == "" {
				t.Errorf("generateAgentID() returned empty ID")
			}

			// Verify format: prefix-hexstring
			if !tt.wantErr && agentID != "" {
				parts := strings.Split(agentID, "-")
				if len(parts) < 2 {
					t.Errorf("generateAgentID() returned ID without dash separator: %q", agentID)
				}
			}
		})
	}
}

// TestGetSpawnInfoForRole tests retrieving spawn information
func TestGetSpawnInfoForRole(t *testing.T) {
	tests := []struct {
		name      string
		roleName  string
		task      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "Valid role info request",
			roleName: "code-reviewer",
			task:    "Review code",
			wantErr: false,
		},
		{
			name:      "Invalid role",
			roleName:  "unknown-role",
			task:      "Some task",
			wantErr:   true,
			errSubstr: "not found",
		},
		{
			name:      "Empty task",
			roleName:  "researcher",
			task:      "",
			wantErr:   true,
			errSubstr: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := GetSpawnInfoForRole(tt.roleName, tt.task)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpawnInfoForRole() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if info == nil {
					t.Errorf("GetSpawnInfoForRole() returned nil info")
				}

				if info.AgentID == "" {
					t.Errorf("GetSpawnInfoForRole() returned empty agent ID")
				}

				if info.RoleName != tt.roleName {
					t.Errorf("GetSpawnInfoForRole() RoleName = %q, want %q", info.RoleName, tt.roleName)
				}

				if info.Task != tt.task {
					t.Errorf("GetSpawnInfoForRole() Task = %q, want %q", info.Task, tt.task)
				}

				if info.Model == "" {
					t.Errorf("GetSpawnInfoForRole() returned empty model")
				}

				if info.Status != "spawning" {
					t.Errorf("GetSpawnInfoForRole() Status = %q, want %q", info.Status, "spawning")
				}
			}
		})
	}
}

// TestBuiltinRolesExist tests that built-in roles are properly defined
func TestBuiltinRolesExist(t *testing.T) {
	requiredRoles := []string{"code-reviewer", "researcher", "implementer"}

	for _, roleName := range requiredRoles {
		t.Run("Role "+roleName, func(t *testing.T) {
			role := GetBuiltinRole(roleName)
			if role == nil {
				t.Errorf("GetBuiltinRole(%q) returned nil", roleName)
			}

			if role != nil {
				if err := role.Validate(); err != nil {
					t.Errorf("Built-in role %q failed validation: %v", roleName, err)
				}

				if role.Model == "" {
					t.Errorf("Role %q has no model specified", roleName)
				}

				if role.MaxTokens <= 0 {
					t.Errorf("Role %q has invalid max_tokens: %d", roleName, role.MaxTokens)
				}
			}
		})
	}
}
