# FLIP2 Implementation - Antigravity Delegation Instructions

**Date**: January 1, 2026
**Delegated By**: Coordinator Claude
**Assigned To**: Antigravity (Human-in-loop supervisor)
**Priority**: HIGH
**Duration**: 26-28 weeks (6.5-7 months)

---

## Your Mission

You are the **PRIMARY EXECUTION SUPERVISOR** for the FLIP2 implementation. The coordinator (me) will minimize involvement to preserve context. You have full authority to:

1. ✅ Spawn Gemini Flash and Haiku agents
2. ✅ Delegate tasks from the execution plan
3. ✅ Monitor model performance and switch between models
4. ✅ Fix bugs as they arise
5. ✅ Track comprehensive statistics
6. ✅ Optimize prompts for both models

---

## Execution Plan Location

**Primary Reference**: `/Users/arielspivakovsky/src/flip/flip2/IMPLEMENTATION_PLAN_2026.md`
- 137 total tasks broken into 4 phases
- Each task has: ID, description, acceptance criteria, effort, model assignment

**Cost Estimate**: `/Users/arielspivakovsky/src/flip/flip2/REMAINING_WORK_ESTIMATE_2026.md`
- 129 tasks remaining
- $12.54 total budget
- 26-28 weeks timeline

**Metrics Tracking**: `/Users/arielspivakovsky/src/flip/flip2/IMPLEMENTATION_METRICS_2026.md`
- Update this after each task completion

---

## Phase 0: Start Here (Next 3-4 Weeks)

### Immediate Tasks to Delegate

Launch these **3 parallel tasks** this week:

#### Task 1: MCP-002 (Registry Data Structure)
**Assign To**: Gemini Flash (preferred) or Haiku (if Flash struggles)
**Effort**: 4 hours
**Cost**: ~$0.08 (Gemini) or $0.04 (Haiku)

**Prompt Template**:
```
You are implementing MCP-002: Create MCP server registry data structure.

TASK: Design and implement a Go struct to store MCP server metadata.

REQUIREMENTS:
- Store: server ID, name, capabilities, connection status, tools list
- Thread-safe access (use sync.RWMutex)
- Methods: Add, Remove, Update, List, Get
- Unit tests with 100% coverage

ACCEPTANCE CRITERIA:
- Code compiles without errors
- All unit tests pass
- Thread-safe concurrent access verified

DELIVERABLES:
- /Users/arielspivakovsky/src/flip/flip2/internal/mcp/registry.go
- /Users/arielspivakovsky/src/flip/flip2/internal/mcp/registry_test.go

DEPENDENCIES: MCP-001 (already complete at internal/mcp/server.go)

REFERENCE: Read /Users/arielspivakovsky/src/flip/flip2/internal/mcp/server.go for interface

Report back when complete with test results.
```

#### Task 2: MCP-005 (Tool Router Interface)
**Assign To**: Gemini Flash
**Effort**: 4 hours
**Cost**: ~$0.08

**Prompt Template**:
```
You are implementing MCP-005: Design Tool Router interface.

TASK: Create Go interface for capability-based tool routing.

REQUIREMENTS:
- Interface for matching task requirements to MCP tool capabilities
- Methods: RegisterTool, FindToolByCapability, RouteTask
- Support for multiple capability types: file-ops, browser, database, git
- Well-documented with examples

ACCEPTANCE CRITERIA:
- Interface compiles and is documented
- Example implementations provided
- Integration tests with mock MCP tools

DELIVERABLES:
- /Users/arielspivakovsky/src/flip/flip2/internal/mcp/router.go
- /Users/arielspivakovsky/src/flip/flip2/internal/mcp/router_test.go

DEPENDENCIES: MCP-001 (complete)

Report back when complete.
```

#### Task 3: CTX-002 (Fix process.go Context Leaks)
**Assign To**: Haiku (simpler, surgical task)
**Effort**: 3 hours
**Cost**: ~$0.03

**Prompt Template**:
```
You are implementing CTX-002: Fix process.go context leaks.

TASK: Add defer cancel() to all context.With* calls in process.go.

REQUIREMENTS:
- Read /Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go
- Find all context.WithTimeout, context.WithCancel calls
- Add "defer cancel()" immediately after each one
- Verify no goroutine leaks

ACCEPTANCE CRITERIA:
- All contexts have defer cancel()
- Code compiles without errors
- No goroutine count increases during tests

DELIVERABLES:
- Modified /Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go
- Verification via go test -race

DEPENDENCIES: CTX-001 (audit complete, report at internal/reports/context-audit.md)

Report back when complete.
```

---

## Model Selection Strategy

### Prefer Gemini Flash For:
- ✅ New package creation (like httpclient today)
- ✅ Large implementations (100+ lines)
- ✅ Test suite generation
- ✅ Complex data structures
- ✅ Integration work
- ✅ **Cost optimization** (3.3x cheaper than Haiku)

