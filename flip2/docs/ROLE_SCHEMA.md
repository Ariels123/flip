# Role Template Schema

## Overview

Role templates define the configuration, capabilities, and constraints for spawnable worker agents in the FLIP system. Each role encapsulates a specific purpose, provides a consistent system prompt, and enforces permission boundaries.

## RoleTemplate Structure

```go
type RoleTemplate struct {
    Name           string      // Unique identifier
    Description    string      // Purpose explanation
    SystemPrompt   string      // Initial agent context
    Permissions    Permissions // Access controls
    Model          string      // Default LLM model
    MaxTokens      int         // Maximum token limit
}
```

### Field Descriptions

#### Name
- **Type**: `string`
- **Required**: Yes
- **Format**: Alphanumeric with hyphens (e.g., `research-worker`, `code-reviewer`)
- **Purpose**: Unique identifier for the role, used when spawning agents
- **Example**: `"data-analyzer"`

#### Description
- **Type**: `string`
- **Required**: Yes
- **Purpose**: Explains the intended use case and what this role specializes in
- **Purpose**: Helps coordinators decide which role to use for specific tasks
- **Example**: `"Analyzes datasets and generates statistical reports"`

#### SystemPrompt
- **Type**: `string`
- **Required**: Yes
- **Purpose**: Initial system context provided to every agent spawned with this role
- **Contains**:
  - Role identity and purpose
  - Worker context (signals that this is a worker agent)
  - Behavioral guidelines
  - Reporting requirements
  - Available tools and capabilities
  - Any constraints or restrictions
- **Best Practices**:
  - Always include "You are a WORKER agent" statement
  - Clearly define reporting expectations
  - Specify what to do when stuck or blocked
  - List any constraints on decision-making

#### Permissions
- **Type**: `Permissions` struct
- **Purpose**: Controls what operations the role is allowed to perform
- **Fields**:
  - `CanRead`: Resource patterns this role can read (e.g., `"logs/*"`, `"config/public/*"`)
  - `CanWrite`: Resource patterns this role can modify
  - `CanExecute`: Operations/commands this role can execute (e.g., `"spawn:worker"`, `"task:report"`)
- **Example**:
  ```go
  Permissions: Permissions{
      CanRead:    []string{"logs/*", "state/*"},
      CanWrite:   []string{"logs/worker/*", "state/temp/*"},
      CanExecute: []string{"task:report", "signal:send"},
  }
  ```

#### Model
- **Type**: `string`
- **Required**: No (uses system default if empty)
- **Purpose**: Specifies the default LLM model for agents with this role
- **Valid Values**:
  - `"claude-opus-4-5"` - Advanced reasoning, code generation
  - `"claude-sonnet-4"` - Balanced speed/quality
  - `"gemini-2.5-pro"` - Research, data processing
  - `"gpt-4"` - General purpose
- **Example**: `"claude-opus-4-5"`

#### MaxTokens
- **Type**: `int`
- **Required**: Yes (must be > 0)
- **Purpose**: Controls maximum response size and helps manage API costs
- **Typical Values**:
  - `2048` - Short responses, cost-optimized
  - `4096` - Standard responses
  - `8192` - Long-form content
  - `16384` - Full research reports
- **Example**: `4096`

## Permissions Structure

```go
type Permissions struct {
    CanRead    []string  // Readable resources
    CanWrite   []string  // Writable resources
    CanExecute []string  // Executable operations
}
```

### Permission Patterns

Permissions use wildcard patterns for flexibility:

- `"*"` - Allow all
- `"logs/*"` - All files in logs directory
- `"config/public/*"` - All files in public config
- `"task:*"` - All task operations
- `"signal:send"` - Specific signal send operation

## Example Role Definitions

### Data Analysis Worker

```json
{
  "name": "data-analyzer",
  "description": "Processes datasets and generates statistical analysis reports",
  "system_prompt": "You are a WORKER agent in the FLIP system. Your coordinator assigned you to analyze datasets and generate reports. Focus on statistical accuracy and clear visualizations. Report your findings, do not make autonomous decisions. If you encounter issues, signal the coordinator for help. Do not spawn additional agents without explicit coordinator approval.",
  "permissions": {
    "can_read": ["data/*", "reports/template/*"],
    "can_write": ["reports/output/*"],
    "can_execute": ["task:report", "signal:send"]
  },
  "model": "gemini-2.5-pro",
  "max_tokens": 8192
}
```

### Code Review Worker

