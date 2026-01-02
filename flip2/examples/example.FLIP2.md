# FLIP2.md - Project Configuration Example

**Project:** DataAnalytics Platform
**Version:** 1.0
**Coordinator:** claude-coordinator
**Last Updated:** 2026-01-01

---

## Overview

This example demonstrates a complete FLIP2.md configuration for a data analytics platform with specialized agent roles, custom commands, intelligent routing, and auto-loaded context files.

---

## Agents

Define custom agent roles with specific permissions, capabilities, and cost budgets.

### Agent Role: Data Analyst
- **ID Pattern:** `analyst-*`
- **Model:** gemini
- **Capabilities:** `read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** `access-secrets, execute-destructive`
- **Cost Budget (USD/hour):** 2.50
- **Description:** Processes data, generates reports, analyzes metrics from various sources. Optimized for cost-effective data analysis and transformation tasks.

### Agent Role: Code Reviewer
- **ID Pattern:** `reviewer-*`
- **Model:** claude
- **Capabilities:** `approve-changes`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks, report-status`
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 4.00
- **Description:** Reviews code changes, validates quality, and approves merges. Uses Claude for superior code understanding and architectural insights.

### Agent Role: Research Lead
- **ID Pattern:** `research-*`
- **Model:** claude
- **Capabilities:** `spawn-workers, read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-all-tasks, escalate, report-status`
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 5.00
- **Description:** Leads research initiatives, spawns worker teams, coordinates with other agents, and makes critical decisions. Full analysis and synthesis capabilities.

### Agent Role: Data Quality Monitor
- **ID Pattern:** `monitor-*`
- **Model:** gemini
- **Capabilities:** `read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, report-status`
- **Max Concurrent Tasks:** 10
- **Escalation Required For:** `access-secrets, execute-destructive, modify-all-tasks`
- **Cost Budget (USD/hour):** 1.50
- **Description:** Continuously monitors data quality, validates pipelines, and reports anomalies. High throughput for routine quality checks.

---

## Commands

Register project-specific slash commands for common workflows.

### Command: /analyze
- **Aliases:** `analyze-data, check, run-analysis`
- **Handler:** `analyst-worker`
- **Args:** `<dataset> [--format=json|csv] [--depth=1-5]`
- **Description:** Analyze dataset and generate comprehensive report with metrics, trends, and insights
- **Requires Approval:** no
- **Allowed Roles:** `analyst, research, coordinator`

### Command: /review-code
- **Aliases:** `review, code-review, check-pr`
- **Handler:** `reviewer-worker`
- **Args:** `<pr-number> [--strict]`
- **Description:** Review code changes for quality, style, architecture, and potential issues
- **Requires Approval:** no
- **Allowed Roles:** `reviewer, research, coordinator`

### Command: /validate-data
- **Aliases:** `validate, check-quality, quality-check`
- **Handler:** `monitor-worker`
- **Args:** `<table-name> [--rules=FILE]`
- **Description:** Validate data integrity and quality against configured rules
- **Requires Approval:** no
- **Allowed Roles:** `monitor, analyst, coordinator`

### Command: /deploy-pipeline
- **Aliases:** `deploy, release, push-pipeline`
- **Handler:** `./scripts/deploy_pipeline.sh`
- **Args:** `<pipeline-name> <environment> [--dry-run]`
- **Description:** Deploy data pipeline to specified environment with safety checks
- **Requires Approval:** yes
- **Allowed Roles:** `research, coordinator`

### Command: /research
- **Aliases:** `investigate, research-topic`
- **Handler:** `research-worker`
- **Args:** `<topic> [--scope=broad|narrow] [--depth=quick|thorough]`
- **Description:** Spawn research team to investigate topic and provide synthesis
- **Requires Approval:** no
- **Allowed Roles:** `research, coordinator`

---

## Routing

Define rules for routing tasks to agents based on characteristics, cost, and requirements.

### Route: Fast Data Analysis
- **When:** `task.type == "analysis" && task.tokens_estimated < 5000 && task.complexity < 5`
- **Route To:** `gemini`
- **Reason:** Gemini is faster and cheaper for straightforward analysis tasks with clear requirements
- **Cost Impact:** `-0.30`

### Route: Complex Data Processing
- **When:** `task.type == "analysis" && task.tokens_estimated >= 5000 || task.complexity >= 7`
- **Route To:** `claude`
- **Reason:** Claude provides superior reasoning for complex data transformations and novel insights
- **Cost Impact:** `+0.50`

### Route: Code Review Expertise
- **When:** `task.type == "review" && task.requires_accuracy == true`
- **Route To:** `claude`
- **Reason:** Claude's architectural understanding and code reasoning significantly improves review quality
- **Cost Impact:** `+0.40`

### Route: Quality Monitoring
- **When:** `task.type == "monitoring" || task.type == "validation"`
- **Route To:** `gemini`
- **Reason:** Routine quality checks and monitoring are cost-effective with Gemini
- **Cost Impact:** `-0.40`

