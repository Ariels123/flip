# FLIP2.md Schema Documentation

**Version:** 1.0
**Status:** Production Ready
**Purpose:** Define project-specific agent roles, commands, routing rules, and auto-context loading

---

## Overview

`FLIP2.md` is a **project configuration file** that extends FLIP2's base capabilities with:
- Custom agent role definitions and permissions
- Project-specific slash command registration
- Model routing overrides (cost optimization, capability matching)
- Auto-loading of context files for agent spawning

Place `FLIP2.md` in your **project root directory** to configure how FLIP2 agents behave within that project.

---

## Schema Sections

### 1. AGENTS Section

Defines custom agent roles with specific permissions and capabilities.

**Purpose:** Restrict agent capabilities, define team structure, enforce approval workflows

**Format:**
```yaml
## Agents

### Agent Role: [ROLE_NAME]
- **ID Pattern:** `[role-name]-*`
- **Model:** `[claude|gemini|custom]`
- **Capabilities:** `[capability1, capability2, ...]`
- **Permissions:** `[perm1, perm2, ...]`
- **Max Concurrent Tasks:** `[N]`
- **Escalation Required For:** `[action1, action2, ...]`
- **Cost Budget (USD/hour):** `[N.NN]`
- **Description:** Brief description
```

**Valid Capabilities:**
- `spawn-workers` - Can spawn other agents
- `modify-tasks` - Can create/delete tasks
- `approve-changes` - Can approve code/content changes
- `access-secrets` - Can read sensitive data
- `execute-destructive` - Can delete/modify production data
- `external-api-calls` - Can call third-party APIs
- `read-logs` - Can access system logs
- `write-config` - Can modify system configuration

**Valid Permissions:**
- `read-inbox` - Read signals/messages
- `send-signals` - Send messages to other agents
- `create-tasks` - Create new tasks
- `modify-own-tasks` - Modify tasks they own
- `modify-all-tasks` - Modify any task
- `report-status` - Send status updates
- `escalate` - Request human intervention

**Example:**
```markdown
### Agent Role: Data Analyst
- **ID Pattern:** `analyst-*`
- **Model:** `gemini`
- **Capabilities:** `read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`
- **Max Concurrent Tasks:** `5`
- **Escalation Required For:** `access-secrets, execute-destructive`
- **Cost Budget (USD/hour):** `2.50`
- **Description:** Processes data, generates reports, analyzes metrics
```

### 2. COMMANDS Section

Register project-specific slash commands that agents can execute.

**Purpose:** Extend CLI with domain-specific commands, standardize workflows

**Format:**
```markdown
## Commands

### Command: /[command-name]
- **Aliases:** `[alias1, alias2]`
- **Handler:** `[role-name]` or `[script-path]`
- **Args:** `<arg1> <arg2>`
- **Description:** What the command does
- **Requires Approval:** `[yes|no]`
- **Allowed Roles:** `[role1, role2]`
```

**Handler Types:**
- `[role-name]` - Route to specific agent role (e.g., `analyst-worker`)
- `[script-path]` - Execute local script (e.g., `./scripts/deploy.sh`)
- `builtin:[command]` - Use FLIP2 builtin command

**Example:**
```markdown
### Command: /analyze-logs
- **Aliases:** `logs, analyze`
- **Handler:** `analyst-worker`
- **Args:** `<log-file> [--filters=KEY:VALUE]`
- **Description:** Analyze system logs and generate report
- **Requires Approval:** `no`
- **Allowed Roles:** `analyst, coordinator`

### Command: /deploy
- **Aliases:** `release, push`
- **Handler:** `./scripts/deploy.sh`
- **Args:** `<environment> [--dry-run]`
- **Description:** Deploy to staging or production
- **Requires Approval:** `yes`
- **Allowed Roles:** `coordinator`
```

### 3. ROUTING Section

Override default model routing rules based on task characteristics.