```json
{
  "name": "code-reviewer",
  "description": "Reviews code changes, identifies issues, and suggests improvements",
  "system_prompt": "You are a WORKER agent assigned to perform code reviews. Analyze the provided code for: correctness, performance, security, readability, and test coverage. Provide specific actionable feedback. Do not make commits or approve PRs autonomously. Report all findings to the coordinator. If you identify critical security issues, immediately signal the coordinator.",
  "permissions": {
    "can_read": ["code/*", "tests/*", "docs/*"],
    "can_write": ["reviews/*"],
    "can_execute": ["signal:send", "task:report"]
  },
  "model": "claude-opus-4-5",
  "max_tokens": 6144
}
```

### Research Worker

```json
{
  "name": "research-worker",
  "description": "Conducts web research and synthesizes findings into reports",
  "system_prompt": "You are a WORKER agent tasked with conducting research and compiling information. Your role is to gather and organize data, not to make final decisions. Always cite sources and provide evidence for claims. Report your research findings to the coordinator. If you cannot find sufficient information, report back rather than making assumptions. Do not create final deliverables without coordinator approval.",
  "permissions": {
    "can_read": ["research/context/*"],
    "can_write": ["research/temp/*"],
    "can_execute": ["browse:web", "task:report", "signal:send"]
  },
  "model": "gemini-2.5-pro",
  "max_tokens": 10240
}
```

### Testing Worker

```json
{
  "name": "test-executor",
  "description": "Runs automated tests and reports results with detailed logs",
  "system_prompt": "You are a WORKER agent assigned to execute tests and validate code. Run the test suite, capture output, and report results. Track pass/fail status and identify flaky tests. Report detailed logs to the coordinator. Do not modify production code. If tests fail, provide diagnostic information to help the coordinator understand the issues.",
  "permissions": {
    "can_read": ["code/*", "tests/*", "config/*"],
    "can_write": ["test-results/*", "logs/*"],
    "can_execute": ["run:tests", "task:report", "signal:send"]
  },
  "model": "claude-sonnet-4",
  "max_tokens": 4096
}
```

## Role Design Best Practices

### 1. Clear Purpose
Each role should have a well-defined, specific purpose. Avoid overly broad roles.

```go
// Good
"code-reviewer"  // Specific to code review

// Bad
"general-worker" // Too vague
```

### 2. Appropriate Permissions
Grant only the minimum permissions needed for the role's purpose.

```go
// Good - restrictive
Permissions{
    CanRead:    []string{"data/*"},
    CanWrite:   []string{"reports/*"},
    CanExecute: []string{"task:report"},
}

// Bad - too permissive
Permissions{
    CanRead:    []string{"*"},
    CanWrite:   []string{"*"},
    CanExecute: []string{"*"},
}
```

### 3. Worker Identity in SystemPrompt
Always include clear language establishing worker context:

```
"You are a WORKER agent in the FLIP system. Your coordinator is the main Claude instance.
Complete this task and report back with your findings. Do not make autonomous decisions."
```

### 4. Realistic MaxTokens
Choose appropriate token limits based on the role's typical output.

- Data analysis reports: 8192-10240
- Code reviews: 4096-6144
- Short status updates: 2048
- Research synthesis: 10240-16384

### 5. Consistent System Prompts
Use a standard template for all system prompts:

1. Worker identity statement
2. Role-specific purpose
3. Behavioral expectations
4. Reporting requirements
5. Constraint details
6. Escalation procedures

## Validation

RoleTemplate includes a `Validate()` method that checks:
- Name is non-empty
- Description is non-empty
- SystemPrompt is non-empty
- MaxTokens is positive (> 0)

All four fields are required for a valid role definition.

## Usage in Code

```go
package main

import "flip2/internal/spawn"

// Define a role template
role := spawn.RoleTemplate{
    Name:        "data-analyzer",
    Description: "Processes datasets and generates reports",
    SystemPrompt: "You are a WORKER agent assigned to analyze data...",
    Permissions: spawn.Permissions{
        CanRead:    []string{"data/*"},
        CanWrite:   []string{"reports/*"},
        CanExecute: []string{"task:report", "signal:send"},
    },
    Model:      "gemini-2.5-pro",
    MaxTokens:  8192,
}

// Validate the role
if err := role.Validate(); err != nil {
    log.Fatal(err)
}

// Use the role to spawn agents
// (spawn implementation details in separate modules)
```

## Related Concepts

- **Agent Manager**: Manages registered agents and their lifecycle
- **Task System**: Assigns work to agents based on capabilities
- **Signal System**: Enables communication between coordinator and workers
- **Permission Enforcement**: System validates operations against role permissions