### Route: Urgent Research Coordination
- **When:** `task.priority == "high" && task.deadline < 60 && task.requires_speed == true`
- **Route To:** `research`
- **Reason:** Research lead provides rapid coordination and decision-making under time pressure
- **Cost Impact:** `+0.00`

### Route: Standard Analysis
- **When:** `task.type == "analysis" && task.priority == "normal"`
- **Route To:** `gemini`
- **Reason:** Default route for routine analysis work optimizes cost
- **Cost Impact:** `-0.30`

### Route: High Accuracy Requirements
- **When:** `task.requires_accuracy == true && task.complexity > 5`
- **Route To:** `claude`
- **Reason:** When accuracy is critical, Claude's superior reasoning justifies higher cost
- **Cost Impact:** `+0.60`

---

## Context

Specify files to auto-load when spawning agents for this project.

### Auto-Load Files
- `./README.md` - Project overview and quick start guide (weight: high)
- `./docs/ARCHITECTURE.md` - System design, data flow, and component relationships (weight: high)
- `./docs/DATA_MODELS.md` - Data schema and model definitions (weight: high)
- `./CODING_STANDARDS.md` - Code style guide and best practices (weight: medium)
- `./docs/API_REFERENCE.md` - API specifications and endpoint documentation (weight: high)
- `./docs/PIPELINE_GUIDE.md` - Data pipeline configuration and operations (weight: medium)
- `./.env.example` - Environment variables template (weight: low)
- `./config/*.yaml` - All configuration files (weight: medium)
- `./docs/TROUBLESHOOTING.md` - Common issues and solutions (weight: low)

---

## Example Workflow

### Scenario 1: Routine Data Analysis
1. User runs: `/analyze sales_data --format=json --depth=3`
2. Command routes to `analyst-worker` handler
3. Agent loads context files (README, DATA_MODELS, API_REFERENCE)
4. Task evaluated: `tokens_estimated=3000, complexity=4, type=analysis`
5. Routing matches "Fast Data Analysis" → uses Gemini (-0.30 cost impact)
6. Report generated and returned

### Scenario 2: Complex Research with Approval
1. User runs: `/deploy-pipeline etl-pipeline production`
2. Command requires approval → escalates to coordinator
3. After approval, executes `./scripts/deploy_pipeline.sh`
4. High priority, urgent task → routes to `research` leader
5. Research agent spawns monitoring agents
6. Uses Claude for deployment verification
7. Reports back to coordinator with success/failure details

### Scenario 3: Quality Monitoring (Continuous)
1. Scheduled task monitors data quality every hour
2. Task type: `monitoring`, priority: `normal`
3. Routes to Gemini via "Quality Monitoring" rule
4. Runs validation checks from `./docs/PIPELINE_GUIDE.md`
5. Reports anomalies back to team

---

## Configuration Notes

### Cost Optimization
- **Analyst role (Gemini):** Optimized for high-volume, lower-complexity tasks
- **Reviewer role (Claude):** Specialized for code quality where accuracy matters most
- **Research role (Claude):** Handles complex reasoning and coordination
- **Monitor role (Gemini):** Handles routine checks at scale

### Capability Restrictions
- Only `research` agents can spawn workers
- Only `reviewer` agents can approve changes
- Secrets access requires escalation for all roles except coordinators

### Skill-Based Routing
The routing rules implement intelligent task assignment:
- Simple analysis → Gemini (fast, cheap)
- Complex analysis → Claude (accurate, thorough)
- Code review → Claude (architectural understanding)
- Monitoring → Gemini (high throughput, cost-effective)

### Context Organization
Auto-loaded context is prioritized by weight:
1. **High weight:** Architecture, data models, API specs - loaded first
2. **Medium weight:** Standards, guides - loaded second
3. **Low weight:** Templates, templates - loaded last

This ensures agents have critical context available first.

---

## Validation

Before using this configuration in production:

```bash
# Validate syntax and schema
flip2 validate --config ./FLIP2.md

# Validate specific sections
flip2 validate --config ./FLIP2.md --section agents
flip2 validate --config ./FLIP2.md --section commands
flip2 validate --config ./FLIP2.md --section routing
flip2 validate --config ./FLIP2.md --section context
```

---

## Next Steps

1. **Customize for your project:**
   - Replace agent names with your team structure
   - Update handlers to match your scripts
   - Adjust cost budgets based on your actual costs
   - Update context files to match your documentation

2. **Test routing:**
   - Create test tasks with various priorities/complexities
   - Verify they route to expected agents
   - Monitor costs and adjust routes if needed

3. **Monitor performance:**
   - Track which routes are actually used
   - Review cost impact vs. quality outcomes
   - Update routes quarterly based on actual usage

4. **Document custom extensions:**
   - Add team-specific capabilities
   - Define project-specific task attributes
   - Create guidelines for new agent roles

---

**Status:** Production Ready
**Last Updated:** 2026-01-01
**Created for:** FLIP2 Configuration Parser Example