### Use Haiku For:
- ✅ Targeted bugfixes (< 20 line changes)
- ✅ Surgical refactoring
- ✅ Simple test additions
- ✅ Documentation tasks
- ✅ **When Gemini Flash struggles** (backup)

### Switch from Gemini to Haiku If:
1. Gemini takes > 3 iterations to fix compilation errors
2. Gemini produces failing tests after 2 attempts
3. Task is simpler than expected (< 50 lines)
4. Gemini's output quality drops below 80%

---

## Performance Tracking

### Create This Tracking File

**Location**: `/Users/arielspivakovsky/src/flip/flip2/MODEL_PERFORMANCE_STATS.md`

**Track for Each Task**:
```markdown
### TASK-ID: Description
- **Model Used**: Gemini Flash / Haiku
- **Estimated Effort**: Xh
- **Actual Effort**: Yh
- **Iterations**: N (how many attempts to get working code)
- **Lines of Code**: XXX
- **Test Coverage**: XX%
- **Compilation**: Pass/Fail on first try
- **Tests Pass**: Pass/Fail on first try
- **Code Quality**: 1-10 rating
- **Cost**: $X.XX
- **Decision**: Keep model / Switch to other model
- **Notes**: Any issues or observations
```

### Weekly Summary Stats

Every Friday, generate:
```markdown
## Week of YYYY-MM-DD

| Metric | Gemini Flash | Haiku |
|--------|--------------|-------|
| Tasks Completed | X | Y |
| Success Rate (first try) | XX% | YY% |
| Avg Iterations | N | M |
| Total Cost | $X.XX | $Y.YY |
| Cost per Task | $X.XX | $Y.YY |
| Lines of Code | XXX | YYY |
| Test Coverage | XX% | YY% |
| Quality Rating | X/10 | Y/10 |
```

---

## Bug Monitoring & Fixing

### Continuous Monitoring

**Check every 2 hours**:
```bash
cd /Users/arielspivakovsky/src/flip/flip2

# Check if code compiles
go build ./...

# Run all tests
go test ./...

# Check for race conditions
go test -race ./...

# Look for agent error reports
./flip task list --status failed
```

### Bug Priority System

**P0 Critical** (fix within 1 hour):
- System won't compile
- All tests failing
- Daemon crashes on startup
- Data corruption

**P1 High** (fix within 4 hours):
- Single package won't compile
- Test suite fails for one package
- Memory/goroutine leaks
- API endpoints broken

**P2 Medium** (fix within 1 day):
- Individual test failures
- Code style violations
- Missing error handling
- Documentation gaps

**P3 Low** (fix when convenient):
- Code cleanup opportunities
- Performance optimizations
- Nice-to-have features

### Bug Fixing Process

1. **Detect**: Automated testing or agent reports
2. **Triage**: Assign priority P0-P3
3. **Assign**:
   - P0/P1: Haiku (fast, surgical fixes)
   - P2/P3: Gemini Flash (if part of larger work)
4. **Verify**: Run tests after fix
5. **Document**: Update IMPLEMENTATION_METRICS_2026.md

---

## Prompt Optimization Guidelines

### For Gemini Flash

**DO**:
- ✅ Provide complete context (read relevant files first)
- ✅ Specify exact file paths for deliverables
- ✅ Include acceptance criteria as checklist
- ✅ Request test coverage explicitly
- ✅ Ask for working code on first try
- ✅ Provide examples from similar completed work

**DON'T**:
- ❌ Give vague requirements
- ❌ Assume it knows project structure
- ❌ Skip acceptance criteria
- ❌ Forget to mention testing requirements
- ❌ Let it iterate more than 3 times

**Template Structure**:
```
You are implementing TASK-ID: [name].

TASK: [One sentence description]

REQUIREMENTS: [Bulleted list, 4-6 items]

ACCEPTANCE CRITERIA: [Checklist format]

DELIVERABLES: [Exact file paths]

DEPENDENCIES: [What's already done, where to find it]

REFERENCE: [Point to similar code to learn from]

Report back when complete with test results.
```

### For Haiku

**DO**:
- ✅ Be surgical and specific
- ✅ Point to exact file and line numbers
- ✅ Provide before/after examples
- ✅ Keep scope minimal
- ✅ Emphasize "minimal changes"

**DON'T**:
- ❌ Give large, open-ended tasks
- ❌ Ask for architectural decisions
- ❌ Request new package creation
- ❌ Expect comprehensive test suites

**Template Structure**:
```
You are fixing [specific issue] in [file].

PROBLEM: [What's broken, exact line numbers]

SOLUTION: [Specific change needed]

REQUIREMENTS:
- Change only [specific function/lines]
- Preserve existing behavior
- Add defer cancel() / fix error / etc.

DELIVERABLES: Modified [file path]

Report back when complete.
```

---

## Coordinator Escalation

**When to Escalate to Coordinator Claude**:

1. **Architectural Decisions**:
   - Major design choices (use Opus for these)
   - Breaking changes to existing APIs
   - Security-sensitive implementations

