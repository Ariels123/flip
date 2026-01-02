# FLIP2.md - React Frontend Configuration

**Project:** React Frontend
**Version:** 1.0
**Coordinator:** claude-coordinator
**Last Updated:** 2026-01-01

---

## Overview

This FLIP2.md configuration optimizes agent routing for React/TypeScript frontend development. Specialized roles handle UI component development, styling, testing, and code review with cost-conscious routing between Haiku (styling/UI components) and Sonnet (complex interactions).

---

## Agents

Define custom agent roles for frontend development.

### Agent Role: Frontend Developer
- **ID Pattern:** `frontend-dev-*`
- **Model:** sonnet
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 2.75
- **Description:** Develops React components, handles state management, implements features. Sonnet provides excellent balance of capability and cost for complex component logic.

### Agent Role: UI Reviewer
- **ID Pattern:** `ui-reviewer-*`
- **Model:** sonnet
- **Capabilities:** `approve-changes, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 3
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 2.50
- **Description:** Reviews component implementations, validates accessibility, checks performance. Sonnet's balance of capability and cost suitable for thorough code review.

### Agent Role: Styling Specialist
- **ID Pattern:** `styling-*`
- **Model:** haiku
- **Capabilities:** `read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 0.75
- **Description:** Implements CSS/styling, theme customizations, responsive design. Haiku provides exceptional cost efficiency for systematic styling tasks.

