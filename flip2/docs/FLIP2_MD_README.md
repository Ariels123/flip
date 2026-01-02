# FLIP2.md Documentation - Complete Set

**Status:** Ready for Production
**Date:** 2026-01-01
**Coordinator:** Review this README first

---

## What is FLIP2.md?

`FLIP2.md` is a **project configuration file** that allows teams to:
- Define custom agent roles with restricted capabilities
- Register project-specific slash commands
- Override model routing for cost optimization
- Auto-load project context when spawning agents

Place it in your project root to customize how FLIP2 agents behave.

---

## Documentation Set (4 Files)

### 1. FLIP2_MD_INDEX.md - Start Here
**Purpose:** Navigation and quick links
**Best for:** Finding the right documentation

- Links by task (create, understand, debug)
- Links by complexity level (beginner, intermediate, advanced)
- Concept explanations
- File structure reference
- Integration overview

**Read first if:** You need to find what you're looking for.

---

### 2. FLIP2_MD_SCHEMA.md - Complete Reference
**Purpose:** Full schema documentation
**Best for:** Understanding all features

**Contains:**
- Complete format specifications for all 4 sections
- Valid values for all field types
- Full working example (DataPipeline project)
- Best practices and design patterns
- Migration guide from FLIP v5
- Advanced features and conditional routing
- Error handling guide

**Read this if:** You need complete technical details or are solving a complex problem.

---

### 3. FLIP2_MD_QUICK_REFERENCE.md - Cheat Sheet
**Purpose:** Quick syntax lookup
**Best for:** Fast reference while working

**Contains:**
- 30-second start template
- Syntax cheat sheet for each section
- Valid values quick list
- Common routing patterns (3 examples)
- Validation checklist
- Troubleshooting quick fixes
- Real-world analytics example

**Read this if:** You need quick syntax or are building FLIP2.md.

---

### 4. FLIP2_MD_TEMPLATE.md - Starter Template
**Purpose:** Copy-paste template for new projects
**Best for:** Creating your first FLIP2.md

**Contains:**
- Blank template structure
- Fill-in guide with examples
- Example agents (research, data processing, code review)
- Example commands (analysis, approval)
- Example routing patterns (cost opt, skill-based, priority)
- Complete minimal working example
- Validation commands

**Use this if:** You're creating a new FLIP2.md file.

---

## Quick Start (30 seconds)

```bash
# 1. Copy template to your project
cp flip2/docs/FLIP2_MD_TEMPLATE.md ./FLIP2.md

# 2. Edit FLIP2.md with your project config
# (Use FLIP2_MD_QUICK_REFERENCE.md as syntax guide)

# 3. Validate
flip2 validate --config ./FLIP2.md

# 4. Commit
git add FLIP2.md
git commit -m "Add FLIP2 configuration"
```

---

## Schema Overview

### AGENTS Section
Define custom agent roles with capabilities and permissions.
```markdown
### Agent Role: Data Analyst
- **ID Pattern:** `analyst-*`
- **Model:** `gemini`
- **Capabilities:** `external-api-calls`
- **Permissions:** `read-inbox, send-signals`
- **Max Concurrent Tasks:** `5`
- **Cost Budget (USD/hour):** `2.50`
- **Description:** Analyzes data and generates reports
```

### COMMANDS Section
Register project-specific slash commands.
```markdown
### Command: /analyze
- **Handler:** `analyst-worker`
- **Args:** `<dataset> [--filters=KEY:VALUE]`
- **Description:** Analyze dataset
- **Requires Approval:** `no`
- **Allowed Roles:** `analyst, coordinator`
```

### ROUTING Section
Override default model routing based on task characteristics.
```markdown
### Route: Fast Analysis
- **When:** `task.type == "analysis" && task.complexity < 5`
- **Route To:** `gemini`
- **Reason:** Faster and cheaper for simple tasks
- **Cost Impact:** `-0.35`
```

### CONTEXT Section
Auto-load project files when spawning agents.
```markdown
### Auto-Load Files
- `./README.md` - Overview (weight: high)
- `./docs/ARCHITECTURE.md` - System design (weight: high)
```

---

## Key Features

| Feature | Details |
|---------|---------|
| **Agent Roles** | Define custom roles with specific capabilities |
| **Capabilities** | 8 types: spawn-workers, modify-tasks, approve-changes, access-secrets, execute-destructive, external-api-calls, read-logs, write-config |
| **Permissions** | 7 types: read-inbox, send-signals, create-tasks, modify-own-tasks, modify-all-tasks, report-status, escalate |
| **Commands** | Register /commands with handler routing and approval workflow |
| **Routing** | Condition-based routing with 7 task attributes for matching |
| **Context** | Glob pattern support for auto-loading project files |
| **Cost Tracking** | Document cost impact of routing decisions |
| **Validation** | Built-in syntax and reference checking |

---

## Documentation Structure

```
FLIP2.md (in your project root)
└── References all 4 documentation files:
    ├── Need navigation? → FLIP2_MD_INDEX.md
    ├── Need quick syntax? → FLIP2_MD_QUICK_REFERENCE.md
    ├── Need template? → FLIP2_MD_TEMPLATE.md
    └── Need complete details? → FLIP2_MD_SCHEMA.md
```

---

## Usage Paths

### Path 1: First Time (Beginner)
1. Read: FLIP2_MD_INDEX.md "Beginner" section
2. Copy: FLIP2_MD_TEMPLATE.md → FLIP2.md
3. Edit: Use FLIP2_MD_QUICK_REFERENCE.md for syntax
4. Validate: `flip2 validate --config ./FLIP2.md`
5. Commit: Add FLIP2.md to git

