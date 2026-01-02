# FLIP2.md - Go Web Service Configuration

**Project:** Go Web Service
**Version:** 1.0
**Coordinator:** claude-coordinator
**Last Updated:** 2026-01-01

---

## Overview

This FLIP2.md configuration optimizes agent routing and task execution for Go web service development, testing, and deployment. Specialized roles handle code reviews, API implementation, and test execution with cost-conscious routing.

---

## Agents

Define custom agent roles with specific permissions and capabilities for Go development.

### Agent Role: Code Reviewer
- **ID Pattern:** `code-reviewer-*`
- **Model:** claude
- **Capabilities:** `approve-changes, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 4.00
- **Description:** Reviews Go code for quality, architecture, safety, and adherence to standards. Uses Claude for superior architectural insights and complex code analysis.

### Agent Role: API Implementer
- **ID Pattern:** `api-impl-*`
- **Model:** claude
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 4.50
- **Description:** Implements API endpoints and complex business logic. Requires Claude's superior reasoning for intricate Go concurrency patterns and architectural decisions.

### Agent Role: Test Engineer
- **ID Pattern:** `test-engineer-*`
- **Model:** haiku
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** `access-secrets`
- **Cost Budget (USD/hour):** 0.80
- **Description:** Writes unit tests, integration tests, and test scenarios. Cost-optimized for routine test implementation with sufficient capability for Go testing patterns.

### Agent Role: Build & Deploy Lead
- **ID Pattern:** `build-lead-*`
- **Model:** claude
- **Capabilities:** `spawn-workers, read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-all-tasks, escalate`
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 5.00
- **Description:** Leads build and deployment processes, coordinates testing, manages pipeline orchestration. Full reasoning capabilities needed for deployment decisions.

---

## Commands

Register project-specific slash commands for Go development workflows.

### Command: /build
- **Aliases:** `build, compile, make`
- **Handler:** `build-lead-worker`
- **Args:** `[--target=linux|darwin|windows] [--arch=amd64|arm64] [--optimize]`
- **Description:** Build the Go service for specified target platform and architecture
- **Requires Approval:** no
- **Allowed Roles:** `build-lead, coordinator`

### Command: /test
- **Aliases:** `test, run-tests, verify`
- **Handler:** `test-engineer-worker`
- **Args:** `[--suite=unit|integration|all] [--coverage] [--verbose]`
- **Description:** Run test suite with optional coverage analysis and verbose output
- **Requires Approval:** no
- **Allowed Roles:** `test-engineer, coordinator`

### Command: /deploy
- **Aliases:** `deploy, release, push`
- **Handler:** `build-lead-worker`
- **Args:** `<environment> [--dry-run] [--rollback-on-fail]`
- **Description:** Deploy service to staging or production with safety checks
- **Requires Approval:** yes
- **Allowed Roles:** `build-lead, coordinator`

### Command: /review-code
- **Aliases:** `review, code-review, pr-check`
- **Handler:** `code-reviewer-worker`
- **Args:** `<file-path|pr-number> [--strict] [--focus=performance|safety|style]`
- **Description:** Review Go code for quality, potential issues, and adherence to standards
- **Requires Approval:** no
- **Allowed Roles:** `code-reviewer, coordinator`

### Command: /implement-api
- **Aliases:** `implement, add-endpoint, implement-feature`
- **Handler:** `api-impl-worker`
- **Args:** `<endpoint> [--method=GET|POST|PUT|DELETE] [--spec=OPENAPI_FILE]`
- **Description:** Implement API endpoint with full request/response handling and validation
- **Requires Approval:** no
- **Allowed Roles:** `api-impl, coordinator`

---

## Routing

Define intelligent task routing based on complexity, type, and requirements.

### Route: Testing Tasks (Cost-Optimized)
- **When:** `task.type == "test" && task.complexity < 6`
- **Route To:** `haiku`
- **Reason:** Haiku provides excellent cost efficiency for test writing and execution
- **Cost Impact:** `-0.65`

### Route: Complex API Implementation
- **When:** `task.type == "implementation" && (task.complexity >= 7 || task.requires_concurrency == true)`
- **Route To:** `claude`
- **Reason:** Go concurrency patterns and complex logic require Claude's superior reasoning
- **Cost Impact:** `+0.55`

### Route: Code Review (Quality Critical)
- **When:** `task.type == "review" && task.requires_accuracy == true`
- **Route To:** `claude`
- **Reason:** Code review quality directly impacts system reliability; Claude's analysis is essential
- **Cost Impact:** `+0.50`

### Route: Simple Testing
- **When:** `task.type == "test" && task.complexity <= 3`
- **Route To:** `haiku`
- **Reason:** Unit tests with straightforward scenarios are cost-effective with Haiku
- **Cost Impact:** `-0.70`

### Route: High-Priority Deployment
- **When:** `task.type == "deploy" && task.priority == "high"`
- **Route To:** `claude`
- **Reason:** Critical deployments need Claude's careful analysis for safety
- **Cost Impact:** `+0.60`

### Route: Standard Build & Test
- **When:** `task.type == "build" || (task.type == "test" && task.priority == "normal")`
- **Route To:** `haiku`
- **Reason:** Routine build and test tasks are cost-effective with Haiku
- **Cost Impact:** `-0.65`

### Route: API Development (Standard)
- **When:** `task.type == "implementation" && task.complexity < 7`
- **Route To:** `claude`
- **Reason:** API development still benefits from Claude's reasoning even for simpler tasks
- **Cost Impact:** `+0.40`

---

## Context

Specify files to auto-load when spawning agents for this Go project.

### Auto-Load Files
- `./README.md` - Project overview, build instructions, and quick start (weight: high)
- `./go.mod` - Go module dependencies and version information (weight: high)
- `./internal/**/*.go` - All internal package implementations (weight: high)
- `./api/openapi.yaml` - API specification for endpoint development (weight: high)
- `./docs/ARCHITECTURE.md` - System design, component structure, data flow (weight: high)
- `./docs/CODING_STANDARDS.md` - Go style guide and best practices (weight: medium)
- `./Makefile` - Build targets and common commands (weight: medium)
- `./scripts/deploy.sh` - Deployment automation script (weight: medium)
- `.env.example` - Environment variable template (weight: low)
- `./docs/TESTING.md` - Testing strategy and guidelines (weight: medium)

---

## Example Workflows

### Workflow 1: Implement New API Endpoint
1. User runs: `/implement-api /api/v1/users --method=POST --spec=./api/openapi.yaml`
2. Routes to `api-impl-worker` handler
3. Agent loads context: go.mod, internal/**, ARCHITECTURE.md, API spec
4. Agent analyzes OpenAPI spec for requirements
5. Task routed based on complexity → Claude for proper concurrency handling
6. Implementation includes request validation, error handling, database operations
7. Result provided with code ready for review

### Workflow 2: Test Implementation with Review
1. User runs: `/test --suite=integration --coverage`
2. Routes to `test-engineer-worker` handler
3. Agent loads context: README.md, internal/**, TESTING.md
4. Test suite executes with coverage analysis
5. Then user runs: `/review-code internal/handlers/users.go`
6. Routes to `code-reviewer-worker` handler
7. Claude reviews for safety, performance, and style compliance
8. Feedback provided if issues found

### Workflow 3: Deployment Pipeline
1. User runs: `/build --target=linux --arch=amd64 --optimize`
2. Routes to `build-lead-worker` handler
3. Build executes with optimizations for Linux production
4. User runs: `/test --suite=all`
5. Complete test suite executes (routed to Haiku for cost efficiency)
6. User runs: `/deploy production --dry-run`
7. Approval required, then actual deployment executes
8. Build lead monitors and reports success/failure

---

## Configuration Notes

### Model Selection Rationale
- **Claude for reviews & implementation:** Architectural decisions and code quality require superior reasoning
- **Haiku for testing:** Test writing is systematic; Haiku provides excellent cost efficiency
- **Claude for builds/deploys:** Critical decisions need careful analysis

### Cost Optimization Strategy
- Testing tasks route to Haiku by default: ~65% cost savings
- Complex implementation routes to Claude: ~55% cost increase justified by quality
- High-priority tasks use Claude regardless: Safety > Cost

### Capability Distribution
- Only `build-lead` agents can spawn workers (safety)
- Only `code-reviewer` agents can approve (quality gate)
- All agents can access logs and communicate

### Context Priority
1. **High:** go.mod, internal code, API spec, architecture (loaded immediately)
2. **Medium:** Build scripts, coding standards, testing guides (loaded second)
3. **Low:** Environment templates (loaded last)

---

## Validation

Before using this configuration in production:

```bash
# Validate syntax and schema
flip2 validate --config ./go-web-service.FLIP2.md

# Validate specific sections
flip2 validate --config ./go-web-service.FLIP2.md --section agents
flip2 validate --config ./go-web-service.FLIP2.md --section commands
flip2 validate --config ./go-web-service.FLIP2.md --section routing
flip2 validate --config ./go-web-service.FLIP2.md --section context
```

---

## Customization Guide

1. **Add additional internal packages:** Update context to include new `./internal/*/` paths
2. **Add custom agent types:** Define new roles for specific specialized tasks
3. **Adjust cost budgets:** Modify based on actual usage patterns and team priorities
4. **Update deployment targets:** Add new environments to `/deploy` command args
5. **Extend testing patterns:** Add new test suite types as project grows

---

**Status:** Production Ready
**Created for:** CFG-007 - FLIP2 Template Generation
**Template Use:** Copy as `FLIP2.md` to Go web service projects