### Agent Role: Test Engineer
- **ID Pattern:** `test-engineer-*`
- **Model:** haiku
- **Capabilities:** `external-api-calls, read-logs`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-own-tasks`
- **Max Concurrent Tasks:** 5
- **Escalation Required For:** `access-secrets`
- **Cost Budget (USD/hour):** 0.70
- **Description:** Writes unit tests, integration tests, e2e tests for React components. Haiku excels at systematic test writing with excellent cost efficiency.

### Agent Role: Design Systems Lead
- **ID Pattern:** `design-lead-*`
- **Model:** sonnet
- **Capabilities:** `spawn-workers, read-logs, external-api-calls`
- **Permissions:** `read-inbox, send-signals, create-tasks, modify-all-tasks, escalate`
- **Max Concurrent Tasks:** 2
- **Escalation Required For:** `execute-destructive`
- **Cost Budget (USD/hour):** 3.00
- **Description:** Leads design system development, coordinates component library, manages design decisions. Sonnet's reasoning needed for architectural decisions.

---

## Commands

Register project-specific slash commands for React development workflows.

### Command: /dev
- **Aliases:** `dev, start, development`
- **Handler:** `design-lead-worker`
- **Args:** `[--port=PORT] [--open-browser]`
- **Description:** Start development server with hot reload and debugging
- **Requires Approval:** no
- **Allowed Roles:** `design-lead, frontend-dev, coordinator`

### Command: /build
- **Aliases:** `build, compile, production-build`
- **Handler:** `design-lead-worker`
- **Args:** `[--optimize] [--source-maps] [--analyze]`
- **Description:** Build optimized production bundle with optional analysis
- **Requires Approval:** no
- **Allowed Roles:** `design-lead, coordinator`

### Command: /test
- **Aliases:** `test, run-tests, verify`
- **Handler:** `test-engineer-worker`
- **Args:** `[--suite=unit|integration|e2e|all] [--coverage] [--watch]`
- **Description:** Run test suite with optional coverage report and watch mode
- **Requires Approval:** no
- **Allowed Roles:** `test-engineer, coordinator`

### Command: /create-component
- **Aliases:** `new-component, add-component, component`
- **Handler:** `frontend-dev-worker`
- **Args:** `<component-name> [--type=functional|class|form] [--with-tests]`
- **Description:** Create new React component with optional test file
- **Requires Approval:** no
- **Allowed Roles:** `frontend-dev, coordinator`

### Command: /style
- **Aliases:** `style, add-styles, styling`
- **Handler:** `styling-worker`
- **Args:** `<component-name> [--theme=light|dark|custom] [--responsive]`
- **Description:** Add styling to component with theme support and responsiveness
- **Requires Approval:** no
- **Allowed Roles:** `styling, frontend-dev, coordinator`

### Command: /review
- **Aliases:** `review, code-review, pr-check`
- **Handler:** `ui-reviewer-worker`
- **Args:** `<file-path|pr-number> [--focus=performance|a11y|style|all]`
- **Description:** Review component code for quality, accessibility, and performance
- **Requires Approval:** no
- **Allowed Roles:** `ui-reviewer, coordinator`

---

## Routing

Define intelligent routing based on task type, complexity, and cost optimization.

### Route: UI Component Development (Standard)
- **When:** `task.type == "component" && task.complexity < 7`
- **Route To:** `sonnet`
- **Reason:** Component development needs Sonnet's superior reasoning for state management and logic
- **Cost Impact:** `-0.05`

### Route: Complex Interactive Components
- **When:** `task.type == "component" && task.complexity >= 7 && task.requires_state_management == true`
- **Route To:** `sonnet`
- **Reason:** Complex interactions, advanced state patterns, and data flow require Sonnet's reasoning
- **Cost Impact:** `+0.15`

### Route: CSS & Styling (Cost-Optimized)
- **When:** `task.type == "styling" || task.type == "theme" || task.type == "responsive"`
- **Route To:** `haiku`
- **Reason:** CSS implementation is systematic; Haiku provides exceptional cost efficiency for styling
- **Cost Impact:** `-0.75`

### Route: Component Testing (Cost-Optimized)
- **When:** `task.type == "testing" && task.complexity < 6`
- **Route To:** `haiku`
- **Reason:** Unit tests and integration tests are systematic; Haiku excellent at test writing
- **Cost Impact:** `-0.70`

### Route: Complex Test Scenarios
- **When:** `task.type == "testing" && (task.complexity >= 7 || task.requires_e2e == true)`
- **Route To:** `sonnet`
- **Reason:** Complex e2e scenarios and advanced testing logic need Sonnet's reasoning
- **Cost Impact:** `+0.10`

### Route: Code Review (Quality Critical)
- **When:** `task.type == "review" && task.requires_accuracy == true`
- **Route To:** `sonnet`
- **Reason:** Component review needs superior analysis for architecture and patterns
- **Cost Impact:** `+0.05`

### Route: Rapid Styling Tasks
- **When:** `task.type == "styling" && task.priority == "high" && task.deadline < 60`
- **Route To:** `haiku`
- **Reason:** Fast turnaround on styling tasks; Haiku delivers quickly and cost-effectively
- **Cost Impact:** `-0.75`

### Route: State Management Development
- **When:** `task.type == "state-management" || task.requires_redux == true || task.requires_context == true`
- **Route To:** `sonnet`
- **Reason:** State management architecture needs Sonnet's superior reasoning
- **Cost Impact:** `+0.20`

---

## Context

Specify files to auto-load when spawning agents for this React project.

### Auto-Load Files
- `./README.md` - Project overview, setup, and quick start (weight: high)
- `./package.json` - Dependencies, scripts, and project metadata (weight: high)
- `./src/**/*.tsx` - All React component implementations (weight: high)
- `./tsconfig.json` - TypeScript configuration (weight: high)
- `./src/styles/index.css` - Global styles and theme variables (weight: high)
- `./docs/ARCHITECTURE.md` - Component hierarchy and data flow (weight: high)
- `./docs/COMPONENT_LIBRARY.md` - Reusable components and patterns (weight: medium)
- `./src/hooks/*.ts` - Custom React hooks (weight: medium)
- `./docs/CODING_STANDARDS.md` - Code style and best practices (weight: medium)
- `./src/types/*.ts` - TypeScript type definitions (weight: medium)
- `./docs/TESTING.md` - Testing strategy and guidelines (weight: medium)
- `.env.example` - Environment variable template (weight: low)
- `./docs/ACCESSIBILITY.md` - a11y guidelines and requirements (weight: medium)

---

## Example Workflows

### Workflow 1: Create and Style New Component
1. User runs: `/create-component UserProfile --type=functional --with-tests`
2. Routes to `frontend-dev-worker` → Sonnet for component logic
3. Agent loads context: package.json, src/**/*.tsx, COMPONENT_LIBRARY.md, ARCHITECTURE.md
4. Component created with proper hooks, types, and test scaffold
5. User runs: `/style UserProfile --theme=light --responsive`
6. Routes to `styling-worker` → Haiku for CSS implementation
7. Haiku adds responsive styles, theme support, accessibility improvements
8. Cost optimized: Styling at 75% discount vs complex component work
9. User runs: `/test --suite=unit --coverage`
10. Tests execute via Haiku test engineer (70% cost savings on testing)

