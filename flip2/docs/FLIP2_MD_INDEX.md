# FLIP2.md Documentation Index

Complete reference for FLIP2.md project configuration files.

---

## Documents

### 1. FLIP2_MD_SCHEMA.md (Full Reference)
**Purpose:** Comprehensive schema documentation
**Length:** ~500 lines
**Best For:** Understanding all features, implementation details, advanced usage

**Sections:**
- Overview and use cases
- Complete schema definition (agents, commands, routing, context)
- Valid values for all fields
- Complete working example with all sections
- Best practices and patterns
- Migration guide from FLIP v5
- Troubleshooting guide

**Start Here If:** You need to understand the complete schema or solve a specific problem.

---

### 2. FLIP2_MD_QUICK_REFERENCE.md (Cheat Sheet)
**Purpose:** Quick lookup for common tasks
**Length:** ~200 lines
**Best For:** Quick lookups, syntax reference, common patterns

**Sections:**
- 30-second start template
- Section cheat sheet with syntax
- Valid values list
- Routing examples
- Common patterns (cost optimization, skill-based routing)
- Validation checklist
- Troubleshooting quick fixes
- Real-world example

**Start Here If:** You want a quick reference while building FLIP2.md.

---

### 3. FLIP2_MD_TEMPLATE.md (Starter Template)
**Purpose:** Copy-paste template for new projects
**Length:** ~250 lines
**Best For:** Creating new FLIP2.md files

**Sections:**
- Minimal template structure
- Fill-in guide for each section
- Example code for each agent/command/routing pattern
- Validation commands
- Common issues and fixes
- Tips for getting started
- Complete minimal example

**Start Here If:** You're creating FLIP2.md for the first time.

---

## Quick Navigation

### By Task

**I want to create a new FLIP2.md:**
1. Open [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md)
2. Copy template to project root
3. Follow "Fill-In Guide"
4. Validate with `flip2 validate`

**I need to understand a concept:**
1. Check [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) "Valid Values" section
2. If not there, see [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) full documentation

**I'm debugging an error:**
1. Look in [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) "Troubleshooting"
2. Check [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) "Error Handling" section

**I want to see a working example:**
1. [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) has a real-world example
2. [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) has DataPipeline example
3. [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md) has minimal example

### By Complexity Level

**Beginner (Just starting):**
1. Read: Overview in this document
2. Copy: Template from [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md)
3. Reference: [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md)

**Intermediate (Building real configs):**
1. Study: Examples in [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md)
2. Implement: Use [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md)
3. Optimize: Apply patterns from [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md)

**Advanced (Customization, troubleshooting):**
1. Deep dive: Full [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md)
2. Patterns: See "Advanced" and "Best Practices" sections
3. Validation: Use `flip2 validate` with detailed output

---

## Key Concepts

### FLIP2.md vs CLAUDE.md

| Aspect | FLIP2.md | CLAUDE.md |
|--------|----------|----------|
| Scope | Per-project configuration | Global agent defaults |
| Location | Project root | `~/.claude/` |
| Purpose | Project-specific agents, commands, routing | Global coordinator role |
| Versioning | Checked into git | Not versioned |
| Precedence | Overrides CLAUDE.md | Fallback if no FLIP2.md |

### Agents

**What:** Define custom agent roles
**Why:** Restrict capabilities, organize teams, enforce approval workflows
**Where:** `## Agents` section
**Example:**
```markdown
### Agent Role: Analyst
- **ID Pattern:** `analyst-*`
- **Model:** `gemini`
```

### Commands

**What:** Register project-specific slash commands
**Why:** Standardize workflows, provide shorthand for common tasks
**Where:** `## Commands` section
**Example:**
```markdown
### Command: /analyze
- **Handler:** `analyst-worker`
```

### Routing

**What:** Override model assignment based on task attributes
**Why:** Cost optimization, capability matching, load balancing
**Where:** `## Routing` section
**Example:**
```markdown
### Route: Fast Analysis
- **When:** `task.complexity < 5`
- **Route To:** `gemini`
```

### Context

**What:** Auto-load files when spawning agents
**Why:** Reduce setup overhead, ensure consistent context
**Where:** `## Context` section
**Example:**
```markdown
### Auto-Load Files
- `./README.md` - Overview (weight: high)
```

---

## File Structure Reference

### Valid Capability Values
```
spawn-workers           modify-tasks            approve-changes
access-secrets         execute-destructive     external-api-calls
read-logs             write-config
```

See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) for complete list.

### Valid Permission Values
```
read-inbox            send-signals            create-tasks
modify-own-tasks      modify-all-tasks        report-status
escalate
```

See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) for complete list.