2. **Blocked Progress**:
   - Both Gemini and Haiku failing on same task
   - Unclear requirements in plan
   - Conflicting acceptance criteria

3. **Weekly Checkpoint**:
   - Every Friday: Send summary stats
   - Phase completion milestones
   - Go/No-Go decisions

**Escalation Format**:
```bash
# For architectural questions
./flip signal send coordinator "ARCHITECTURE DECISION NEEDED: [topic]. Context: [1-2 sentences]. Options: [A/B/C]."

# For blocked progress
./flip signal send coordinator "BLOCKED: Task [ID] failed after 3 attempts. Gemini: [issue]. Haiku: [issue]. Need guidance."

# For weekly summary
./flip signal send coordinator "WEEK [N] COMPLETE: [X] tasks done, $[Y] spent, [Z] bugs fixed. Stats: [link to file]."
```

---

## Weekly Execution Cadence

### Monday
- Review previous week's completions
- Identify Phase 0 tasks for this week
- Launch 3-6 parallel agents
- Set up monitoring

### Tuesday-Thursday
- Monitor agent progress
- Fix bugs as they arise
- Track performance stats
- Optimize prompts based on results

### Friday
- Generate weekly stats report
- Update IMPLEMENTATION_METRICS_2026.md
- Send summary to coordinator
- Plan next week's tasks

---

## Success Metrics

### Week 1 Target (January 8, 2026)
- ✅ 3 tasks completed: MCP-002, MCP-005, CTX-002
- ✅ All tests passing
- ✅ Zero P0/P1 bugs
- ✅ Model stats initialized

### Week 4 Target (January 29, 2026) - Phase 0 Checkpoint
- ✅ 16 MCP tasks completed
- ✅ 3+ MCP servers connected
- ✅ Tool discovery working
- ✅ MCP sampling functional
- ✅ Cost < $5.00

### Week 16 Target (April 23, 2026) - Phase 1 Checkpoint
- ✅ 78 tasks completed
- ✅ All P1 features operational
- ✅ Cost < $16.00
- ✅ Gemini Flash vs Haiku decision finalized

---

## File Locations Reference

**Plans & Tracking**:
- `/Users/arielspivakovsky/src/flip/flip2/IMPLEMENTATION_PLAN_2026.md` - Detailed task list
- `/Users/arielspivakovsky/src/flip/flip2/REMAINING_WORK_ESTIMATE_2026.md` - Budget & timeline
- `/Users/arielspivakovsky/src/flip/flip2/IMPLEMENTATION_METRICS_2026.md` - Progress tracking
- `/Users/arielspivakovsky/src/flip/flip2/MODEL_PERFORMANCE_STATS.md` - **YOU CREATE THIS**

**Code Quality**:
- `/Users/arielspivakovsky/src/flip/flip2/CODE_QUALITY_REVIEW_2026.md` - Best practices
- `/Users/arielspivakovsky/src/flip/flip2/REVIEW_SUMMARY.txt` - Quality guidelines

**Comparison Data**:
- `/Users/arielspivakovsky/src/flip/flip2/GEMINI_VS_HAIKU_COMPARISON.md` - Today's test results

**Codebase**:
- `/Users/arielspivakovsky/src/flip/flip2/` - Main project root
- Binary: `/Users/arielspivakovsky/src/flip/flip2/flip2`
- Daemon binary: `/Users/arielspivakovsky/src/flip/flip2/flip2d`

---

## Important Reminders

1. **Favor Gemini Flash**: It's 3.3x cheaper and handles large implementations well
2. **Switch to Haiku if**: Gemini struggles after 2-3 iterations
3. **Track everything**: Stats inform future model selection
4. **Fix bugs immediately**: Don't let them pile up
5. **Optimize prompts**: Learn what works for each model
6. **Escalate blockers**: Don't waste time on stuck tasks
7. **Weekly reports**: Keep coordinator informed without overloading context

---

## Your First Actions (Next 24 Hours)

1. ✅ **Create** `/Users/arielspivakovsky/src/flip/flip2/MODEL_PERFORMANCE_STATS.md`
2. ✅ **Launch 3 agents** using prompts above:
   - MCP-002: Gemini Flash
   - MCP-005: Gemini Flash
   - CTX-002: Haiku
3. ✅ **Monitor** progress every 2 hours
4. ✅ **Fix bugs** as they arise
5. ✅ **Track stats** in the performance file
6. ✅ **Report** when all 3 tasks complete

---

## Questions?

Signal coordinator with: `./flip signal send coordinator "QUESTION: [your question]"`

---

**AUTHORITY DELEGATED**: You are authorized to spawn agents, spend budget, and make execution decisions. Coordinator will only intervene for escalations.

**STATUS**: ACTIVE - Proceed immediately

**GOOD LUCK!** 🚀

---

**Delegation Issued**: January 1, 2026, 23:55 EST
**Coordinator**: Claude Sonnet 4.5 (Main Session)
**Supervisor**: Antigravity (Human-in-loop)
**Budget**: $12.54 remaining
**Timeline**: 26-28 weeks
