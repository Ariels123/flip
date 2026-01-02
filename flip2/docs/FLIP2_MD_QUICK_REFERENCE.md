# FLIP2.md - Quick Reference Guide

**TL;DR:** Create `FLIP2.md` in your project root to configure FLIP2 agents, commands, and routing.

---

## 30-Second Start

```markdown
# FLIP2.md

## Agents

### Agent Role: My Worker
- **ID Pattern:** `worker-*`
- **Model:** `claude`
- **Capabilities:** `spawn-workers`
- **Permissions:** `read-inbox, send-signals, create-tasks`
- **Max Concurrent Tasks:** `5`
- **Cost Budget (USD/hour):** `3.00`
- **Description:** General purpose worker

## Commands

### Command: /my-command
- **Handler:** `worker`
- **Args:** `<input>`
- **Description:** Does something useful
- **Requires Approval:** `no`
- **Allowed Roles:** `worker`

## Routing

### Route: Use Claude for Complex
- **When:** `task.complexity > 7`
- **Route To:** `claude`
- **Reason:** Better at hard problems
- **Cost Impact:** `+0.50`

## Context

### Auto-Load Files
- `./README.md` - Overview (weight: high)
```

---

## Section Cheat Sheet

### Agents
```markdown
### Agent Role: [NAME]
- **ID Pattern:** `[lowercase-role]-*`
- **Model:** `claude` | `gemini` | `custom`
- **Capabilities:** `[list separated by comma]`
- **Permissions:** `[list separated by comma]`
- **Max Concurrent Tasks:** `[number]`
- **Escalation Required For:** `[actions]`
- **Cost Budget (USD/hour):** `[0.00]`
- **Description:** [Brief text]
```

### Commands
```markdown
### Command: /[name]
- **Aliases:** `[alias1, alias2]`
- **Handler:** `[role]` | `./path/to/script`
- **Args:** `<required> [optional]`
- **Description:** [What it does]
- **Requires Approval:** `yes` | `no`
- **Allowed Roles:** `[role1, role2]`
```

### Routing
```markdown
### Route: [Description]
- **When:** `[condition1] && [condition2]`
- **Route To:** `[role]` | `[model]`
- **Reason:** [Why this routing]
- **Cost Impact:** `[+0.50]` or `[-0.30]`
```

### Context
```markdown
### Auto-Load Files
- `./path/to/file.md` - Description (weight: high|medium|low)
```

---

## Valid Capability Values

```
spawn-workers          modify-tasks            approve-changes
access-secrets         execute-destructive     external-api-calls
read-logs             write-config
```

---

## Valid Permission Values

```
read-inbox            send-signals            create-tasks
modify-own-tasks      modify-all-tasks        report-status
escalate
```

---

## Valid Task Attributes (for Routing)

```
task.type                  # String: analysis, coding, research, creative, review, debugging
task.priority              # String: high, medium, low
task.tokens_estimated      # Number: estimated token usage
task.complexity            # Number: 1-10 complexity score
task.deadline              # Number: minutes until due
task.requires_accuracy     # Boolean: true/false
task.requires_speed        # Boolean: true/false
```

---

## Routing Examples

**Route by task type:**
```markdown
- **When:** `task.type == "analysis"`
```

**Multiple conditions (AND):**
```markdown
- **When:** `task.type == "analysis" && task.complexity > 5`
```

**Alternative routes (OR - use multiple Route blocks):**
```markdown
### Route: Fast Path
- **When:** `task.tokens_estimated < 5000`
- **Route To:** `gemini`

### Route: Accurate Path
- **When:** `task.tokens_estimated >= 5000`
- **Route To:** `claude`
```

**Numeric comparisons:**
```markdown
- **When:** `task.deadline < 60`           # Due in less than 1 hour
- **When:** `task.complexity > 7`          # Hard problems
- **When:** `task.tokens_estimated <= 2000`
```

---

## File Paths

**Auto-load files in Context section:**
```markdown
- `./README.md`              # Single file
- `./docs/*.md`              # All .md files in docs/
- `./docs/**/*.md`           # Recursive in docs and subdirs
- `./.env.example`           # Template files
- `./config/routes.yaml`     # YAML config
```

---

## Common Patterns

