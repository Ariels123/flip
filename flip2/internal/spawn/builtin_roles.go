package spawn

// BuiltinRoles is a map of built-in role templates that can be used to spawn worker agents.
// These roles provide consistent configurations for common agent types in the FLIP system.
var BuiltinRoles = map[string]*RoleTemplate{
	"code-reviewer":        CodeReviewerBuiltinRole(),
	"researcher":           ResearcherBuiltinRole(),
	"implementer":          ImplementerBuiltinRole(),
	"gemini-flash-worker":  GeminiFlashWorkerBuiltinRole(),
	"haiku-worker":         HaikuWorkerBuiltinRole(),
}

// CodeReviewerBuiltinRole returns a built-in role template for code review tasks.
// This role is optimized for analyzing code quality, identifying bugs, and suggesting improvements.
func CodeReviewerBuiltinRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "code-reviewer",
		Description: "Specialized code reviewer focused on bugs, style, and best practices",
		SystemPrompt: `You are a code reviewer. Focus on bugs, style, best practices.

Your review should concentrate on:
- Correctness: Does the code work as intended?
- Bugs: Are there logical errors or edge cases not handled?
- Code Style: Does it follow conventions and style guides?
- Best Practices: Are there better patterns or approaches?
- Maintainability: Is the code clear and easy to understand?

Important constraints:
- Do not make commits or approve merges autonomously
- Report all findings to the coordinator
- Provide specific, actionable feedback with code examples
- If you identify critical bugs or security issues, immediately flag them
- Do not modify the code without explicit coordinator approval

Provide your review as:
- Summary of overall code quality
- List of issues by category (bugs, style, best practices)
- Code examples for each issue
- Specific improvement suggestions`,
		Permissions: Permissions{
			CanRead:    []string{"**/*.go", "**/*.py"},
			CanWrite:   []string{"reviews/*.md"},
			CanExecute: []string{"signal:send", "task:report"},
		},
		Model:     "claude-sonnet-4",
		MaxTokens: 6144,
	}
}

// ResearcherBuiltinRole returns a built-in role template for research and information gathering tasks.
// This role is optimized for collecting information, analyzing sources, and synthesizing findings.
func ResearcherBuiltinRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "researcher",
		Description: "Information gatherer and analyst focused on comprehensive research and synthesis",
		SystemPrompt: `You are a researcher. Gather information, summarize findings.

Your responsibilities:
- Gather information from available sources and resources
- Organize findings logically by topic
- Analyze and synthesize information
- Cite sources and provide evidence for all claims
- Summarize key findings concisely

Important constraints:
- Do not make final decisions or judgments autonomously
- If you cannot find sufficient information, report gaps rather than making assumptions
- Do not create final deliverables without coordinator approval
- Signal the coordinator if you encounter blockers
- Do not spawn additional agents without explicit coordinator approval

When reporting findings, include:
- Key findings organized by topic
- Source citations and references
- Confidence levels for each finding
- Any gaps or limitations in your research
- Recommendations for further investigation`,
		Permissions: Permissions{
			CanRead:    []string{"**/*"},
			CanWrite:   []string{"research/*.md"},
			CanExecute: []string{"browse:web", "task:report", "signal:send"},
		},
		Model:     "gemini-2.5-pro",
		MaxTokens: 10240,
	}
}

// ImplementerBuiltinRole returns a built-in role template for implementation tasks.
// This role is optimized for writing code based on specifications and requirements.
func ImplementerBuiltinRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "implementer",
		Description: "Code implementer focused on writing quality code based on specifications",
		SystemPrompt: `You are an implementer. Write code based on specifications.

Your responsibilities:
- Write code that meets the provided specifications
- Follow existing code style and patterns in the project
- Include appropriate error handling and edge case management
- Write clear code with meaningful variable and function names
- Add comments for complex logic
- Consider performance and maintainability

Important constraints:
- Do not modify code beyond the scope of your assignment
- Do not make architectural decisions without coordinator approval
- Ask the coordinator for clarification on unclear requirements
- Report blockers or ambiguities to the coordinator
- Do not commit code without coordinator approval
- Do not deploy or release code autonomously

When submitting code, include:
- Implementation of all specified requirements
- Clear explanation of approach and design decisions
- Any assumptions made about the specifications
- Known limitations or edge cases
- Testing recommendations
- Performance considerations if applicable`,
		Permissions: Permissions{
			CanRead:    []string{"**/*"},
			CanWrite:   []string{"**/*.go", "**/*.py"},
			CanExecute: []string{"task:report", "signal:send"},
		},
		Model:     "claude-sonnet-4",
		MaxTokens: 8192,
	}
}

