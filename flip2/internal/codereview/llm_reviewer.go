package codereview

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// LLMReviewer uses an LLM backend to perform code reviews
type LLMReviewer struct {
	name     string
	backend  string // "gemini", "claude", etc.
	repoPath string
	invokeFunc func(ctx context.Context, backend, prompt string) (string, error)
}

// NewLLMReviewer creates a new LLM-based reviewer
func NewLLMReviewer(name, backend, repoPath string, invokeFunc func(ctx context.Context, backend, prompt string) (string, error)) *LLMReviewer {
	return &LLMReviewer{
		name:       name,
		backend:    backend,
		repoPath:   repoPath,
		invokeFunc: invokeFunc,
	}
}

// Name returns the reviewer name
func (r *LLMReviewer) Name() string {
	return r.name
}

// ReviewCode performs a code review using the LLM
func (r *LLMReviewer) ReviewCode(ctx context.Context, scope ReviewScope, target string) ([]Issue, string, error) {
	// Get the code to review based on scope
	codeContent, files, err := r.getCodeToReview(ctx, scope, target)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get code: %w", err)
	}

	if codeContent == "" {
		return []Issue{}, "No changes to review", nil
	}

	// Build the review prompt
	prompt := r.buildReviewPrompt(scope, files, codeContent)

	// Invoke LLM
	response, err := r.invokeFunc(ctx, r.backend, prompt)
	if err != nil {
		return nil, "", fmt.Errorf("LLM invocation failed: %w", err)
	}

	// Parse the response into issues
	issues, summary := r.parseReviewResponse(response, files)

	return issues, summary, nil
}

// getCodeToReview retrieves the code content based on scope
func (r *LLMReviewer) getCodeToReview(ctx context.Context, scope ReviewScope, target string) (string, []string, error) {
	var cmd *exec.Cmd
	var files []string

	switch scope {
	case ScopeUncommitted:
		// Get uncommitted changes
		cmd = exec.CommandContext(ctx, "git", "diff", "HEAD")
		cmd.Dir = r.repoPath

	case ScopeRecentChanges:
		// Get recent commits (last 24 hours)
		cmd = exec.CommandContext(ctx, "git", "log", "-p", "--since=24.hours")
		cmd.Dir = r.repoPath

	case ScopeFile:
		if target == "" {
			return "", nil, fmt.Errorf("target file required for file scope")
		}
		// Get specific file content
		cmd = exec.CommandContext(ctx, "cat", target)
		cmd.Dir = r.repoPath
		files = []string{target}

	default:
		return "", nil, fmt.Errorf("unsupported scope: %s", scope)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get code: %w", err)
	}

	content := string(output)

	// Extract files from diff if not already set
	if len(files) == 0 && strings.Contains(content, "diff --git") {
		files = r.extractFilesFromDiff(content)
	}

	return content, files, nil
}

// extractFilesFromDiff extracts file paths from git diff output
func (r *LLMReviewer) extractFilesFromDiff(diff string) []string {
	files := make(map[string]bool)
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// Extract b/path/to/file
				filePath := strings.TrimPrefix(parts[3], "b/")
				files[filePath] = true
			}
		}
	}

	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	return result
}

// buildReviewPrompt creates the LLM prompt for code review
func (r *LLMReviewer) buildReviewPrompt(scope ReviewScope, files []string, codeContent string) string {
	var prompt strings.Builder

	prompt.WriteString("You are an expert code reviewer. Review the following code changes and identify issues.\n\n")
	prompt.WriteString("Focus on:\n")
	prompt.WriteString("- Critical bugs and logic errors\n")
	prompt.WriteString("- Security vulnerabilities\n")
	prompt.WriteString("- Performance issues\n")
	prompt.WriteString("- Code quality and maintainability\n")
	prompt.WriteString("- Best practices violations\n\n")

	prompt.WriteString(fmt.Sprintf("Scope: %s\n", scope))
	if len(files) > 0 {
		prompt.WriteString(fmt.Sprintf("Files: %s\n\n", strings.Join(files, ", ")))
	}

	prompt.WriteString("Code to review:\n```\n")
	// Limit code content to avoid token limits
	if len(codeContent) > 50000 {
		prompt.WriteString(codeContent[:50000])
		prompt.WriteString("\n... (truncated)")
	} else {
		prompt.WriteString(codeContent)
	}
	prompt.WriteString("\n```\n\n")

	prompt.WriteString("Provide your review in the following format:\n")
	prompt.WriteString("SUMMARY: <brief overall assessment>\n\n")
	prompt.WriteString("ISSUES:\n")
	prompt.WriteString("For each issue, use this format:\n")
	prompt.WriteString("- FILE: <file_path>\n")
	prompt.WriteString("  LINE: <line_number> (if applicable)\n")
	prompt.WriteString("  SEVERITY: critical|warning|info\n")
	prompt.WriteString("  CATEGORY: bug|security|performance|style\n")
	prompt.WriteString("  DESCRIPTION: <what's wrong>\n")
	prompt.WriteString("  SUGGESTION: <how to fix>\n\n")

	return prompt.String()
}

// parseReviewResponse parses the LLM response into structured issues
func (r *LLMReviewer) parseReviewResponse(response string, files []string) ([]Issue, string) {
	issues := []Issue{}
	var summary string

	lines := strings.Split(response, "\n")
	var currentIssue *Issue

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Extract summary
		if strings.HasPrefix(line, "SUMMARY:") {
			summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
			continue
		}

		// Parse issue fields
		if strings.HasPrefix(line, "- FILE:") {
			if currentIssue != nil {
				issues = append(issues, *currentIssue)
			}
			currentIssue = &Issue{
				File: strings.TrimSpace(strings.TrimPrefix(line, "- FILE:")),
			}
		} else if currentIssue != nil {
			if strings.HasPrefix(line, "LINE:") {
				lineStr := strings.TrimSpace(strings.TrimPrefix(line, "LINE:"))
				var lineNum int
				fmt.Sscanf(lineStr, "%d", &lineNum)
				currentIssue.Line = lineNum
			} else if strings.HasPrefix(line, "SEVERITY:") {
				currentIssue.Severity = strings.TrimSpace(strings.TrimPrefix(line, "SEVERITY:"))
			} else if strings.HasPrefix(line, "CATEGORY:") {
				currentIssue.Category = strings.TrimSpace(strings.TrimPrefix(line, "CATEGORY:"))
			} else if strings.HasPrefix(line, "DESCRIPTION:") {
				currentIssue.Description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
			} else if strings.HasPrefix(line, "SUGGESTION:") {
				currentIssue.Suggestion = strings.TrimSpace(strings.TrimPrefix(line, "SUGGESTION:"))
			}
		}
	}

	// Add last issue
	if currentIssue != nil {
		issues = append(issues, *currentIssue)
	}

	// If no summary was extracted, create a default one
	if summary == "" {
		if len(issues) == 0 {
			summary = "Code review completed. No significant issues found."
		} else {
			summary = fmt.Sprintf("Code review completed. Found %d issue(s) requiring attention.", len(issues))
		}
	}

	return issues, summary
}
