# Periodic Code Review Workflow

## Overview

Multi-agent code review system using Claude Opus, Codex, and Antigravity for comprehensive quality assurance after major code changes.

## When to Trigger Reviews

Execute periodic reviews after:
- Major feature implementations (e.g., Vibe Scorecard, Feedback Loop)
- Architectural changes affecting multiple components
- Security-sensitive modifications
- Performance optimization work
- Database schema changes or migrations
- API endpoint additions or modifications

## Review Team Composition

### 1. Claude Opus (Thinking Mode) - Architecture Review
**Focus**: High-level architecture, design patterns, scalability

**Responsibilities**:
- Evaluate architectural decisions and trade-offs
- Identify potential scalability bottlenecks
- Assess integration patterns and coupling
- Review data models and database design
- Provide strategic recommendations

**Invocation**: Create task with assignee='opus' and detailed context

### 2. Codex - Code Quality Review
**Focus**: Code-level quality, best practices, implementation details

**Responsibilities**:
- Check code quality and adherence to best practices
- Identify bugs, anti-patterns, or code smells
- Evaluate error handling completeness
- Assess security vulnerabilities
- Review test coverage and documentation
- Suggest refactoring opportunities

**Invocation**: Create task with assignee='codex' and file list

### 3. Antigravity (AG) - End-to-End Browser Testing
**Focus**: User-facing functionality, browser-based testing

**Responsibilities**:
- Test dashboard and UI components
- Verify API endpoints via browser/Postman
- Check for console errors and performance issues
- Test error handling and edge cases
- Provide UX feedback and improvement suggestions
- Screenshot evidence of bugs or issues

**Invocation**: Create task with assignee='antigravity' and test scenarios

## Review Workflow

### Step 1: Trigger Condition Met
After completing major changes, initiate review cycle:

```sql
-- Create Opus architecture review task
INSERT INTO tasks (task_id, title, description, assignee, status, priority)
VALUES (
  'opus_review_<feature>_001',
  'Architecture Review: <Feature Name>',
  '<Detailed review request with context, files changed, specific concerns>',
  'opus',
  'pending',
  10
);

-- Create Codex code quality review task
INSERT INTO tasks (task_id, title, description, assignee, status, priority)
VALUES (
  'codex_review_<feature>_001',
  'Code Quality Review: <Feature Name>',
  '<Code review request with focus areas, files to review>',
  'codex',
  'pending',
  10
);

-- Create AG browser testing task
INSERT INTO tasks (task_id, title, description, assignee, status, priority)
VALUES (
  'ag_test_<feature>_001',
  'End-to-End Testing: <Feature Name>',
  '<Test scenarios, API endpoints, UI elements to verify>',
  'antigravity',
  'pending',
  10
);
```

### Step 2: Monitor Reviews
Check task status to see when reviews complete:

```sql
SELECT task_id, title, assignee, status, progress
FROM tasks
WHERE task_id LIKE '%_review_%' OR task_id LIKE '%_test_%'
ORDER BY created DESC;
```

### Step 3: Review Findings
Read review results from task results:

```sql
SELECT task_id, assignee, result, completed_at
FROM tasks
WHERE status = 'completed'
  AND task_id IN ('opus_review_<feature>_001', 'codex_review_<feature>_001', 'ag_test_<feature>_001');
```

### Step 4: Address Issues
- Prioritize critical issues flagged by any reviewer
- Create follow-up tasks for bugs or improvements
- Update architecture/code based on recommendations
- Re-test via AG if significant changes made

## Example: Vibe Scorecard Review (2026-01-01)

### Changes Made
- `internal/daemon/daemon.go`: +157 lines (evaluator initialization, API endpoint)
- `internal/vibescore/evaluator.go`: New file (LLM-as-Judge evaluation logic)
- `internal/vibescore/types.go`: New file (data structures, status constants)
- `pb_migrations/12_add_vibescore_collection.go`: New file (database schema)

### Review Tasks Created
1. **Opus**: Architecture review of evaluator service design, LLM backend wrapper pattern, cost tracking integration
2. **Codex**: Code quality review of error handling, JSON parsing, database persistence
3. **AG**: Browser testing of `/api/vibescore/evaluate` endpoint, dashboard integration

### Review Focus Areas
- **Architecture**: Is the vibescore.LLMBackend interface appropriate? Should evaluator be a service or singleton?
- **Code Quality**: Is JSON extraction robust? Are all error cases handled? Is cost tracking working?
- **Testing**: Does the API endpoint work correctly? Are scorecards persisted? Any console errors?

## Metrics

Track review effectiveness:
- Number of bugs caught before production
- Review turnaround time per agent
- Issue severity distribution
- Implementation quality trends over time

## Continuous Improvement

After each review cycle:
- Document recurring issues to prevent in future
- Update coding guidelines based on findings
- Improve review prompts for better signal
- Adjust review frequency based on change velocity

---

**Last Review**: 2026-01-01 (Vibe Scorecard Integration)
**Next Review**: After Feedback Loop + Auto-Retry implementation