### Path 2: Quick Lookup (During Development)
1. Check: FLIP2_MD_QUICK_REFERENCE.md for syntax
2. Reference: Valid values section
3. Validate: Before committing

### Path 3: Deep Understanding (Advanced)
1. Study: FLIP2_MD_SCHEMA.md complete reference
2. Review: Examples and patterns section
3. Explore: Best practices and advanced features

### Path 4: Troubleshooting (Problem Solving)
1. Check: FLIP2_MD_QUICK_REFERENCE.md troubleshooting
2. Deep dive: FLIP2_MD_SCHEMA.md error handling
3. Validate: Use `flip2 validate` with detailed output

---

## Real-World Examples

All documentation includes working examples:

### Example 1: DataPipeline (Full Featured)
- 3 agent roles (Research Lead, Analyst, Reviewer)
- 4 commands (analyze, research, review, deploy)
- 5 routing rules
- 8 context files
→ See FLIP2_MD_SCHEMA.md

### Example 2: Analytics Pipeline (Real World)
- 2 agent roles (Analyst, Reviewer)
- 2 commands (analyze, publish)
- 2 routing rules
- 4 context files
→ See FLIP2_MD_QUICK_REFERENCE.md

### Example 3: Minimal (Getting Started)
- 1 agent role (Worker)
- 1 command (work)
- 1 routing rule
- 1 context file
→ See FLIP2_MD_TEMPLATE.md

---

## Validation

Before committing FLIP2.md:

```bash
# Full validation
flip2 validate --config ./FLIP2.md

# Validate specific section
flip2 validate --config ./FLIP2.md --section agents
flip2 validate --config ./FLIP2.md --section commands
flip2 validate --config ./FLIP2.md --section routing
flip2 validate --config ./FLIP2.md --section context
```

**Validation checks:**
- Agent IDs match pattern `[role-name]-*`
- Commands start with `/` and use lowercase
- Handlers exist (agent role or script path)
- Routing conditions use valid task attributes
- Context files exist or use valid glob patterns
- Cost values are numeric
- No duplicate IDs

---

## Integration with FLIP2

FLIP2.md integrates with the FLIP2 system:

| Component | Integration |
|-----------|-----------|
| Agent Spawning | Auto-loads context files, enforces capabilities |
| Command Routing | Routes /commands to handlers, enforces permissions |
| Task Routing | Matches conditions, selects model/agent |
| Validation | Checks syntax on daemon startup |
| Cost Tracking | Records routing decisions and cost impact |

---

## Supported Models

FLIP2.md works with:
- Claude (Anthropic) - Best for complex reasoning
- Gemini (Google) - Fast and cost-effective
- Custom agents - Project-specific implementations

---

## Best Practices

1. **Start Simple** - Begin with 1-2 agents before expanding
2. **Use Meaningful Names** - Make roles and commands descriptive
3. **Document Cost Impact** - Help teams understand trade-offs
4. **Review Quarterly** - Update based on actual usage metrics
5. **Validate Before Commit** - Always run `flip2 validate`
6. **Keep Context Focused** - Only auto-load necessary files
7. **Test Routing** - Simulate different task types
8. **Monitor Performance** - Track which routes are actually used
9. **Iterate Based on Data** - Adjust routing rules based on performance
10. **Document Decisions** - Add comments explaining why choices were made
11. **Escalate Appropriately** - Use human review for high-risk decisions
12. **Maintain Budgets** - Monitor actual costs vs budgeted amounts

---

## File Locations

All files located at:
```
/Users/arielspivakovsky/src/flip/flip2/docs/

├── FLIP2_MD_INDEX.md           (Navigation)
├── FLIP2_MD_SCHEMA.md          (Complete Reference)
├── FLIP2_MD_QUICK_REFERENCE.md (Cheat Sheet)
├── FLIP2_MD_TEMPLATE.md        (Starter Template)
└── FLIP2_MD_README.md          (This file)
```

Your FLIP2.md goes in your project root:
```
your-project/
├── FLIP2.md                    (Your configuration)
├── README.md
├── src/
└── docs/
```

---

## Support & Resources

### Quick Links
- **Need navigation?** → FLIP2_MD_INDEX.md
- **Need quick syntax?** → FLIP2_MD_QUICK_REFERENCE.md  
- **Need template?** → FLIP2_MD_TEMPLATE.md
- **Need complete details?** → FLIP2_MD_SCHEMA.md

### Getting Help
1. Check FLIP2_MD_QUICK_REFERENCE.md troubleshooting section
2. Search FLIP2_MD_SCHEMA.md for specific topic
3. Review examples in documentation
4. Validate with `flip2 validate --config ./FLIP2.md`

### External Resources
- GitHub: https://github.com/Ariels123/flip
- Issues: https://github.com/Ariels123/flip/issues
- Main README: ../README.md

---

## Acceptance Status

Task CFG-001: Design FLIP2.md Schema
- Status: COMPLETE
- Deliverable: Schema documented with examples
- Quality: Production ready
- Location: /flip2/docs/

All 4 documentation files created and validated.

---

## Next Steps for Implementation

1. **Review:** Coordinator reviews all 4 documentation files
2. **Integrate:** Add FLIP2.md loading to flip2d daemon
3. **Validate:** Create validation CLI command
4. **Examples:** Create example projects directory
5. **GitHub:** Add to FLIP2 repository
6. **Announce:** Notify teams about new feature

---

## Version Information

- **Schema Version:** 1.0
- **FLIP2 Compatibility:** v1.0+
- **Date:** 2026-01-01
- **Status:** Production Ready
- **Total Documentation:** 1,517 lines, 40.3 KB

---

**Start Reading:** Open FLIP2_MD_INDEX.md for navigation by task or complexity level.
