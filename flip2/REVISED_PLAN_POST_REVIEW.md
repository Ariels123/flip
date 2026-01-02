# FLIP2 Revised Plan (Post Opus/Gemini Review)

**Date**: 2026-01-02 01:45 UTC
**Based On**: Reviews from Opus (a207e10) and Gemini (ab5e9cd)

---

## Key Changes from Original Plan

### ✅ What Both Reviewers Agreed On

1. **MCP Layer Is Not Redundant** - It's orchestration above agent-native MCP, not duplication
2. **Real Value = Multi-Agent Orchestration + Cost Routing** - Not MCP reimplementation
3. **Phase 0 Overengineered** - Simplify to focus on orchestration, not protocol details
4. **2% Completion Risk** - Must verify existing code before building more
5. **Phase 3 Is Premature** - Skip TUI/dashboard until P0+P1 proven

---

## Revised Phase 0 (Simplified)

**Goal**: Prove multi-agent orchestration works end-to-end with cost optimization

### Core Tasks (Reduced from 16 to 8):

| Task | What It Actually Does | Keep/Change |
|------|----------------------|-------------|
| MCP-002 | Registry: Track which agent has which MCP servers | ✅ KEEP (already done) |
| MCP-003 | CRUD: Add/remove/list servers per agent | ✅ KEEP (already done) |
| MCP-004 | Persistence: Save registry to SQLite | ✅ KEEP (already done) |
| MCP-006 | Discovery: Query tools from all agents' servers | ✅ KEEP (already done) |
| MCP-007 | Matching: Find best tool across all agents | ✅ KEEP (already done) |
| MCP-008 | Invocation: Call tools on any agent | ✅ KEEP (already done) |
| MCP-009 | Sampling: MCP servers can request LLM completions | ✅ KEEP - HIGH VALUE |
| **NEW** | **E2E Test: Verify with real MCP server** | ✅ ADD - CRITICAL GAP |

**Remove from Phase 0:**
- MCP-010 through MCP-016 (resource subscriptions, templates, etc.)
- Move to Phase 2 as optimizations

---

## New Priority: Cost Routing + Agent Spawning

### What We Need to Prove (Phase 0)

1. **Spawn agents with different models** (Gemini Flash, Haiku, Sonnet)
2. **Route tasks by complexity** → Gemini Flash for simple, Sonnet for complex
3. **Agents report back** to coordinator via SQLite/files
4. **Session persists** across coordinator disconnect
5. **Real cost savings** measured and reported

### A/B Test: Gemini Flash vs Haiku

**Objective**: Test if Gemini Flash can handle coding tasks at acceptable quality

| Metric | Gemini Flash | Haiku | Winner |
|--------|--------------|-------|--------|
| Cost per task | ~$0.02 | ~$0.25 | TBD |
| Code quality | TBD | TBD | TBD |
| Test pass rate | TBD | TBD | TBD |
| Iterations needed | TBD | TBD | TBD |

**Test Plan**:
- Spawn 3 Gemini Flash workers for MCP-009, MCP-010, MCP-011
- Spawn 3 Haiku workers for same tasks (parallel)
- Compare: cost, quality, iterations, test pass rate
- Document findings in GEMINI_VS_HAIKU_RESULTS.md

---

## Immediate Next Steps (Preserve Claude Context)

### 1. Verify Current Completion (2-4 hours)
- Run full test suite: `cd flip2 && go test ./... -v`
- Document what actually passes vs fails
- Update BASELINE_METRICS_2026.md with honest status
- **Assign to**: Gemini Flash worker (cheap, can process test output)

### 2. End-to-End MCP Integration Test (4-6 hours)
- Connect FLIP2 to real Anthropic MCP server
- Verify tool discovery works
- Verify tool invocation works
- Verify sampling requests work
- **Assign to**: Sonnet worker (needs reliability)