**Purpose:** Cost optimization, capability matching, load balancing

**Format:**
```markdown
## Routing

### Route: [Route Name]
- **When:** Task attributes match these conditions
- **Route To:** `[role-name]` or `[model-type]`
- **Reason:** Why this routing choice
- **Cost Impact:** `[+0.50, -0.20]` (cost delta vs default)
```

**Task Attributes for Matching:**
- `task.type` - Task category (analysis, coding, research, creative, etc.)
- `task.priority` - Priority level (high, medium, low)
- `task.tokens_estimated` - Estimated token usage
- `task.complexity` - Complexity score (1-10)
- `task.deadline` - Urgency (minutes until due)
- `task.requires_accuracy` - Boolean
- `task.requires_speed` - Boolean

**Example:**
```markdown
### Route: Fast Data Analysis
- **When:** `task.type == "analysis" && task.tokens_estimated < 5000 && task.deadline > 300`
- **Route To:** `gemini`
- **Reason:** Gemini is faster and cheaper for small analysis tasks
- **Cost Impact:** `-0.30` (saves ~30% on token costs)

### Route: Complex Debugging
- **When:** `task.type == "debugging" && task.complexity > 7`
- **Route To:** `claude`
- **Reason:** Claude handles complex reasoning better for debugging
- **Cost Impact:** `+0.50` (costs more but higher success rate)

### Route: Urgent Coordination
- **When:** `task.priority == "high" && task.deadline < 60 && task.requires_speed == true`
- **Route To:** `coordinator`
- **Reason:** Human coordinator needed for rapid decisions under time pressure
- **Cost Impact:** `0` (human time, handled separately)
```

### 4. CONTEXT Section

Specify files that should be auto-loaded when spawning agents for this project.

**Purpose:** Provide agents with project context, reduce setup overhead

**Format:**
```markdown
## Context

### Auto-Load Files
- `[file-path]` - Description (optional weight: `[low|medium|high]`)
```

**File Selection Guidance:**
- Include: Architecture docs, API specs, coding standards, configuration
- Exclude: Generated files, large logs, binary data
- Use glob patterns: `*.md`, `docs/**/*.md`
- Weights: higher weight = loaded first/prioritized

