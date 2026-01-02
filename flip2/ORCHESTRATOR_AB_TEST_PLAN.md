# Orchestrator A/B Test Plan: Gemini Flash vs Haiku

**Created**: 2026-01-02 01:50 UTC
**Orchestrator**: Autonomous agent (Gemini coordinator preferred)
**Objective**: Balance work between Gemini Flash and Haiku, compare cost/quality results

---

## Test Design

### Parallel Task Assignment

**3 Tasks to Both Models** (total 6 workers):

| Task ID | Description | Complexity | Gemini Flash Worker | Haiku Worker |
|---------|-------------|------------|---------------------|--------------|
| MCP-009 | Implement MCP Sampling support | Medium-High | Worker 7 | Worker 10 |
| RTR-002 | Create task complexity scorer | Medium | Worker 8 | Worker 11 |
| SES-003 | Implement session start/stop | Medium | Worker 9 | Worker 12 |

**Acceptance Criteria** (same for both):
- Code compiles without errors
- Unit tests pass
- Integration with existing code verified
- Documentation included

---

## Worker Spawn Commands

### Gemini Flash Workers

**Worker 7 (MCP-009)**:
```bash
# Try FLIP binary first
cd /Users/arielspivakovsky/src/flip
./flip spawn run worker7 gemini-flash "You are Worker 7 testing Gemini Flash coding quality.

TASK: Implement MCP-009 Sampling Support

CONTEXT:
- Work directory: /Users/arielspivakovsky/src/flip/flip2
- MCP-002 through MCP-008 already complete
- You need to allow MCP servers to request LLM completions from FLIP2's agent pool

REQUIREMENTS:
1. Create sampling handler in internal/mcp/sampling.go
2. Accept completion requests from MCP servers
3. Route requests to appropriate model in agent pool
4. Return completions back to MCP server
5. Write tests in internal/mcp/sampling_test.go

DELIVERABLE: Create WORKER7_SAMPLING_REPORT.md with:
- Implementation summary
- Test results (must pass)
- Integration verification
- Comparison notes vs Haiku

Work autonomously. Report back when complete."
```

**Worker 8 (RTR-002)**:
```bash
./flip spawn run worker8 gemini-flash "You are Worker 8 testing Gemini Flash coding quality.

TASK: Implement RTR-002 Task Complexity Scorer

CONTEXT:
- Work directory: /Users/arielspivakovsky/src/flip/flip2
- Create new package: internal/routing/scorer.go
- This enables cost-optimized task routing

REQUIREMENTS:
1. Algorithm to rate task complexity 1-5
2. Factors: keyword analysis, token count, code vs text
3. Unit tests with 90%+ accuracy vs human ratings
4. Integration with routing engine

DELIVERABLE: Create WORKER8_SCORER_REPORT.md

Work autonomously. Report when complete."
```

**Worker 9 (SES-003)**:
```bash
./flip spawn run worker9 gemini-flash "You are Worker 9 testing Gemini Flash coding quality.

TASK: Implement SES-003 Session Start/Stop

CONTEXT:
- Work directory: /Users/arielspivakovsky/src/flip/flip2
- Session schema already defined (SES-001, SES-002)
- Implement commands: flip2 session start/stop

REQUIREMENTS:
1. Create internal/session/manager.go
2. Start session: create entry in SQLite
3. Stop session: save state, mark inactive
4. CLI commands in cmd/flip2/session.go
5. Tests in internal/session/manager_test.go

DELIVERABLE: Create WORKER9_SESSION_REPORT.md

Work autonomously. Report when complete."
```

### Haiku Workers (Baseline Comparison)

**Worker 10 (MCP-009 - Haiku baseline)**:
```bash
# Use Task tool with model: haiku
# Same prompt as Worker 7, but using Haiku
```

**Worker 11 (RTR-002 - Haiku baseline)**:
```bash
# Same prompt as Worker 8, but using Haiku
```

**Worker 12 (SES-003 - Haiku baseline)**:
```bash
# Same prompt as Worker 9, but using Haiku
```

---

## Orchestrator Instructions

### Step 1: Spawn Workers (Parallel)

**Option A: Use FLIP binary**
```bash
cd /Users/arielspivakovsky/src/flip

# If FLIP binary supports gemini-flash:
./flip spawn run worker7 gemini-flash "<prompt from above>"
./flip spawn run worker8 gemini-flash "<prompt from above>"
./flip spawn run worker9 gemini-flash "<prompt from above>"

# For Haiku comparison:
./flip spawn run worker10 haiku "<same prompt as worker7>"
./flip spawn run worker11 haiku "<same prompt as worker8>"
./flip spawn run worker12 haiku "<same prompt as worker9>"
```