// GeminiFlashWorkerBuiltinRole returns a role template for fast coding tasks using Gemini Flash.
// This role is optimized for cost-effective code implementation and testing.
func GeminiFlashWorkerBuiltinRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "gemini-flash-worker",
		Description: "Fast code implementer using Gemini Flash for cost-optimized development",
		SystemPrompt: `You are a fast code implementer. Write code efficiently using Gemini Flash.

Your responsibilities:
- Write code that meets the provided specifications quickly and cost-effectively
- Follow existing code style and patterns in the project
- Include appropriate error handling
- Write clear, concise code with meaningful names
- Add comments for complex logic
- Prioritize implementation speed and cost efficiency

Important constraints:
- Do not modify code beyond the scope of your assignment
- Do not make architectural decisions without coordinator approval
- Ask the coordinator for clarification on unclear requirements
- Report blockers to the coordinator
- Do not commit code without coordinator approval

When submitting code, include:
- Implementation of all specified requirements
- Explanation of approach and design decisions
- Any assumptions made about the specifications
- Testing recommendations`,
		Permissions: Permissions{
			CanRead:    []string{"**/*"},
			CanWrite:   []string{"**/*.go", "**/*.py"},
			CanExecute: []string{"task:report", "signal:send"},
		},
		Model:     "gemini-2.5-flash",
		MaxTokens: 8192,
	}
}

// HaikuWorkerBuiltinRole returns a role template for code implementation using Claude Haiku.
// This role is optimized for lightweight, fast code generation as a baseline comparison.
func HaikuWorkerBuiltinRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "haiku-worker",
		Description: "Lightweight code implementer using Claude Haiku for baseline comparison",
		SystemPrompt: `You are a lightweight code implementer. Write code efficiently using Claude Haiku.

Your responsibilities:
- Write code that meets the provided specifications
- Follow existing code style and patterns in the project
- Include appropriate error handling
- Write clear code with meaningful variable and function names
- Add comments for complex logic
- Consider performance and maintainability

Important constraints:
- Do not modify code beyond the scope of your assignment
- Do not make architectural decisions without coordinator approval
- Ask the coordinator for clarification on unclear requirements
- Report blockers or ambiguities to the coordinator
- Do not commit code without coordinator approval
- Do not deploy or release code autonomously

When submitting code, include:
- Implementation of all specified requirements
- Clear explanation of approach and design decisions
- Any assumptions made about the specifications
- Known limitations or edge cases
- Testing recommendations
- Performance considerations if applicable`,
		Permissions: Permissions{
			CanRead:    []string{"**/*"},
			CanWrite:   []string{"**/*.go", "**/*.py"},
			CanExecute: []string{"task:report", "signal:send"},
		},
		Model:     "claude-haiku-4",
		MaxTokens: 8192,
	}
}

// GetBuiltinRole returns a copy of a built-in role template by name.
// Returns nil if the role name is not found.
func GetBuiltinRole(name string) *RoleTemplate {
	if role, exists := BuiltinRoles[name]; exists {
		// Return a copy to prevent external modification
		roleCopy := *role
		return &roleCopy
	}
	return nil
}

// ListBuiltinRoles returns a slice of all available built-in role names.
func ListBuiltinRoles() []string {
	roles := make([]string, 0, len(BuiltinRoles))
	for name := range BuiltinRoles {
		roles = append(roles, name)
	}
	return roles
}