**Example:**
```markdown
## Context

### Auto-Load Files
- `./README.md` - Project overview (weight: high)
- `./docs/ARCHITECTURE.md` - System design (weight: high)
- `./CODING_STANDARDS.md` - Code style guide (weight: medium)
- `./.env.example` - Environment template (weight: medium)
- `./docs/API.md` - API reference (weight: high)
- `./config/*.yaml` - Configuration files (weight: low)
```

---

## Complete Example: FLIP2.md

Create this file in your project root:

```markdown
# FLIP2.md - Project Configuration

## Overview
This file configures FLIP2 agents, commands, and routing for the DataPipeline project.

**Project:** DataPipeline
**Version:** 1.0
**Coordinator:** claude-coordinator

---

## Agents

### Agent Role: Research Lead
- **ID Pattern:** `research-*`
- **Model:** `claude`
- **Capabilities:** `spawn-workers, access-secrets, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-all-tasks, escalate`
- **Max Concurrent Tasks:** `3`
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** `5.00`
- **Description:** Leads research initiatives, spawns worker teams, approves findings

### Agent Role: Data Analyst
- **ID Pattern:** `analyst-*`
- **Model:** `gemini`
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`
- **Max Concurrent Tasks:** `5`
- **Escalation Required For:** `access-secrets, execute-destructive`
- **Cost Budget (USD/hour):** `2.50`
- **Description:** Analyzes data, generates reports, identifies patterns

### Agent Role: Code Reviewer
- **ID Pattern:** `reviewer-*`
- **Model:** `claude`
- **Capabilities:** `approve-changes, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`
- **Max Concurrent Tasks:** `4`
- **Escalation Required For:** `spawn-workers, access-secrets`
- **Cost Budget (USD/hour):** `4.00`
- **Description:** Reviews code changes, validates quality, approves PRs

---

## Commands

### Command: /analyze-pipeline
- **Aliases:** `analyze, check-pipeline, diagnose`
- **Handler:** `analyst-worker`
- **Args:** `<pipeline-id> [--depth=1-5] [--output=json|markdown]`
- **Description:** Analyze data pipeline performance and identify bottlenecks
- **Requires Approval:** `no`
- **Allowed Roles:** `analyst, research-lead, coordinator`

### Command: /research-topic
- **Aliases:** `research, explore`
- **Handler:** `research-lead`
- **Args:** `<topic> [--sources=N] [--depth=brief|detailed]`
- **Description:** Launch guided research on specified topic
- **Requires Approval:** `no`
- **Allowed Roles:** `research-lead, coordinator`

### Command: /review-code
- **Aliases:** `review, inspect-code`
- **Handler:** `./scripts/code_review.sh`
- **Args:** `<file-or-dir> [--strict] [--rules=./config/lint.yaml]`
- **Description:** Run code review with project standards
- **Requires Approval:** `yes`
- **Allowed Roles:** `reviewer, coordinator`

### Command: /deploy-verified
- **Aliases:** `release, push-prod`
- **Handler:** `./scripts/deploy.sh`
- **Args:** `<version> [--environment=staging|production]`
- **Description:** Deploy after verification checks pass
- **Requires Approval:** `yes`
- **Allowed Roles:** `coordinator`

---

## Routing

### Route: Fast Data Analysis
- **When:** `task.type == "analysis" && task.tokens_estimated < 5000 && !task.requires_accuracy`
- **Route To:** `gemini`
- **Reason:** Gemini is fast and cost-effective for exploratory analysis
- **Cost Impact:** `-0.35`

### Route: Accurate Analysis
- **When:** `task.type == "analysis" && task.requires_accuracy == true`
- **Route To:** `claude`
- **Reason:** Claude provides more accurate analysis for critical decisions
- **Cost Impact:** `+0.50`

### Route: Code Review Quality
- **When:** `task.type == "review" && task.complexity > 6`
- **Route To:** `claude`
- **Reason:** Claude excels at detailed code reviews and security analysis
- **Cost Impact:** `+0.25`

### Route: Research with Speed
- **When:** `task.type == "research" && task.deadline < 120`
- **Route To:** `gemini`
- **Reason:** Gemini's speed advantage for time-sensitive research
- **Cost Impact:** `-0.40`

### Route: Escalation
- **When:** `task.priority == "high" && task.complexity > 8`
- **Route To:** `coordinator`
- **Reason:** Complex high-priority tasks need human coordination
- **Cost Impact:** `0`

---

## Context

### Auto-Load Files
- `./README.md` - Project overview and quick start (weight: high)
- `./docs/ARCHITECTURE.md` - System design and data flow (weight: high)
- `./docs/API.md` - REST API specifications (weight: high)
- `./CODING_STANDARDS.md` - Code style and conventions (weight: medium)
- `./docs/DEPLOYMENT.md` - Deployment procedures (weight: medium)
- `./.env.example` - Environment variable template (weight: low)
- `./config/routes.yaml` - Routing configuration (weight: high)
- `./docs/TROUBLESHOOTING.md` - Common issues and solutions (weight: low)

---

## Workflow Example

**Scenario:** New research task arrives

1. **Task Created:** Research team members on market trends
   - Type: research
   - Priority: high
   - Estimated tokens: 8000

2. **Routing Decision:**
   - Matches: `task.type == "research" && deadline < 120`
   - Routes to: `gemini` (fast and cost-effective)
   - Auto-loads: README.md, ARCHITECTURE.md, API.md

3. **Agent Execution:**
   - Gemini analyst spawned with context files loaded
   - Executes `/research-topic` command
   - Generates report with findings

4. **Approval:**
   - Report sent to research-lead for review
   - If high-quality: approved automatically
   - If needs verification: escalated to claude-coordinator

---

## Best Practices

### Agents
1. **Keep role definitions focused** - Each role should have 3-4 core capabilities
2. **Set realistic budgets** - Monitor actual usage and adjust quarterly
3. **Use escalation strategically** - Escalate when cost/risk justifies human review

### Commands
1. **Use lowercase with hyphens** - `/analyze-pipeline` not `/analyzePipeline`
2. **Provide clear args** - Document all arguments and their effects
3. **Require approval for destructive commands** - Any delete/modify operation

### Routing
1. **Match on multiple conditions** - Avoid single-condition routes
2. **Document cost impact** - Help teams understand trade-offs
3. **Review quarterly** - Update routes based on actual performance

### Context
1. **Keep files <100KB each** - Large files slow agent startup
2. **Prioritize by weight** - High weight files loaded first when token budget tight
3. **Use glob patterns** - `docs/**/*.md` loads all doc files recursively

---

## Advanced: Conditional Routing

Route based on agent's recent performance:

```markdown
### Route: High Performance Claude
- **When:** `task.type == "research" && agent.success_rate > 0.9 && agent.avg_cost < 2.0`
- **Route To:** `claude`
- **Reason:** Route to performers with proven track record
- **Cost Impact:** `-0.15`
```

Route based on time of day:

```markdown
### Route: Night Operations
- **When:** `task.priority == "high" && current_hour >= 22 || current_hour < 6`
- **Route To:** `gemini`
- **Reason:** Gemini's API has lower latency at night
- **Cost Impact:** `-0.10`
```

---

## Validation Rules

FLIP2 validates FLIP2.md on startup:

1. **Agent IDs** must be unique and match pattern `[role-name]-*`
2. **Commands** must start with `/` and use lowercase
3. **Handlers** must exist (role name or valid script path)
4. **Routing conditions** must use valid task attributes
5. **Context files** must exist or use valid glob patterns
6. **Cost values** must be numeric (can be negative)

If validation fails, FLIP2 will:
1. Log detailed error message
2. Disable affected agents/commands
3. Fall back to default routing
4. Continue with reduced functionality

---

## Migration from FLIP v5

If upgrading from FLIP v5.x:

### Old Structure:
```
CLAUDE.md (global agent configuration)
```

### New Structure:
```
FLIP2.md (project-specific configuration)
CLAUDE.md (coordinator role definition, optional)
```

### Migration Steps:
1. Copy global CLAUDE.md as template
2. Create FLIP2.md in project root
3. Define project-specific agents in FLIP2.md
4. Override global routing rules as needed
5. Test with FLIP2 CLI: `flip2 validate --config ./FLIP2.md`

---

## Error Handling

### Common Validation Errors

**Error:** `Invalid agent ID: "DataAnalyst-1"`
```
Fix: Use lowercase with hyphens: "data-analyst-1"
```

**Error:** `Handler not found: analyst-worker`
```
Fix: Ensure agent with ID pattern "analyst-*" exists
```

**Error:** `File not found: ./docs/API.md`
```
Fix: Use glob pattern: ./docs/**/*.md
```

**Error:** `Routing condition syntax error`
```
Fix: Ensure conditions use valid task attributes
    Valid: task.type, task.priority, task.complexity
    Invalid: task.owner, task.duration
```

---

## Support & Resources

- **FLIP2 Main Docs:** `/flip2/README.md`
- **Deployment Guide:** `/flip2/README_DEPLOYMENT.md`
- **GitHub Issues:** https://github.com/Ariels123/flip/issues
- **Schema Validator:** `flip2 validate --help`

---

**Last Updated:** 2026-01-01
**Schema Version:** 1.0
**Status:** Production Ready