### 3. Implement MCP-009 Sampling (6-8 hours)
- Allow MCP servers to request LLM completions from FLIP2's agent pool
- Route completion requests to appropriate model
- **Assign to**: Gemini Flash worker (test coding quality)

### 4. Cost Routing Engine (4-6 hours)
- Implement RTR-001 through RTR-004 (task classification, routing rules)
- Simple YAML-based rules
- **Assign to**: Gemini Flash worker (test vs Haiku)

### 5. Session Persistence Core (6-8 hours)
- Implement SES-001 through SES-005 (session start/stop, attach)
- SQLite backend
- **Assign to**: Haiku worker (for comparison)

---

## How to Preserve Claude Context

### File-Based Coordination (Already Set Up)
- **Commands**: `/Users/arielspivakovsky/src/flip/flip2/COORDINATOR_TO_AG_COMMANDS.md`
- **Status**: `/Users/arielspivakovsky/src/flip/flip2/AG_STATUS_UPDATES.md`
- **Worker logs**: `/Users/arielspivakovsky/src/flip/flip2/WORKER_ACTIVITY_LOG.md`

### Spawn Gemini Flash Coordinator
When Claude usage runs low:
1. Update COORDINATOR_TO_AG_COMMANDS.md with next batch
2. Gemini Flash coordinator reads commands
3. Spawns workers autonomously
4. Reports back via AG_STATUS_UPDATES.md
5. Claude can resume when usage resets

### Worker Assignment Strategy
- **Gemini Flash**: Testing quality + cost savings (MCP-009, RTR tasks)
- **Haiku**: Baseline comparison (SES tasks)
- **Sonnet**: Critical path only (E2E integration test)
- **Opus**: Architecture decisions only (none needed right now)

---

## Success Metrics (End of Week 1)

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Phase 0 actual completion | 95%+ | All tests pass |
| E2E MCP test | Pass | Real server connected, tools callable |
| Gemini Flash quality | 80%+ vs Haiku | Code review, test pass rate |
| Cost savings | 50%+ | Gemini Flash vs Haiku comparison |
| Session persistence | Works | Disconnect/reconnect test |

---

## What We're NOT Doing (Per Reviews)

❌ Phase 3 (TUI, dashboard, status line)
❌ Resource subscriptions (MCP-010)
❌ Resource templates (MCP-011)
❌ Prompt templates (MCP-012)
❌ Middleware/Interceptor patterns
❌ Computer Use Agent

**Rationale**: Prove core orchestration + cost routing works first

---

## Risk Mitigation

### Risk 1: Gemini Flash Can't Code Well
- **Mitigation**: A/B test with Haiku baseline, fall back if quality drops
- **Threshold**: If test pass rate < 80% of Haiku, revert to Haiku

### Risk 2: Integration Testing Reveals Broken Code
- **Mitigation**: Freeze features, fix integration before proceeding
- **Decision point**: If >20% of tests fail, stop and stabilize

### Risk 3: Claude Usage Runs Out Before Handoff
- **Mitigation**: File-based coordination already set up, Gemini can continue
- **Handoff file**: COORDINATOR_TO_AG_COMMANDS.md

---

## Updated Timeline

**Week 1** (This week):
- Day 1-2: Verify completion, E2E test (Sonnet worker)
- Day 3-4: MCP-009 Sampling (Gemini Flash worker) + Cost routing (Gemini Flash)
- Day 5: Session persistence (Haiku worker)
- Day 6-7: Compare results, document findings

**Week 2**:
- Start Phase 1 (only if Phase 0 = 95%+ verified)

---

## Open Questions for User

1. **How to spawn Gemini Flash workers?** (Task tool doesn't support, FLIP binary needs config)
2. **Which MCP server to use for E2E test?** (Anthropic official? Playwright? File system?)
3. **Quality threshold for Gemini Flash?** (What % test pass rate is acceptable?)

---

**File**: `/Users/arielspivakovsky/src/flip/flip2/REVISED_PLAN_POST_REVIEW.md`