### Workflow 2: Feature Implementation with Review
1. User runs: `/create-component ProductCard --type=functional`
2. Routes to `frontend-dev-worker` → Sonnet
3. Implements product card with price display, ratings, purchase button
4. Component integrates with state management (Redux/Context)
5. User runs: `/review src/components/ProductCard.tsx --focus=a11y`
6. Routes to `ui-reviewer-worker` → Sonnet for quality review
7. Reviews for accessibility (ARIA labels, keyboard navigation)
8. Reviews for performance (memo, lazy loading)
9. Reviews for code quality and patterns
10. Provides feedback if improvements needed

### Workflow 3: Complete Feature Development Pipeline
1. User runs: `/create-component ShoppingCart --type=functional`
2. Sonnet develops component with state management logic
3. User runs: `/style ShoppingCart --theme=light --responsive`
4. Haiku applies styling, responsive design, theme support
5. User runs: `/test --suite=unit --coverage`
6. Haiku writes unit tests for component logic
7. User runs: `/test --suite=e2e`
8. Routes to Sonnet for complex e2e scenarios (+10% cost)
9. Tests verify user interaction flows end-to-end
10. User runs: `/build --optimize --analyze`
11. Design lead builds optimized production bundle
12. Bundle analysis helps identify optimization opportunities

### Workflow 4: Design System Maintenance
1. User runs: `/create-component Button --type=functional`
2. Frontend dev creates base Button component
3. User runs: `/style Button --theme=light`
4. Styling specialist adds theme variants, responsive sizes
5. User runs: `/test --suite=all`
6. Test engineer creates comprehensive test coverage
7. User runs: `/review src/components/Button.tsx --focus=all`
8. UI reviewer checks for: performance, accessibility, code quality
9. Design lead coordinates and manages design system updates
10. Cost controlled: Haiku handles styling and testing, Sonnet handles logic

---

## Configuration Notes

### Model Selection Strategy
- **Sonnet for components:** Superior reasoning for React patterns, hooks, state management
- **Haiku for styling:** Systematic CSS implementation at exceptional cost efficiency
- **Haiku for testing:** Test writing is structured; Haiku provides ~70% cost savings
- **Sonnet for reviews:** Code review quality essential for maintainability

### Cost Optimization Strategy
- Styling tasks route to Haiku: ~75% cost savings
- Unit testing routes to Haiku: ~70% cost savings
- Component development uses Sonnet: Only ~5% cost premium for better reasoning
- Complex testing routes to Sonnet: ~10% cost premium for advanced scenarios

### Haiku's Frontend Strengths
- CSS/SCSS implementation and theme management
- Systematic unit test writing
- Responsive design implementation
- Accessibility fixes and adjustments

### Sonnet's Frontend Strengths
- Component architecture and design
- Complex state management (Redux, Context patterns)
- Performance optimization decisions
- Code quality review and architectural insights

### Capability Restrictions
- Only `design-lead` can spawn workers (coordination)
- Only `ui-reviewer` can approve changes
- Test engineer handles all testing routines

### Context Priority
1. **High:** package.json, src/**/*.tsx, styles, component library (loaded first)
2. **Medium:** Architecture, coding standards, hooks, types (loaded second)
3. **Low:** Environment templates, testing guides (loaded last)

---

## Validation

Before using this configuration in production:

```bash
# Validate syntax and schema
flip2 validate --config ./react-frontend.FLIP2.md

# Validate specific sections
flip2 validate --config ./react-frontend.FLIP2.md --section agents
flip2 validate --config ./react-frontend.FLIP2.md --section commands
flip2 validate --config ./react-frontend.FLIP2.md --section routing
flip2 validate --config ./react-frontend.FLIP2.md --section context
```

---

## Customization Guide

1. **Add new component types:** Update `/create-component` with additional templates
2. **Custom styling themes:** Extend `/style` command with project-specific themes
3. **Framework-specific hooks:** Add custom React hooks to context
4. **State management:** Update routing based on Redux/Zustand/other choices
5. **Testing framework:** Adjust routing if using Vitest, Playwright, Cypress

---

**Status:** Production Ready
**Created for:** CFG-007 - FLIP2 Template Generation
**Template Use:** Copy as `FLIP2.md` to React/TypeScript frontend projects
