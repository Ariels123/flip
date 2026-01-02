# FLIP2.md Template

Copy this file to your project root as `FLIP2.md` and customize for your needs.

---

```markdown
# FLIP2.md - Project Configuration

**Project:** [Your Project Name]
**Version:** 1.0
**Coordinator:** [your-coordinator-id]
**Last Updated:** [DATE]

---

## Overview

This file configures FLIP2 behavior for the [Project Name] project.

---

## Agents

Define custom agent roles with specific permissions and capabilities.

### Agent Role: [Role Name]
- **ID Pattern:** `[role-name]-*`
- **Model:** `claude` | `gemini` | `custom`
- **Capabilities:** `[capability1, capability2]`
- **Permissions:** `[permission1, permission2]`
- **Max Concurrent Tasks:** `[N]`
- **Escalation Required For:** `[action1, action2]`
- **Cost Budget (USD/hour):** `[N.NN]`
- **Description:** [Brief description of what this role does]

---

## Commands

Register project-specific slash commands.

### Command: /[command-name]
- **Aliases:** `[alias1, alias2]`
- **Handler:** `[role-name]` or `./path/to/script.sh`
- **Args:** `<arg1> [arg2]`
- **Description:** [What the command does]
- **Requires Approval:** `yes` | `no`
- **Allowed Roles:** `[role1, role2]`

---

## Routing

Define rules for routing tasks to specific agents based on task attributes.

### Route: [Route Description]
- **When:** `[condition] && [condition]`
- **Route To:** `[role]` or `[model]`
- **Reason:** [Why this routing choice]
- **Cost Impact:** `[+0.00]` (positive = more expensive, negative = cheaper)

---

## Context

Specify files that should be auto-loaded when spawning agents.

### Auto-Load Files
- `./path/to/file.md` - [Description] (weight: high|medium|low)

---
```

---

## Fill-In Guide

### Step 1: Basic Info
```markdown
**Project:** MyProject
**Version:** 1.0
**Coordinator:** claude-coordinator
```
Replace with your actual project name and coordinator agent ID.

### Step 2: Define Agents

**Example 1: Research Team**
```markdown
### Agent Role: Research Lead
- **ID Pattern:** `research-*`
- **Model:** `claude`
- **Capabilities:** `spawn-workers, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-all-tasks`
- **Max Concurrent Tasks:** `3`
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** `5.00`
- **Description:** Leads research, spawns worker teams, approves findings
```

**Example 2: Data Processing**
```markdown
### Agent Role: Processor
- **ID Pattern:** `processor-*`
- **Model:** `gemini`
- **Capabilities:** `external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** `10`
- **Escalation Required For:** `access-secrets`
- **Cost Budget (USD/hour):** `2.00`
- **Description:** Processes data, transforms formats, validates quality
```

**Example 3: Code Review**
```markdown
### Agent Role: Code Reviewer
- **ID Pattern:** `reviewer-*`
- **Model:** `claude`
- **Capabilities:** `approve-changes`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** `5`
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** `4.00`
- **Description:** Reviews code, validates quality, approves changes
```

### Step 3: Define Commands

**Example 1: Analysis Command**
```markdown
### Command: /analyze
- **Aliases:** `analyze, check, run-analysis`
- **Handler:** `processor-worker`
- **Args:** `<dataset> [--format=json|csv] [--depth=1-5]`
- **Description:** Analyze dataset and generate report
- **Requires Approval:** `no`
- **Allowed Roles:** `processor, coordinator`
```

**Example 2: Approval Command**
```markdown
### Command: /approve
- **Aliases:** `approve, accept`
- **Handler:** `./scripts/approve.sh`
- **Args:** `<item-id> [--reason=TEXT]`
- **Description:** Approve item for next stage
- **Requires Approval:** `yes`
- **Allowed Roles:** `reviewer, coordinator`
```

### Step 4: Define Routing

**Pattern: Cost Optimization**
```markdown
### Route: Fast & Cheap
- **When:** `task.tokens_estimated < 5000 && task.complexity < 5`
- **Route To:** `gemini`
- **Reason:** Gemini is faster and cheaper for simple tasks
- **Cost Impact:** `-0.35`

