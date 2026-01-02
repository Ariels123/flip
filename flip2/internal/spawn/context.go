// Package spawn provides role-based worker instantiation and management.
package spawn

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"flip2/internal/config"
)

// ProjectContext represents loaded FLIP2.md configuration and auto-referenced context files.
//
// This struct provides the public API for working with project context that is
// automatically loaded on agent spawn.
type ProjectContext struct {
	// ConfigPath is the absolute path to the FLIP2.md file that was loaded
	ConfigPath string

	// Config is the parsed FLIP2.md configuration
	Config *config.ProjectConfig

	// FLIP2MDContent is the raw content of the FLIP2.md file
	FLIP2MDContent string

	// LoadedFiles contains metadata about each successfully loaded context file
	LoadedFiles []LoadedFile

	// TotalContextSize is the total size in bytes of all loaded content
	TotalContextSize int
}

// LoadedFile represents a single context file that was loaded as part of project context.
type LoadedFile struct {
	// Path is the file path as specified in FLIP2.md
	Path string

	// Weight is the importance weight: "high", "medium", or "low"
	Weight string

	// Size is the size in bytes of the loaded content (may be truncated)
	Size int

	// Truncated indicates if the file was truncated to fit within the 100KB context limit
	Truncated bool

	// Content is the actual file content
	Content string
}

// LoadProjectContext loads FLIP2.md configuration and auto-referenced context files
// from the specified working directory.
//
// This function implements CFG-006's core requirement to auto-load FLIP2.md on spawn.
// It:
//
// 1. Searches for FLIP2.md in workingDir or its parent directories (up to 5 levels)
// 2. Parses the FLIP2.md configuration file
// 3. Extracts the Context.AutoLoadFiles section
// 4. Reads and loads all referenced context files in weight order (high, medium, low)
// 5. Returns a structured ProjectContext ready for injection into agent prompts
// 6. Enforces a 100KB limit on total context size
//
// If FLIP2.md is not found, this function returns an empty ProjectContext gracefully
// without error, allowing workers to spawn even in projects without FLIP2.md.
//
// Parameters:
//   - workingDir: The directory to start searching from (uses current working directory if empty)
//
// Returns:
//   - context: Structured ProjectContext containing config and loaded files
//   - error: Any error that occurred during loading or parsing
//
// Example:
//
//	ctx, err := LoadProjectContext("./")
//	if err != nil {
//		log.Fatal(err)
//	}
//	enhanced := InjectContextOnSpawn(role.SystemPrompt, ctx)
func LoadProjectContext(workingDir string) (*ProjectContext, error) {
	if workingDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		workingDir = cwd
	}

	ctx := &ProjectContext{
		LoadedFiles: []LoadedFile{},
	}

	// Find FLIP2.md starting from workingDir
	flip2Path := findFLIP2MDFrom(workingDir)
	if flip2Path == "" {
		// FLIP2.md is optional; return empty context gracefully
		return ctx, nil
	}

	ctx.ConfigPath = flip2Path

	// Read FLIP2.md content
	flip2Content, err := os.ReadFile(flip2Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read FLIP2.md at %s: %w", flip2Path, err)
	}

	ctx.FLIP2MDContent = string(flip2Content)
	ctx.TotalContextSize = len(flip2Content)

	// Parse FLIP2.md configuration
	projectConfig, err := config.ParseFLIP2MD(flip2Path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse FLIP2.md: %w", err)
	}

	ctx.Config = projectConfig

	// Load auto-referenced context files in order of weight
	const maxContextSize = 100 * 1024 // 100KB limit
	baseDir := filepath.Dir(flip2Path)

	// Process files in weight order (high priority first)
	for _, weight := range []string{"high", "medium", "low"} {
		for _, ctxFile := range projectConfig.Context.AutoLoadFiles {
			if ctxFile.Weight != weight {
				continue
			}

			// Check if we've exceeded the context size limit
			if ctx.TotalContextSize >= maxContextSize {
				break
			}

			// Resolve file path relative to FLIP2.md location
			filePath := filepath.Join(baseDir, ctxFile.Path)

			// Try to read the file
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				// Skip files that can't be read but don't fail overall
				continue
			}

			// Check if adding this file would exceed the limit
			newSize := ctx.TotalContextSize + len(fileContent)
			if newSize > maxContextSize {
				// Try to fit a truncated version
				remainingSpace := maxContextSize - ctx.TotalContextSize
				if remainingSpace > 100 { // Only include if there's meaningful space
					truncatedContent := fileContent[:remainingSpace-50]
					ctx.LoadedFiles = append(ctx.LoadedFiles, LoadedFile{
						Path:      ctxFile.Path,
						Weight:    weight,
						Size:      len(truncatedContent),
						Truncated: true,
						Content:   string(truncatedContent),
					})
					ctx.TotalContextSize = maxContextSize
				}
				break
			}

			// File fits completely
			ctx.LoadedFiles = append(ctx.LoadedFiles, LoadedFile{
				Path:      ctxFile.Path,
				Weight:    weight,
				Size:      len(fileContent),
				Truncated: false,
				Content:   string(fileContent),
			})
			ctx.TotalContextSize = newSize
		}
	}

	return ctx, nil
}