### Valid Task Attributes (for Routing)
```
task.type              task.priority           task.tokens_estimated
task.complexity        task.deadline           task.requires_accuracy
task.requires_speed
```

See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) for details.

---

## Validation Checklist

Before committing FLIP2.md:

- [ ] Agent IDs follow pattern `[lowercase-role]-*`
- [ ] Commands start with `/` and use lowercase
- [ ] Handlers exist (agent role or valid script path)
- [ ] Routing conditions use valid task attributes
- [ ] Context files exist or use valid glob patterns
- [ ] Cost values are numeric (positive or negative)
- [ ] No duplicate agent IDs
- [ ] No duplicate command names
- [ ] Capabilities are from valid list
- [ ] Permissions are from valid list

**Run:** `flip2 validate --config ./FLIP2.md`

---

## Workflow Example

### Scenario: New Analytics Project

**Step 1: Create FLIP2.md**
```bash
cp flip2/docs/FLIP2_MD_TEMPLATE.md ./FLIP2.md
```

**Step 2: Define your agents**
```markdown
### Agent Role: Data Analyst
- **ID Pattern:** `analyst-*`
- **Model:** `gemini`
...
```

**Step 3: Define your commands**
```markdown
### Command: /analyze
- **Handler:** `analyst-worker`
...
```

**Step 4: Define routing rules**
```markdown
### Route: Fast Analysis
- **When:** `task.complexity < 5`
- **Route To:** `gemini`
...
```

**Step 5: Define context files**
```markdown
### Auto-Load Files
- `./README.md` - Overview (weight: high)
...
```

**Step 6: Validate**
```bash
flip2 validate --config ./FLIP2.md
```

**Step 7: Commit**
```bash
git add FLIP2.md
git commit -m "Add FLIP2 configuration for analytics project"
```

---

## Examples by Use Case

### Data Processing Project
See [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md) - "Analytics Pipeline" example

### Research Team Project
See [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) - "DataPipeline" example

### Cost Optimization Focus
See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) - "Pattern 1" section

### Skill-Based Routing
See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) - "Pattern 2" section

---

## Common Patterns

### Pattern 1: Cost Optimization
Route simple tasks to cheaper models, complex tasks to more capable models.
**Reference:** [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) - "Routing" section

### Pattern 2: Skill-Based Routing
Route tasks to agents specialized in specific domains.
**Reference:** [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md) - "Step 4" examples

### Pattern 3: Priority-Based Escalation
Route urgent tasks to human coordinators, routine tasks to AI workers.
**Reference:** [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) - "Pattern 3" section

---

## Troubleshooting Guide

### Problem: "Agent not found"
**Cause:** Handler references non-existent agent role
**Fix:** See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) - Troubleshooting

### Problem: "File not found in context"
**Cause:** File path doesn't exist or uses wrong syntax
**Fix:** See [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) - "Context" section

### Problem: "Invalid routing condition"
**Cause:** Condition uses wrong task attributes
**Fix:** See [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md) - Valid Attributes

For more issues, see:
- [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) - "Error Handling"
- [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md) - "Common Issues"

---

## Best Practices

1. **Start simple** - Minimal agents/commands first
2. **Use meaningful names** - Descriptive role and command names
3. **Document cost impact** - Help teams understand trade-offs
4. **Review quarterly** - Update based on actual usage and performance
5. **Test before commit** - Use `flip2 validate` before git commit
6. **Keep context focused** - Only auto-load necessary files
7. **Monitor routing** - Track which routes are actually used

See [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) - "Best Practices" for details.

---

## Integration with FLIP2

FLIP2.md integrates with FLIP2 system:

1. **Agent Spawning:** Auto-loads context files when spawning agents
2. **Command Execution:** Routes `/command` calls to specified handlers
3. **Task Routing:** Matches tasks to agents based on routing rules
4. **Validation:** `flip2 validate` checks syntax and references

**Related:** See main [../README.md](../README.md) for FLIP2 system overview.

---

## Updates & Versioning

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-01-01 | Initial schema with 4 sections: agents, commands, routing, context |

Check [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md) "Version History" for detailed changes.

---

## Support

- **Quick Syntax:** [FLIP2_MD_QUICK_REFERENCE.md](./FLIP2_MD_QUICK_REFERENCE.md)
- **Complete Reference:** [FLIP2_MD_SCHEMA.md](./FLIP2_MD_SCHEMA.md)
- **Getting Started:** [FLIP2_MD_TEMPLATE.md](./FLIP2_MD_TEMPLATE.md)
- **Validation:** `flip2 validate --help`
- **Issues:** https://github.com/Ariels123/flip/issues

---

**Last Updated:** 2026-01-01
**Schema Version:** 1.0
**Status:** Production Ready