### Route: Accurate & Thorough
- **When:** `task.tokens_estimated >= 5000 || task.complexity >= 5`
- **Route To:** `claude`
- **Reason:** Claude provides better quality for complex tasks
- **Cost Impact:** `+0.50`
```

**Pattern: Skill-Based Routing**
```markdown
### Route: Code Reviews
- **When:** `task.type == "review"`
- **Route To:** `reviewer`
- **Reason:** Specialists handle code reviews
- **Cost Impact:** `+0.20`

### Route: Data Processing
- **When:** `task.type == "analysis"`
- **Route To:** `processor`
- **Reason:** Optimized for analytics and transformation
- **Cost Impact:** `-0.30`
```

**Pattern: Priority-Based Escalation**
```markdown
### Route: Urgent to Human
- **When:** `task.priority == "high" && task.deadline < 60`
- **Route To:** `coordinator`
- **Reason:** Human decision-making for urgent issues
- **Cost Impact:** `0`

### Route: Normal Processing
- **When:** `task.priority != "high"`
- **Route To:** `processor`
- **Reason:** Routine work goes to worker
- **Cost Impact:** `-0.40`
```

### Step 5: Define Context

**Example: Documentation Files**
```markdown
### Auto-Load Files
- `./README.md` - Project overview and quick start (weight: high)
- `./docs/ARCHITECTURE.md` - System design and data flow (weight: high)
- `./docs/API.md` - API specifications (weight: high)
- `./CODING_STANDARDS.md` - Code style guide (weight: medium)
- `./.env.example` - Environment variable template (weight: low)
```

**Example: Configuration Files**
```markdown
### Auto-Load Files
- `./config/*.yaml` - All configuration files (weight: high)
- `./docs/**/*.md` - All documentation (weight: medium)
- `./.env.example` - Environment template (weight: low)
```

---

## Validation

Before committing, verify:

```bash
# Check syntax
flip2 validate --config ./FLIP2.md

# Check specific section
flip2 validate --config ./FLIP2.md --section agents
flip2 validate --config ./FLIP2.md --section commands
flip2 validate --config ./FLIP2.md --section routing
flip2 validate --config ./FLIP2.md --section context
```

---

## Common Issues

**Issue:** Agent handler not found
```
Fix: Ensure agent ID pattern exists (e.g., for handler "processor",
     define "### Agent Role: Processor" with ID Pattern "processor-*")
```

**Issue:** File not found in context
```
Fix: Use correct relative paths from project root
     ./docs/file.md  ✓
     /absolute/path  ✗
     docs/file.md    ✗
```

**Issue:** Routing condition fails validation
```
Fix: Check attribute names are valid:
     task.type, task.priority, task.complexity ✓
     task.owner, task.created_at               ✗
```

---

## Tips

1. **Start simple** - Begin with 1-2 agents and basic routing
2. **Use meaningful names** - Make agent and command names descriptive
3. **Document cost impact** - Help teams understand trade-offs
4. **Update quarterly** - Review and update based on actual usage
5. **Test routing** - Simulate different task types before deploying

---

## Complete Minimal Example

```markdown
# FLIP2.md

## Agents

### Agent Role: Worker
- **ID Pattern:** `worker-*`
- **Model:** `gemini`
- **Capabilities:** `external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks`
- **Max Concurrent Tasks:** `5`
- **Cost Budget (USD/hour):** `2.00`
- **Description:** General purpose worker

## Commands

### Command: /work
- **Handler:** `worker`
- **Args:** `<task>`
- **Description:** Execute a task
- **Requires Approval:** `no`
- **Allowed Roles:** `worker`

## Routing

### Route: Use Gemini
- **When:** `task.complexity < 7`
- **Route To:** `gemini`
- **Cost Impact:** `-0.30`

## Context

### Auto-Load Files
- `./README.md` - Overview (weight: high)
```

---

**Ready to use!** Copy this template, fill in the sections, and save as `FLIP2.md` in your project root.

For detailed documentation, see: [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md)