**Option B: Use Task tool (if FLIP binary unavailable)**
```bash
# For Gemini Flash - may need workaround since Task tool doesn't support it
# Try calling with model="haiku" but instructing to use Gemini API directly?

# For Haiku (this works):
Task(model="haiku", prompt="<worker prompt>")
```

**Option C: Ask user for correct spawn method**
If both fail, document in AG_STATUS_UPDATES.md and ask user for help.

### Step 2: Monitor Progress

Check worker outputs every 10 minutes:
```bash
# If using FLIP:
./flip status
./flip task list

# Check for report files:
ls -la /Users/arielspivakovsky/src/flip/flip2/WORKER*_REPORT.md
```

### Step 3: Collect Results

When all 6 workers complete, gather:

**For each worker:**
1. Implementation time (start → finish)
2. Token usage (from agent logs)
3. Code quality (does it compile?)
4. Test pass rate (% tests passing)
5. Iterations needed (how many attempts?)
6. Integration success (works with existing code?)

### Step 4: Compare Results

Create `/Users/arielspivakovsky/src/flip/flip2/GEMINI_VS_HAIKU_RESULTS.md`:

```markdown
# Gemini Flash vs Haiku Comparison Results

## Task 1: MCP-009 Sampling

| Metric | Gemini Flash (Worker 7) | Haiku (Worker 10) | Winner |
|--------|-------------------------|-------------------|--------|
| Time to complete | X min | Y min | ? |
| Token usage | X tokens | Y tokens | ? |
| Cost | $X | $Y | ? |
| Code compiles | ✅/❌ | ✅/❌ | ? |
| Tests pass rate | X% | Y% | ? |
| Iterations needed | X | Y | ? |
| Integration success | ✅/❌ | ✅/❌ | ? |

## Task 2: RTR-002 Scorer
[Same table]

## Task 3: SES-003 Session
[Same table]

## Overall Summary

**Cost Savings**: Gemini Flash is X% cheaper than Haiku
**Quality**: Gemini Flash achieved Y% of Haiku's quality
**Recommendation**: Use Gemini Flash for [task types] / Stick with Haiku for [task types]
```

### Step 5: Report Back

Update `/Users/arielspivakovsky/src/flip/flip2/AG_STATUS_UPDATES.md`:
```markdown
## A/B Test Complete

- 6 workers spawned (3 Gemini Flash, 3 Haiku)
- Results in GEMINI_VS_HAIKU_RESULTS.md
- Recommendation: [Use Gemini Flash / Stick with Haiku / Mixed strategy]
```

---

## Success Criteria

**Minimum acceptable Gemini Flash performance:**
- Code compiles: 100% (same as Haiku)
- Tests pass: ≥80% of Haiku's pass rate
- Integration works: ✅ (critical)
- Cost savings: ≥50% vs Haiku

**Decision Matrix:**

| Gemini Flash Quality | Cost Savings | Decision |
|---------------------|--------------|----------|
| ≥90% of Haiku | ≥50% | ✅ Use Gemini Flash for all coding |
| 80-90% of Haiku | ≥50% | ✅ Use Gemini Flash for simple tasks |
| 70-80% of Haiku | ≥50% | ⚠️ Use Gemini Flash for tests only |
| <70% of Haiku | Any | ❌ Stick with Haiku |

---

## Fallback Plan

If Gemini Flash quality is insufficient (<70% of Haiku):
1. Document findings in GEMINI_VS_HAIKU_RESULTS.md
2. Use Haiku for all coding tasks
3. Reserve Gemini Flash for: data processing, log analysis, bulk text tasks
4. Retest Gemini Flash in 3 months (models improve)

---

## Open Questions for User

1. **How to spawn Gemini Flash workers?** (FLIP binary command? Task tool workaround?)
2. **Quality threshold?** (Is 80% of Haiku acceptable?)
3. **Which tasks to prioritize?** (If Gemini Flash fails, which tasks are most critical?)

---

**Orchestrator**: Read this plan, spawn 6 workers, compare results, report findings.
**File**: `/Users/arielspivakovsky/src/flip/flip2/ORCHESTRATOR_AB_TEST_PLAN.md`