// InjectContextOnSpawn injects loaded project context into a role's system prompt.
//
// This function is the counterpart to LoadProjectContext. It takes a role's system
// prompt and a loaded ProjectContext, then produces an enhanced prompt that includes
// the project context in a clearly marked section.
//
// The enhanced prompt format is:
//
//	[Original System Prompt]
//
//	===== PROJECT CONTEXT =====
//
//	## FLIP2.md Configuration
//	[Config content]
//
//	## Project Files
//	[Loaded context files with their contents]
//
// This allows agents spawned in the FLIP system to automatically understand:
// - Project structure and configuration
// - Important reference files and documentation
// - Role-specific context and requirements
// - Any other files marked for auto-loading
//
// Parameters:
//   - systemPrompt: The original role's system prompt
//   - projectContext: The loaded project context (from LoadProjectContext)
//
// Returns:
//   - enhancedPrompt: The system prompt with project context injected, or the
//     original prompt if no context is available
//
// Example:
//
//	role := GetBuiltinRole("worker")
//	ctx, _ := LoadProjectContext("./")
//	enhanced := InjectContextOnSpawn(role.SystemPrompt, ctx)
//	// enhanced now contains both the role prompt and project context
func InjectContextOnSpawn(systemPrompt string, projectContext *ProjectContext) string {
	if projectContext == nil || projectContext.ConfigPath == "" {
		// No context to inject; return original prompt
		return systemPrompt
	}

	// Build context string from loaded content
	contextParts := []string{}

	// Add FLIP2.md configuration first
	if projectContext.FLIP2MDContent != "" {
		contextParts = append(contextParts,
			"## FLIP2.md Configuration\n```markdown\n"+projectContext.FLIP2MDContent+"\n```")
	}

	// Add loaded files in weight order
	if len(projectContext.LoadedFiles) > 0 {
		contextParts = append(contextParts, "## Project Context Files")

		for _, lf := range projectContext.LoadedFiles {
			header := fmt.Sprintf("### %s", lf.Path)
			if lf.Weight != "" {
				header += fmt.Sprintf(" (weight: %s)", lf.Weight)
			}
			if lf.Truncated {
				header += " [truncated to fit]"
			}

			if lf.Content != "" {
				contextParts = append(contextParts, header+"\n```\n"+lf.Content+"\n```")
			}
		}
	}

	if len(contextParts) == 0 {
		// No context content to add
		return systemPrompt
	}

	// Format final context section
	contextStr := "===== PROJECT CONTEXT =====\n\n" + strings.Join(contextParts, "\n\n")

	// Combine with original prompt
	if systemPrompt == "" {
		return contextStr
	}

	return systemPrompt + "\n\n" + contextStr
}

// findFLIP2MDFrom searches for FLIP2.md starting from startDir and walking up
// the directory tree up to 5 levels. Returns the first FLIP2.md path found,
// or empty string if none is found.
func findFLIP2MDFrom(startDir string) string {
	cwd := startDir

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