### Pattern 1: Cost Optimization
```markdown
## Routing

### Route: Fast Analysis
- **When:** `task.type == "analysis" && task.tokens_estimated < 5000`
- **Route To:** `gemini`
- **Cost Impact:** `-0.35`

### Route: Accurate Analysis
- **When:** `task.type == "analysis" && task.tokens_estimated >= 5000`
- **Route To:** `claude`
- **Cost Impact:** `+0.50`
```

### Pattern 2: Skill-Based Routing
```markdown
## Routing

### Route: Code Review
- **When:** `task.type == "review"`
- **Route To:** `reviewer`
- **Reason:** Specialist for this task type

### Route: Data Analysis
- **When:** `task.type == "analysis"`
- **Route To:** `analyst`
- **Reason:** Optimized for analytics
```

### Pattern 3: Priority-Based Escalation
```markdown
## Routing

### Route: Urgent to Human
- **When:** `task.priority == "high" && task.deadline < 60`
- **Route To:** `coordinator`
- **Reason:** Humans handle urgent decisions

### Route: Normal to AI
- **When:** `task.priority != "high"`
- **Route To:** `gemini`
- **Cost Impact:** `-0.30`
```

---

## Validation Checklist

Before committing FLIP2.md:

- [ ] All agent IDs follow pattern `[lowercase-role]-*`
- [ ] All commands start with `/` and use lowercase
- [ ] All handlers exist (agent role or script path)
- [ ] All routing conditions use valid attributes
- [ ] All context files exist or use valid glob patterns
- [ ] All cost values are numeric (can be negative)
- [ ] No duplicate agent IDs
- [ ] No duplicate command names

**Validate with:**
```bash
flip2 validate --config ./FLIP2.md
```

---

## Troubleshooting

**Problem:** Agent not found
```
Solution: Ensure ID pattern is defined, e.g., `worker-*` for handler `worker`
```

**Problem:** Command not working
```
Solution: Check handler exists and allowed roles include yours
```

**Problem:** Files not loading as context
```
Solution: Verify file paths are correct, use glob patterns for multiple files
```

**Problem:** Routing not applied
```
Solution: Check condition syntax, must use valid task attributes
```

---

## Real-World Example

**Project:** Analytics Pipeline

```markdown
# FLIP2.md

## Agents

### Agent Role: Data Analyst
- **ID Pattern:** `analyst-*`
- **Model:** `gemini`
- **Capabilities:** `external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** `10`
- **Cost Budget (USD/hour):** `2.50`
- **Description:** Analyzes data and generates reports

### Agent Role: Reviewer
- **ID Pattern:** `reviewer-*`
- **Model:** `claude`
- **Capabilities:** `approve-changes`
- **Permissions:** `read-inbox, send-signals, modify-all-tasks`
- **Max Concurrent Tasks:** `5`
- **Cost Budget (USD/hour):** `4.00`
- **Description:** Reviews analysis, validates quality

## Commands

### Command: /analyze
- **Handler:** `analyst`
- **Args:** `<dataset> [--filters=KEY:VALUE]`
- **Description:** Analyze dataset
- **Requires Approval:** `no`
- **Allowed Roles:** `analyst, reviewer`

### Command: /publish
- **Handler:** `./scripts/publish.sh`
- **Args:** `<report-id>`
- **Description:** Publish report
- **Requires Approval:** `yes`
- **Allowed Roles:** `reviewer`

## Routing

### Route: Fast Analysis
- **When:** `task.type == "analysis" && task.tokens_estimated < 3000`
- **Route To:** `gemini`
- **Cost Impact:** `-0.40`

### Route: Quality Review
- **When:** `task.type == "review"`
- **Route To:** `claude`
- **Cost Impact:** `+0.20`

## Context

### Auto-Load Files
- `./README.md` - Project overview (weight: high)
- `./docs/ARCHITECTURE.md` - Data flow (weight: high)
- `./docs/DATASETS.md` - Available datasets (weight: high)
- `./.env.example` - Environment vars (weight: low)
```

---

## Next Steps

1. **Create FLIP2.md** in your project root
2. **Copy the template** from "30-Second Start" section
3. **Customize for your project**
4. **Validate:** `flip2 validate --config ./FLIP2.md`
5. **Commit to git** - it's checked in with the project

For full details, see: [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md)

---

**Last Updated:** 2026-01-01
**Schema Version:** 1.0
