# Spawn Orchestrator Command

**For User**: Copy/paste this command to spawn the orchestrator agent

---

## Option 1: FLIP Binary (Preferred)

```bash
cd /Users/arielspivakovsky/src/flip

./flip spawn run orchestrator gemini-flash "You are the FLIP2 Orchestrator conducting an A/B test.

**YOUR MISSION**: Balance work between Gemini Flash and Haiku workers, compare results.

**READ THESE FILES**:
1. /Users/arielspivakovsky/src/flip/flip2/ORCHESTRATOR_AB_TEST_PLAN.md (full test plan)
2. /Users/arielspivakovsky/src/flip/flip2/COORDINATOR_TO_AG_COMMANDS.md (context)
3. /Users/arielspivakovsky/src/flip/flip2/REVISED_PLAN_POST_REVIEW.md (strategy)

**EXECUTE**:
1. Spawn 3 Gemini Flash workers (MCP-009, RTR-002, SES-003)
2. Spawn 3 Haiku workers (same tasks for comparison)
3. Monitor progress every 10 minutes
4. Collect metrics: time, tokens, cost, quality, test pass rate
5. Create GEMINI_VS_HAIKU_RESULTS.md with comparison
6. Report recommendation in AG_STATUS_UPDATES.md

**WORK AUTONOMOUSLY**. Report results when complete.

**COMMUNICATION**:
- Read commands: COORDINATOR_TO_AG_COMMANDS.md
- Write status: AG_STATUS_UPDATES.md
- Worker activity: WORKER_ACTIVITY_LOG.md"
```

---

## Option 2: Antigravity (If FLIP Binary Unavailable)

```bash
# In new terminal with Antigravity + Gemini 3:
# Paste this prompt:

You are the FLIP2 Orchestrator conducting an A/B test comparing Gemini Flash vs Haiku for coding tasks.

Work directory: /Users/arielspivakovsky/src/flip/flip2

READ FIRST:
- ORCHESTRATOR_AB_TEST_PLAN.md (full test plan)
- COORDINATOR_TO_AG_COMMANDS.md (context)

YOUR TASK:
1. Spawn 6 workers (3 Gemini Flash, 3 Haiku) on same tasks
2. Compare: cost, quality, test pass rate
3. Document in GEMINI_VS_HAIKU_RESULTS.md
4. Recommend strategy

SPAWN WORKERS:
- Use FLIP binary: ./flip spawn run <worker-id> <model> "<prompt>"
- If blocked, use alternative spawn method
- Tasks: MCP-009 (Sampling), RTR-002 (Scorer), SES-003 (Session)

REPORT:
Write status to AG_STATUS_UPDATES.md every 10 minutes.
```

---

## Option 3: Manual Spawn (User Does It)

If autonomous spawning doesn't work:

```bash
cd /Users/arielspivakovsky/src/flip

# Gemini Flash workers
./flip spawn run worker7 gemini-flash "$(cat ORCHESTRATOR_AB_TEST_PLAN.md | grep -A 50 'Worker 7')"
./flip spawn run worker8 gemini-flash "$(cat ORCHESTRATOR_AB_TEST_PLAN.md | grep -A 50 'Worker 8')"
./flip spawn run worker9 gemini-flash "$(cat ORCHESTRATOR_AB_TEST_PLAN.md | grep -A 50 'Worker 9')"

# Haiku workers (for comparison)
./flip spawn run worker10 haiku "$(cat ORCHESTRATOR_AB_TEST_PLAN.md | grep -A 50 'Worker 7' | sed 's/Worker 7/Worker 10/g')"
./flip spawn run worker11 haiku "$(cat ORCHESTRATOR_AB_TEST_PLAN.md | grep -A 50 'Worker 8' | sed 's/Worker 8/Worker 11/g')"
./flip spawn run worker12 haiku "$(cat ORCHESTRATOR_AB_TEST_PLAN.md | grep -A 50 'Worker 9' | sed 's/Worker 9/Worker 12/g')"
```

---

**Recommended**: Try Option 1 first (FLIP binary with orchestrator). If that fails, use Option 2 (Antigravity manual). If all else fails, Option 3 (manual spawn all 6 workers).
