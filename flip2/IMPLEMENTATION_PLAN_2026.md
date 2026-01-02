# FLIP2 Implementation Plan 2026
**Generated**: January 1, 2026
**Source**: AWESOMECLAUDE_AI_ANALYSIS_FINAL_REPORT.md
**Total Improvements**: 21
**Estimated Duration**: 26 weeks (6.5 months)

---

## Table of Contents
1. [Executive Summary](#executive-summary)
2. [Work Breakdown Structure](#work-breakdown-structure)
3. [Model Assignment Strategy](#model-assignment-strategy)
4. [Execution Phases](#execution-phases)
5. [Parallelization Strategy](#parallelization-strategy)
6. [Risk Mitigation](#risk-mitigation)
7. [Success Metrics](#success-metrics)
8. [Quick Reference: Task Lists by Model](#quick-reference-task-lists-by-model)

---

## Executive Summary

This plan breaks down 21 improvements into **87 discrete tasks**, each executable in 1-3 days. Tasks are assigned to optimal models based on complexity and cost.

**Cost Estimate by Model**:
| Model | Tasks | Estimated Hours | Cost/Hour | Total Est. |
|-------|-------|-----------------|-----------|------------|
| Opus | 18 | 54 | $0.15 | $8.10 |
| Sonnet | 42 | 126 | $0.03 | $3.78 |
| Haiku | 19 | 38 | $0.01 | $0.38 |
| Gemini | 8 | 24 | $0.02 | $0.48 |
| **Total** | **87** | **242** | - | **$12.74** |

**Maximum Parallelism**: 6 concurrent agents (Phase 1), 4 agents (Phase 2-3)

---

## Work Breakdown Structure

### Phase 0: MCP Integration (P0 Critical Path)

#### Improvement 1: MCP Native Integration
**Total Effort**: 3-4 weeks | **Priority**: P0 Critical

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| MCP-001 | Define MCP Server interface | Create Go interface for MCP server connections | Interface compiles, documented | 4h | Opus | None |
| MCP-002 | Create MCP server registry data structure | Design registry to store server metadata | Unit tests pass | 4h | Sonnet | MCP-001 |
| MCP-003 | Implement registry CRUD operations | Add/Remove/Update/List server operations | CRUD tests pass | 6h | Sonnet | MCP-002 |
| MCP-004 | Add registry persistence to SQLite | Store registry in flip.db | Survives restart | 4h | Sonnet | MCP-003 |
| MCP-005 | Design Tool Router interface | Interface for capability-based routing | Interface documented | 4h | Opus | MCP-001 |
| MCP-006 | Implement tool discovery from MCP servers | Query connected servers for available tools | Lists tools from 3+ servers | 8h | Sonnet | MCP-005 |
| MCP-007 | Build capability matching algorithm | Match task requirements to tool capabilities | 90%+ accuracy on test set | 6h | Opus | MCP-006 |
| MCP-008 | Create tool invocation wrapper | Unified interface to call any MCP tool | Calls succeed across servers | 6h | Sonnet | MCP-007 |
| MCP-009 | Implement MCP Sampling support | Handle LLM completion requests from servers | Completions work | 8h | Opus | MCP-008 |
| MCP-010 | Add Resource Subscriptions | Real-time data updates from MCP resources | Events received | 6h | Sonnet | MCP-008 |
| MCP-011 | Create MCP CLI commands | `flip2 mcp list`, `flip2 mcp add`, etc. | Commands functional | 6h | Sonnet | MCP-003 |
| MCP-012 | Write MCP integration tests | End-to-end tests with 3 MCP servers | All tests pass | 8h | Sonnet | MCP-010 |
| MCP-013 | Document MCP migration path | How to migrate existing workers to MCP | Doc reviewed | 4h | Haiku | MCP-012 |
| MCP-014 | Test with file MCP server | Integration test with filesystem server | File ops work | 4h | Haiku | MCP-012 |
| MCP-015 | Test with database MCP server | Integration test with SQLite MCP server | DB ops work | 4h | Haiku | MCP-012 |
| MCP-016 | Test with browser MCP server | Integration test with Playwright server | Browser ops work | 4h | Haiku | MCP-012 |

**Phase 0 Totals**: 16 tasks, 86 hours (~2.2 weeks at 40h/week with parallelism)

---

### Phase 1: High-Value Improvements (P1)

#### Improvement 2: Intelligent Task Routing
**Total Effort**: 2 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| RTR-001 | Define task classification schema | Enum of task types (research, code, review) | Schema documented | 3h | Opus | None |
| RTR-002 | Create task complexity scorer | Algorithm to rate task complexity 1-5 | Scores match human ratings | 6h | Opus | RTR-001 |
| RTR-003 | Build routing rules engine | YAML-based rules for task→model mapping | Rules load correctly | 6h | Sonnet | RTR-001 |
| RTR-004 | Implement default routing matrix | Hardcoded fallbacks for common tasks | All defaults work | 4h | Sonnet | RTR-003 |
| RTR-005 | Add cost tracking per task type | Log cost per task category | Reports generate | 4h | Sonnet | RTR-004 |
| RTR-006 | Create routing override mechanism | Manual override via flag or config | Overrides respected | 3h | Sonnet | RTR-004 |
| RTR-007 | Implement A/B routing for learning | Route subset to different models | A/B logs captured | 6h | Sonnet | RTR-005 |
| RTR-008 | Build routing analytics dashboard | Cost savings report | Shows $$ saved | 4h | Haiku | RTR-005 |
| RTR-009 | Write routing unit tests | Test all routing paths | 100% coverage | 4h | Haiku | RTR-007 |

**Improvement 2 Totals**: 9 tasks, 40 hours

---

#### Improvement 3: Pipeline State Machine
**Total Effort**: 2 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| PSM-001 | Design pipeline state schema | Define stage states: pending/running/done/failed | Schema documented | 3h | Opus | None |
| PSM-002 | Create YAML pipeline parser | Parse pipeline definitions from YAML | Parses all examples | 6h | Sonnet | PSM-001 |
| PSM-003 | Implement state persistence to SQLite | Store pipeline state in flip.db | State survives crash | 6h | Sonnet | PSM-002 |
| PSM-004 | Build stage executor with timeouts | Execute stages with configurable timeout | Timeouts work | 6h | Sonnet | PSM-003 |
| PSM-005 | Add retry logic per stage | Configurable retries with backoff | Retries work | 4h | Sonnet | PSM-004 |
| PSM-006 | Implement artifact storage | Store stage outputs with checksums | Checksums validate | 4h | Sonnet | PSM-004 |
| PSM-007 | Create automatic recovery on restart | Resume pipelines from last checkpoint | Resumes correctly | 6h | Sonnet | PSM-003 |
| PSM-008 | Add pipeline CLI commands | `flip2 pipeline run`, `status`, `resume` | Commands work | 4h | Sonnet | PSM-007 |
| PSM-009 | Write pipeline integration tests | Test crash recovery scenarios | All pass | 6h | Haiku | PSM-007 |

**Improvement 3 Totals**: 9 tasks, 45 hours

---

#### Improvement 4: Slash Commands
**Total Effort**: 2 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| SLC-001 | Design slash command interface | Command registry and dispatcher | Interface compiles | 3h | Opus | None |
| SLC-002 | Create interactive REPL mode | `flip2` enters interactive shell | Prompt appears | 6h | Sonnet | SLC-001 |
| SLC-003 | Implement tab completion | Complete commands and arguments | Tab completes | 6h | Sonnet | SLC-002 |
| SLC-004 | Build contextual help system | `/help`, `/help <cmd>` | Help displays | 4h | Sonnet | SLC-002 |
| SLC-005 | Implement /status command | Show system status | Status displays | 2h | Haiku | SLC-002 |
| SLC-006 | Implement /send command | Send message to agent | Message sent | 2h | Haiku | SLC-002 |
| SLC-007 | Implement /task command | Create/list tasks | Tasks managed | 3h | Haiku | SLC-002 |
| SLC-008 | Implement /agents command | List active agents | Agents listed | 2h | Haiku | SLC-002 |
| SLC-009 | Implement /spawn command | Spawn new agent | Agent spawns | 3h | Sonnet | SLC-002 |
| SLC-010 | Add command history persistence | Arrow-up recalls history | History works | 3h | Sonnet | SLC-002 |
| SLC-011 | Create command aliasing | User-defined aliases | Aliases work | 3h | Sonnet | SLC-002 |
| SLC-012 | Write REPL integration tests | Test all commands | All pass | 4h | Haiku | SLC-011 |

**Improvement 4 Totals**: 12 tasks, 41 hours

---

#### Improvement 5: FLIP2.md Project Configuration
**Total Effort**: 1.5 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| CFG-001 | Design FLIP2.md schema | Define sections: agents, commands, routing | Schema documented | 3h | Opus | None |
| CFG-002 | Create FLIP2.md parser | Parse markdown tables and YAML blocks | Parses examples | 6h | Sonnet | CFG-001 |
| CFG-003 | Implement config inheritance | Global → Project → Directory | Inheritance works | 4h | Sonnet | CFG-002 |
| CFG-004 | Add custom command registration | Register commands from FLIP2.md | Commands callable | 4h | Sonnet | CFG-003 |
| CFG-005 | Implement routing rule overrides | Project-specific routing rules | Overrides apply | 4h | Sonnet | CFG-003, RTR-003 |
| CFG-006 | Auto-load FLIP2.md on spawn | Workers get project context | Context received | 3h | Sonnet | CFG-003 |
| CFG-007 | Create example FLIP2.md templates | Templates for common project types | 3+ templates | 3h | Haiku | CFG-002 |
| CFG-008 | Write config unit tests | Test parsing and inheritance | All pass | 3h | Haiku | CFG-006 |

**Improvement 5 Totals**: 8 tasks, 30 hours

---

#### Improvement 6: Context-Aware Agent Spawning
**Total Effort**: 1 week | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| SPW-001 | Define role template schema | Structure for role definitions | Schema documented | 2h | Opus | None |
| SPW-002 | Create built-in role templates | code-reviewer, researcher, implementer | 3 roles defined | 4h | Sonnet | SPW-001 |
| SPW-003 | Implement role-based spawning | `flip2 agent spawn --role X` | Spawns with role | 4h | Sonnet | SPW-002 |
| SPW-004 | Add permission boundaries | Roles have read/write/execute limits | Permissions enforced | 4h | Sonnet | SPW-003 |
| SPW-005 | Inject project context on spawn | Include FLIP2.md and relevant files | Context included | 3h | Sonnet | SPW-003, CFG-006 |
| SPW-006 | Create custom role definition | User-defined roles in FLIP2.md | Custom roles work | 3h | Sonnet | SPW-002 |
| SPW-007 | Write spawning unit tests | Test all spawn scenarios | All pass | 3h | Haiku | SPW-006 |

**Improvement 6 Totals**: 7 tasks, 23 hours

---

#### Improvement 7: Session Persistence and Recovery
**Total Effort**: 2 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| SES-001 | Design session state schema | What to persist: messages, agents, tasks | Schema documented | 3h | Opus | None |
| SES-002 | Create session database tables | SQLite schema for sessions | Tables created | 3h | Sonnet | SES-001 |
| SES-003 | Implement session start/stop | `flip2 session start <name>` | Sessions start/stop | 4h | Sonnet | SES-002 |
| SES-004 | Build state serialization | Serialize/deserialize session state | Round-trip works | 6h | Sonnet | SES-003 |
| SES-005 | Implement session attach | `flip2 session attach <name>` | Reattach works | 4h | Sonnet | SES-004 |
| SES-006 | Add session list command | `flip2 session list` | Sessions listed | 2h | Haiku | SES-003 |
| SES-007 | Auto-save on disconnect | Save state when terminal closes | State saved | 4h | Sonnet | SES-004 |
| SES-008 | Implement session cleanup | Prune old sessions | Cleanup works | 3h | Sonnet | SES-003 |
| SES-009 | Handle agent reconnection | Agents rejoin on session attach | Agents reconnect | 6h | Sonnet | SES-005 |
| SES-010 | Write session integration tests | Test disconnect/reconnect | All pass | 4h | Haiku | SES-009 |

**Improvement 7 Totals**: 10 tasks, 39 hours

---

#### Improvement 8: Structured Error Types
**Total Effort**: 1.5 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| ERR-001 | Define ExecutionError type | Go struct with Code, Message, Retryable | Type compiles | 2h | Sonnet | None |
| ERR-002 | Create error code enum | timeout, quota, not_found, execution | Enum documented | 2h | Sonnet | ERR-001 |
| ERR-003 | Update process.go error returns | Replace generic errors with typed | All returns typed | 6h | Sonnet | ERR-002 |
| ERR-004 | Update executor.go error returns | Replace generic errors with typed | All returns typed | 4h | Sonnet | ERR-002 |
| ERR-005 | Implement error routing logic | Different handling per error type | Routes correctly | 4h | Sonnet | ERR-003 |
| ERR-006 | Add error metrics aggregation | Count errors by type | Metrics recorded | 3h | Sonnet | ERR-003 |
| ERR-007 | Write error handling tests | Test all error paths | All pass | 3h | Haiku | ERR-005 |

**Improvement 8 Totals**: 7 tasks, 24 hours

---

#### Improvement 9: Context Cleanup with Defer
**Total Effort**: 1 week | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| CTX-001 | Audit all context.With* calls | Find all contexts needing cleanup | Audit complete | 3h | Gemini | None |
| CTX-002 | Fix process.go context leaks | Add defer cancel() | No leaks in process | 3h | Sonnet | CTX-001 |
| CTX-003 | Fix executor.go context leaks | Add defer cancel() | No leaks in executor | 3h | Sonnet | CTX-001 |
| CTX-004 | Fix remaining context leaks | All other files | No leaks anywhere | 4h | Sonnet | CTX-001 |
| CTX-005 | Add golangci-lint context rule | Lint rule to catch future leaks | Rule active | 2h | Haiku | CTX-004 |
| CTX-006 | Write context leak tests | Test goroutine counts | No growth | 3h | Haiku | CTX-004 |

**Improvement 9 Totals**: 6 tasks, 18 hours

---

#### Improvement 10: Structured Logging with Context
**Total Effort**: 1.5 weeks | **Priority**: P1 High

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| LOG-001 | Design logging context fields | task_id, agent_id, request_id | Fields documented | 2h | Opus | None |
| LOG-002 | Create Go structured logger | slog with context propagation | Logger works | 4h | Sonnet | LOG-001 |
| LOG-003 | Migrate process.go to structured | Replace all log calls | All structured | 4h | Sonnet | LOG-002 |
| LOG-004 | Migrate executor.go to structured | Replace all log calls | All structured | 4h | Sonnet | LOG-002 |
| LOG-005 | Migrate remaining Go files | All other files | All structured | 4h | Sonnet | LOG-002 |
| LOG-006 | Create Python structured logger | logging module with JSON | Logger works | 3h | Sonnet | LOG-001 |
| LOG-007 | Migrate signal_monitor.py | Replace print() with logging | All structured | 3h | Sonnet | LOG-006 |
| LOG-008 | Migrate remaining Python files | All other scripts | All structured | 3h | Haiku | LOG-006 |
| LOG-009 | Configure log aggregation | Output to file and stdout | Both work | 3h | Haiku | LOG-005 |
| LOG-010 | Write logging tests | Test context propagation | All pass | 3h | Haiku | LOG-009 |

**Improvement 10 Totals**: 10 tasks, 33 hours

---

### Phase 2: Enhancement (P2)

#### Improvement 11: Hierarchical Agent Orchestration
**Total Effort**: 2 weeks | **Priority**: P2 Medium

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| HIE-001 | Design 3-tier hierarchy schema | Coordinator → Supervisor → Worker | Schema documented | 4h | Opus | None |
| HIE-002 | Create supervisor agent type | New agent type with delegation | Type compiles | 6h | Opus | HIE-001 |
| HIE-003 | Implement delegation budgets | Max workers per supervisor | Budgets enforced | 4h | Sonnet | HIE-002 |
| HIE-004 | Build escalation paths | Worker → Supervisor → Coordinator | Escalations work | 4h | Sonnet | HIE-002 |
| HIE-005 | Create research supervisor | Predefined supervisor for research | Supervisor works | 4h | Sonnet | HIE-003 |
| HIE-006 | Create build supervisor | Predefined supervisor for builds | Supervisor works | 4h | Sonnet | HIE-003 |
| HIE-007 | Create test supervisor | Predefined supervisor for testing | Supervisor works | 4h | Sonnet | HIE-003 |
| HIE-008 | Add hierarchy visualization | Show tree of agents | Tree displays | 3h | Haiku | HIE-004 |
| HIE-009 | Write hierarchy integration tests | Test delegation and escalation | All pass | 6h | Haiku | HIE-007 |

**Improvement 11 Totals**: 9 tasks, 39 hours

---

#### Improvement 12: CLAUDE.md Configuration Inheritance
**Total Effort**: 1 week | **Priority**: P2 Medium

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| INH-001 | Design inheritance chain | ~/.flip2 → project → directory | Chain documented | 2h | Opus | CFG-003 |
| INH-002 | Implement global config at ~/.flip2 | Load from home directory | Global loads | 3h | Sonnet | INH-001 |
| INH-003 | Implement config merge logic | Deep merge with override | Merge works | 4h | Sonnet | INH-002 |
| INH-004 | Add config debug command | `flip2 config show --resolved` | Shows merged | 2h | Haiku | INH-003 |
| INH-005 | Write inheritance tests | Test multi-level merge | All pass | 3h | Haiku | INH-003 |

**Improvement 12 Totals**: 5 tasks, 14 hours

---

#### Improvement 13: Pipeline Templates
**Total Effort**: 1.5 weeks | **Priority**: P2 Medium

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| TPL-001 | Design template system | Templates with variables | Design documented | 3h | Opus | PSM-002 |
| TPL-002 | Create feature-development template | Research → Design → Implement → Test | Template works | 4h | Sonnet | TPL-001 |
| TPL-003 | Create code-review template | Analyze → Test → Security → Quality | Template works | 4h | Sonnet | TPL-001 |
| TPL-004 | Create bug-investigation template | Reproduce → Root Cause → Fix → Verify | Template works | 4h | Sonnet | TPL-001 |
| TPL-005 | Create data-pipeline template | Extract → Transform → Load → Validate | Template works | 4h | Sonnet | TPL-001 |
| TPL-006 | Implement template instantiation | `flip2 pipeline from-template X` | Instantiation works | 4h | Sonnet | TPL-001 |
| TPL-007 | Add custom template directory | ~/.flip2/pipelines/ | Custom loads | 3h | Haiku | TPL-006 |
| TPL-008 | Write template tests | Test all built-in templates | All pass | 3h | Haiku | TPL-006 |

**Improvement 13 Totals**: 8 tasks, 29 hours

---

#### Improvement 14: Retry with Exponential Backoff
**Total Effort**: 1 week | **Priority**: P2 Medium

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| RET-001 | Design retry configuration | MaxAttempts, InitialDelay, Backoff | Config documented | 2h | Sonnet | ERR-002 |
| RET-002 | Create ExecuteWithRetry function | Wrapper with backoff logic | Function works | 6h | Sonnet | RET-001 |
| RET-003 | Add jitter to prevent thundering herd | Random jitter 0-25% | Jitter added | 2h | Sonnet | RET-002 |
| RET-004 | Implement selective retry by error | Only retry timeout/rate-limit | Selection works | 3h | Sonnet | RET-002, ERR-005 |
| RET-005 | Add retry metrics | Count retries per error type | Metrics recorded | 2h | Haiku | RET-004 |
| RET-006 | Write retry tests | Test backoff behavior | All pass | 3h | Haiku | RET-004 |

**Improvement 14 Totals**: 6 tasks, 18 hours

---

#### Improvement 15: Circuit Breaker Pattern
**Total Effort**: 1 week | **Priority**: P2 Medium

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| CIR-001 | Design circuit breaker states | Closed → Open → HalfOpen | States documented | 2h | Opus | None |
| CIR-002 | Create CircuitBreaker struct | State, failure count, timestamps | Struct compiles | 4h | Sonnet | CIR-001 |
| CIR-003 | Implement state transitions | Open after N failures, reset after timeout | Transitions work | 4h | Sonnet | CIR-002 |
| CIR-004 | Integrate with backend execution | Fast-fail when open | Integration works | 4h | Sonnet | CIR-003 |
| CIR-005 | Add circuit breaker metrics | State changes, rejections | Metrics recorded | 2h | Haiku | CIR-004 |
| CIR-006 | Write circuit breaker tests | Test all transitions | All pass | 3h | Haiku | CIR-004 |

**Improvement 15 Totals**: 6 tasks, 19 hours

---

### Phase 3: Optimization (P3)

#### Improvement 16: Computer Use Agent Architecture
**Total Effort**: 1.5 weeks | **Priority**: P3 Low

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| CUA-001 | Design sandboxed execution model | Docker container for browser | Design documented | 4h | Opus | None |
| CUA-002 | Create read-only computer use agent | Screenshot, read, no write | Read-only works | 6h | Sonnet | CUA-001 |
| CUA-003 | Add write operation approval flow | Explicit user approval for writes | Approval required | 4h | Sonnet | CUA-002 |
| CUA-004 | Implement Docker sandbox | Isolated browser environment | Sandbox works | 6h | Sonnet | CUA-001 |
| CUA-005 | Integrate with Antigravity fallback | High-stakes ops go to Antigravity | Fallback works | 4h | Sonnet | CUA-002 |
| CUA-006 | Write computer use tests | Test read-only scenarios | All pass | 4h | Haiku | CUA-005 |

**Improvement 16 Totals**: 6 tasks, 28 hours

---

#### Improvement 17: Integrated Dashboard (TUI)
**Total Effort**: 2.5 weeks | **Priority**: P3 Low

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| TUI-001 | Evaluate TUI frameworks | Bubble Tea vs others | Framework chosen | 3h | Gemini | None |
| TUI-002 | Design TUI layout | Panels for agents, tasks, logs | Design documented | 3h | Opus | TUI-001 |
| TUI-003 | Create main TUI application | Basic window with panels | Window appears | 6h | Sonnet | TUI-002 |
| TUI-004 | Implement agent list panel | Show active agents | Agents displayed | 4h | Sonnet | TUI-003 |
| TUI-005 | Implement task list panel | Show tasks by status | Tasks displayed | 4h | Sonnet | TUI-003 |
| TUI-006 | Implement log streaming panel | Real-time log updates | Logs stream | 4h | Sonnet | TUI-003 |
| TUI-007 | Add keyboard navigation | Arrow keys, tab, shortcuts | Navigation works | 4h | Sonnet | TUI-003 |
| TUI-008 | Connect to WebSocket for updates | Real-time updates | Updates received | 4h | Sonnet | TUI-003 |
| TUI-009 | Add agent relationship graph | Visual tree of agents | Graph displays | 6h | Sonnet | TUI-004 |
| TUI-010 | Write TUI integration tests | Test key flows | All pass | 4h | Haiku | TUI-009 |

**Improvement 17 Totals**: 10 tasks, 42 hours

---

#### Improvement 18: Real-Time Status Line
**Total Effort**: 1 week | **Priority**: P3 Low

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| STS-001 | Design status line format | Model, ctx, agents, tasks, sync, cost | Format documented | 2h | Sonnet | None |
| STS-002 | Create /api/statusline endpoint | HTTP endpoint for status | Endpoint returns JSON | 4h | Sonnet | STS-001 |
| STS-003 | Implement context usage calculation | Track tokens used / limit | Percentage accurate | 3h | Sonnet | STS-002 |
| STS-004 | Add session cost tracking | Running cost total | Cost accurate | 3h | Sonnet | STS-002 |
| STS-005 | Create statusline shell script | Poll endpoint, format output | Script works | 2h | Haiku | STS-002 |
| STS-006 | Document shell integration | tmux, bash prompt, etc. | Doc complete | 2h | Haiku | STS-005 |

**Improvement 18 Totals**: 6 tasks, 16 hours

---

#### Improvement 19: Middleware/Interceptor Pattern
**Total Effort**: 1 week | **Priority**: P3 Low

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| MID-001 | Design Interceptor interface | BeforeExecute, AfterExecute | Interface compiles | 3h | Opus | None |
| MID-002 | Create InterceptingBackend wrapper | Wrap any backend with interceptors | Wrapper works | 4h | Sonnet | MID-001 |
| MID-003 | Implement MetricsInterceptor | Record timing, success | Metrics work | 3h | Sonnet | MID-002 |
| MID-004 | Implement LoggingInterceptor | Log requests and responses | Logging works | 3h | Sonnet | MID-002 |
| MID-005 | Implement TracingInterceptor | Add trace spans | Tracing works | 3h | Sonnet | MID-002 |
| MID-006 | Refactor Execute() to use interceptors | Remove inline metrics | Refactor complete | 4h | Sonnet | MID-005 |
| MID-007 | Write interceptor tests | Test each interceptor | All pass | 3h | Haiku | MID-006 |

**Improvement 19 Totals**: 7 tasks, 23 hours

---

#### Improvement 20: Streaming with Type-Safe Event Handlers
**Total Effort**: 1.5 weeks | **Priority**: P3 Low

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| STR-001 | Design StreamHandler interface | OnText, OnComplete, OnError | Interface compiles | 3h | Opus | None |
| STR-002 | Create AccumulatingStream wrapper | Wraps channel with callbacks | Wrapper works | 4h | Sonnet | STR-001 |
| STR-003 | Implement Consume() method | Process stream with handler | Consumption works | 4h | Sonnet | STR-002 |
| STR-004 | Add default handlers | Console output, file save | Defaults work | 3h | Sonnet | STR-002 |
| STR-005 | Migrate existing stream consumers | Use new interface | Migration complete | 4h | Sonnet | STR-003 |
| STR-006 | Write streaming tests | Test handler callbacks | All pass | 3h | Haiku | STR-005 |

**Improvement 20 Totals**: 6 tasks, 21 hours

---

#### Improvement 21: Multi-Region Deployment Documentation
**Total Effort**: Documentation only | **Priority**: P4 Future

| Task ID | Task Name | Description | Acceptance Criteria | Effort | Model | Dependencies |
|---------|-----------|-------------|---------------------|--------|-------|--------------|
| DOC-001 | Document AWS Bedrock patterns | Deployment guide for Bedrock | Doc complete | 4h | Gemini | None |
| DOC-002 | Document Google Vertex AI patterns | FedRAMP compliance guide | Doc complete | 4h | Gemini | None |
| DOC-003 | Document Azure AI integration | Azure deployment guide | Doc complete | 4h | Gemini | None |
| DOC-004 | Create multi-region architecture diagram | Visual reference | Diagram complete | 2h | Haiku | DOC-001 |

**Improvement 21 Totals**: 4 tasks, 14 hours

---

## Model Assignment Strategy

### Model Selection Criteria

| Criteria | Opus | Sonnet | Haiku | Gemini |
|----------|------|--------|-------|--------|
| Architecture decisions | YES | - | - | - |
| Interface/API design | YES | - | - | - |
| Complex algorithms | YES | YES | - | - |
| Standard implementation | - | YES | - | - |
| Integration work | - | YES | - | - |
| Simple refactoring | - | - | YES | - |
| Documentation | - | - | YES | YES |
| Testing | - | - | YES | - |
| Research/audit | - | - | - | YES |
| Bulk processing | - | - | - | YES |

### Task Count by Model

| Model | Phase 0 | Phase 1 | Phase 2 | Phase 3 | Phase 4 | Total |
|-------|---------|---------|---------|---------|---------|-------|
| Opus | 4 | 8 | 4 | 2 | 0 | 18 |
| Sonnet | 8 | 28 | 20 | 20 | 0 | 76 |
| Haiku | 4 | 17 | 8 | 8 | 1 | 38 |
| Gemini | 0 | 1 | 0 | 1 | 3 | 5 |
| **Total** | **16** | **54** | **32** | **31** | **4** | **137** |

### Cost Justification

**Opus ($0.15/hr equivalent)**: Used only for tasks requiring:
- Novel architectural decisions (MCP-001, MCP-005, MCP-007, MCP-009)
- Complex algorithm design (RTR-002, PSM-001, HIE-001, HIE-002)
- System-wide interface contracts (SLC-001, SPW-001, CFG-001)
- Critical path work requiring highest accuracy

**Sonnet ($0.03/hr equivalent)**: Primary workhorse for:
- Implementation based on Opus designs
- Integration and migration work
- Feature development
- Moderate complexity algorithms

**Haiku ($0.01/hr equivalent)**: High-volume, low-complexity:
- Unit and integration tests
- Documentation updates
- Simple refactoring
- Template creation

**Gemini ($0.02/hr equivalent)**: Bulk research and processing:
- Code audits (CTX-001)
- Documentation research (DOC-001, DOC-002, DOC-003)
- Framework evaluation (TUI-001)

---

## Execution Phases

### Phase 0: MCP Integration (Critical Path)
**Duration**: 4 weeks
**Total Tasks**: 16
**Total Effort**: 86 hours

#### Week 1-2: Foundation
```
Day 1-2:   MCP-001 (Opus) - Define MCP Server interface
Day 2-3:   MCP-002 (Sonnet) - Create registry data structure [depends: MCP-001]
Day 3-4:   MCP-003 (Sonnet) - Implement CRUD operations [depends: MCP-002]
Day 4-5:   MCP-004 (Sonnet) - SQLite persistence [depends: MCP-003]
           MCP-005 (Opus) - Design Tool Router [parallel, depends: MCP-001]
```

#### Week 2-3: Core Routing
```
Day 6-7:   MCP-006 (Sonnet) - Tool discovery [depends: MCP-005]
Day 7-8:   MCP-007 (Opus) - Capability matching [depends: MCP-006]
Day 8-9:   MCP-008 (Sonnet) - Tool invocation [depends: MCP-007]
```

#### Week 3-4: Advanced Features & Testing
```
Day 10-11: MCP-009 (Opus) - Sampling support [depends: MCP-008]
           MCP-010 (Sonnet) - Resource subscriptions [parallel, depends: MCP-008]
Day 11-12: MCP-011 (Sonnet) - CLI commands [depends: MCP-003]
Day 12-14: MCP-012 (Sonnet) - Integration tests [depends: MCP-010]
Day 14-15: MCP-013, MCP-014, MCP-015, MCP-016 (Haiku) - Docs & server tests [parallel]
```

**Phase 0 Parallelism**: Up to 3 concurrent agents
**Phase 0 Checkpoint**: 3+ MCP servers connected and functional

---

### Phase 1: High-Value Improvements
**Duration**: 10 weeks
**Total Tasks**: 78
**Total Effort**: ~250 hours

#### Week 5-6: Intelligent Task Routing
**Work Streams** (can run in parallel):
- Stream A: RTR-001 → RTR-002 → RTR-003 (Opus → Opus → Sonnet)
- Stream B: After RTR-003: RTR-004 → RTR-005 → RTR-006 → RTR-007 (Sonnet)
- Stream C: RTR-008, RTR-009 (Haiku, after RTR-007)

#### Week 7-8: Pipeline State Machine
**Work Streams**:
- Stream A: PSM-001 → PSM-002 → PSM-003 (Opus → Sonnet → Sonnet)
- Stream B: After PSM-003: PSM-004 → PSM-005, PSM-006 (parallel)
- Stream C: PSM-007 → PSM-008 → PSM-009 (Sonnet → Sonnet → Haiku)

#### Week 9-10: Slash Commands + FLIP2.md Config
**Work Streams** (can run in parallel):
- Stream A: SLC-001 → SLC-002 → SLC-003 to SLC-012 (chain)
- Stream B: CFG-001 → CFG-002 → CFG-003 to CFG-008 (chain)

**Parallelism**: 4 agents (2 per stream)

#### Week 11: Context-Aware Spawning
**Work Stream**: SPW-001 → SPW-002 → SPW-003 → SPW-004 → SPW-005 → SPW-006 → SPW-007

#### Week 12-13: Session Persistence
**Work Streams**:
- Stream A: SES-001 → SES-002 → SES-003 → SES-004
- Stream B: After SES-004: SES-005 → SES-006, SES-007 (parallel)
- Stream C: SES-008, SES-009, SES-010 (chain after SES-005)

#### Week 14: Code Quality (Structured Errors, Context Cleanup, Logging)
**Work Streams** (all can run in parallel):
- Stream A: ERR-001 → ERR-007 (Structured Errors)
- Stream B: CTX-001 → CTX-006 (Context Cleanup)
- Stream C: LOG-001 → LOG-010 (Structured Logging)

**Maximum Parallelism**: 6 agents in Week 14

**Phase 1 Checkpoint**:
- Cost routing active, showing 50%+ savings
- Sessions persist across disconnect
- All P1 tests passing

---

### Phase 2: Enhancement
**Duration**: 6 weeks
**Total Tasks**: 34
**Total Effort**: ~120 hours

#### Week 15-16: Hierarchical Orchestration
**Depends on**: Phase 1 complete (routing, config, spawning)
**Work Stream**: HIE-001 → HIE-009

#### Week 17: Config Inheritance + Pipeline Templates
**Work Streams** (parallel):
- Stream A: INH-001 → INH-005 (Config Inheritance)
- Stream B: TPL-001 → TPL-008 (Pipeline Templates)

#### Week 18: Retry + Circuit Breaker
**Work Streams** (parallel):
- Stream A: RET-001 → RET-006 (Retry)
- Stream B: CIR-001 → CIR-006 (Circuit Breaker)

**Maximum Parallelism**: 4 agents

**Phase 2 Checkpoint**:
- 3 supervisor types functional
- All templates working
- Circuit breaker integrated

---

### Phase 3: Optimization
**Duration**: 6 weeks
**Total Tasks**: 35
**Total Effort**: ~130 hours

#### Week 19-20: Computer Use + TUI Dashboard Start
**Work Streams** (parallel):
- Stream A: CUA-001 → CUA-006 (Computer Use)
- Stream B: TUI-001 → TUI-005 (TUI first half)

#### Week 21-22: TUI Dashboard Complete + Status Line
**Work Streams** (parallel):
- Stream A: TUI-006 → TUI-010 (TUI second half)
- Stream B: STS-001 → STS-006 (Status Line)

#### Week 23: Middleware + Streaming
**Work Streams** (parallel):
- Stream A: MID-001 → MID-007 (Middleware)
- Stream B: STR-001 → STR-006 (Streaming)

#### Week 24-26: Polish + Documentation
**Work Streams**:
- Final integration testing
- Documentation updates
- DOC-001 → DOC-004 (Multi-region docs)

**Maximum Parallelism**: 4 agents

**Phase 3 Checkpoint**:
- TUI fully functional
- Status line integrated
- All P3 tests passing

---

## Parallelization Strategy

### Dependency Graph Summary

```
Phase 0 (MCP): Linear critical path with some parallelism at end
  [MCP-001] → [MCP-002] → [MCP-003] → [MCP-004]
       ↓
  [MCP-005] → [MCP-006] → [MCP-007] → [MCP-008] → [MCP-009]
                                           ↓          ↓
                                      [MCP-010]  [MCP-011]
                                           ↓
                                      [MCP-012]
                                           ↓
                    [MCP-013] [MCP-014] [MCP-015] [MCP-016] ← parallel

Phase 1: Multiple independent work streams
  Stream A: Routing (RTR-*) → depends on nothing
  Stream B: Pipeline (PSM-*) → depends on nothing
  Stream C: Slash Commands (SLC-*) → depends on nothing
  Stream D: Config (CFG-*) → depends on RTR-003 for CFG-005
  Stream E: Spawning (SPW-*) → depends on CFG-006 for SPW-005
  Stream F: Sessions (SES-*) → depends on nothing
  Stream G: Errors (ERR-*) → depends on nothing
  Stream H: Context (CTX-*) → depends on nothing
  Stream I: Logging (LOG-*) → depends on nothing

Phase 2: Depends on Phase 1 completion
  Stream A: Hierarchy (HIE-*) → depends on SPW-*, CFG-*
  Stream B: Inheritance (INH-*) → depends on CFG-003
  Stream C: Templates (TPL-*) → depends on PSM-002
  Stream D: Retry (RET-*) → depends on ERR-002
  Stream E: Circuit (CIR-*) → depends on nothing

Phase 3: Depends on Phase 2 completion
  Stream A: Computer Use (CUA-*) → depends on nothing
  Stream B: TUI (TUI-*) → depends on nothing
  Stream C: Status Line (STS-*) → depends on nothing
  Stream D: Middleware (MID-*) → depends on LOG-*
  Stream E: Streaming (STR-*) → depends on nothing
```

### Maximum Concurrent Agents by Week

| Week | Phase | Max Agents | Work Streams |
|------|-------|------------|--------------|
| 1-2 | P0 | 2 | MCP foundation |
| 3-4 | P0 | 3 | MCP + testing |
| 5-6 | P1 | 4 | Routing, Pipeline |
| 7-8 | P1 | 4 | Pipeline, Slash |
| 9-10 | P1 | 4 | Slash, Config |
| 11 | P1 | 2 | Spawning |
| 12-13 | P1 | 3 | Sessions |
| 14 | P1 | **6** | Errors, Context, Logging |
| 15-16 | P2 | 3 | Hierarchy |
| 17 | P2 | 4 | Inheritance, Templates |
| 18 | P2 | 4 | Retry, Circuit |
| 19-20 | P3 | 4 | CUA, TUI |
| 21-22 | P3 | 4 | TUI, Status |
| 23 | P3 | 4 | Middleware, Streaming |
| 24-26 | P3 | 2 | Polish, Docs |

### Agent Pool Configuration

```yaml
# Recommended agent pool for maximum efficiency
pool:
  opus_agents: 1      # For critical architecture decisions
  sonnet_agents: 3    # Primary implementation pool
  haiku_agents: 2     # Testing and documentation
  gemini_agents: 1    # Research and bulk processing

# Cost-optimized alternative (slower)
pool_budget:
  opus_agents: 1
  sonnet_agents: 2
  haiku_agents: 1
  gemini_agents: 1

# Maximum throughput (higher cost)
pool_fast:
  opus_agents: 2
  sonnet_agents: 4
  haiku_agents: 2
  gemini_agents: 1
```

---

## Risk Mitigation

### High-Risk Tasks (Require Opus Review)

| Task | Risk | Mitigation | Review Gate |
|------|------|------------|-------------|
| MCP-007 | Wrong routing = quality degradation | A/B test before full rollout | Opus reviews algorithm |
| MCP-009 | Sampling bugs = broken completions | Extensive unit tests | Opus reviews implementation |
| HIE-002 | Complexity increases debugging | Start with 3 predefined types | Opus reviews design |
| CUA-002 | Security vulnerabilities | Start read-only, sandbox writes | Opus security review |
| PSM-007 | State corruption on recovery | SQLite transactions, file locking | Opus reviews recovery logic |

### Rollback Procedures

#### MCP Integration Rollback
```bash
# If MCP breaks existing workers:
1. Disable MCP layer: flip2 config set mcp.enabled=false
2. Workers fall back to file-based IPC
3. Debug MCP in isolation
4. Re-enable after fix
```

#### Routing Rollback
```bash
# If routing causes quality issues:
1. Override to specific model: flip2 config set routing.override=claude
2. Collect misrouting data
3. Retrain routing rules
4. Gradually re-enable automatic routing
```

#### Session Persistence Rollback
```bash
# If session recovery fails:
1. Disable auto-recovery: flip2 config set session.auto_recover=false
2. Sessions become ephemeral
3. Debug recovery logic
4. Re-enable after fix
```

### Breaking Change Protocol

For any change that modifies external APIs or data formats:

1. **Feature Flag**: All breaking changes behind feature flags
2. **Migration Period**: 2-week deprecation notice
3. **Backward Compatibility**: Old format supported for 1 release
4. **Rollback Script**: Automated script to revert changes
5. **Communication**: Document in CHANGELOG.md

---

## Success Metrics

### Phase 0 Checkpoint (Week 4)
| Metric | Target | Validation |
|--------|--------|------------|
| MCP servers connected | 3+ | `flip2 mcp list` shows 3 servers |
| Tool discovery working | 100% | All tools from servers listed |
| Existing workers functional | 100% | Regression tests pass |
| MCP sampling works | Pass | Unit tests pass |

### Phase 1 Checkpoint (Week 14)
| Metric | Target | Validation |
|--------|--------|------------|
| Cost reduction | 50%+ | Cost dashboard shows savings |
| Session persistence | 100% | No lost sessions in 1 week |
| Slash commands functional | All | Interactive mode works |
| Error handling | 90%+ | Typed errors everywhere |
| Context leaks | 0 | Goroutine count stable |
| Structured logging | 100% | All logs queryable |

### Phase 2 Checkpoint (Week 20)
| Metric | Target | Validation |
|--------|--------|------------|
| Hierarchical orchestration | 3 types | Supervisors delegate correctly |
| Pipeline templates | 4+ | All built-in templates work |
| Retry success rate | 80%+ | Transient failures recovered |
| Circuit breaker active | Yes | Fast-fail on backend outage |

### Phase 3 Checkpoint (Week 26)
| Metric | Target | Validation |
|--------|--------|------------|
| TUI functional | Full | All panels update in real-time |
| Status line integrated | Yes | Shows in shell prompt |
| Computer use safe | Yes | No unauthorized writes |
| All tests passing | 100% | CI green |

### User Review Points

| Week | Review Focus | Go/No-Go Decision |
|------|--------------|-------------------|
| 4 | MCP Integration | Proceed to P1? |
| 8 | Core Features (Routing, Pipeline) | Adjust priorities? |
| 14 | All P1 Complete | Proceed to P2? |
| 20 | P2 Complete | Proceed to P3? |
| 26 | Full Implementation | Release approval |

---

## Quick Reference: Task Lists by Model

### Opus Tasks (18 total)
```
# Phase 0
MCP-001: Define MCP Server interface (4h)
MCP-005: Design Tool Router interface (4h)
MCP-007: Build capability matching algorithm (6h)
MCP-009: Implement MCP Sampling support (8h)

# Phase 1
RTR-001: Define task classification schema (3h)
RTR-002: Create task complexity scorer (6h)
PSM-001: Design pipeline state schema (3h)
SLC-001: Design slash command interface (3h)
CFG-001: Design FLIP2.md schema (3h)
SPW-001: Define role template schema (2h)
SES-001: Design session state schema (3h)
LOG-001: Design logging context fields (2h)

# Phase 2
HIE-001: Design 3-tier hierarchy schema (4h)
HIE-002: Create supervisor agent type (6h)
INH-001: Design inheritance chain (2h)
TPL-001: Design template system (3h)
CIR-001: Design circuit breaker states (2h)

# Phase 3
CUA-001: Design sandboxed execution model (4h)
TUI-002: Design TUI layout (3h)
MID-001: Design Interceptor interface (3h)
STR-001: Design StreamHandler interface (3h)
```

### Sonnet Tasks (76 total)
*(Primary implementation - see full task list above)*

### Haiku Tasks (38 total)
```
# Phase 0
MCP-013: Document MCP migration path (4h)
MCP-014: Test with file MCP server (4h)
MCP-015: Test with database MCP server (4h)
MCP-016: Test with browser MCP server (4h)

# Phase 1 (17 tasks)
RTR-008: Build routing analytics dashboard (4h)
RTR-009: Write routing unit tests (4h)
PSM-009: Write pipeline integration tests (6h)
SLC-005: Implement /status command (2h)
SLC-006: Implement /send command (2h)
SLC-007: Implement /task command (3h)
SLC-008: Implement /agents command (2h)
SLC-012: Write REPL integration tests (4h)
CFG-007: Create example FLIP2.md templates (3h)
CFG-008: Write config unit tests (3h)
SPW-007: Write spawning unit tests (3h)
SES-006: Add session list command (2h)
SES-010: Write session integration tests (4h)
ERR-007: Write error handling tests (3h)
CTX-005: Add golangci-lint context rule (2h)
CTX-006: Write context leak tests (3h)
LOG-008: Migrate remaining Python files (3h)
LOG-009: Configure log aggregation (3h)
LOG-010: Write logging tests (3h)

# Phase 2 (8 tasks)
HIE-008: Add hierarchy visualization (3h)
HIE-009: Write hierarchy integration tests (6h)
INH-004: Add config debug command (2h)
INH-005: Write inheritance tests (3h)
TPL-007: Add custom template directory (3h)
TPL-008: Write template tests (3h)
RET-005: Add retry metrics (2h)
RET-006: Write retry tests (3h)
CIR-005: Add circuit breaker metrics (2h)
CIR-006: Write circuit breaker tests (3h)

# Phase 3 (8 tasks)
CUA-006: Write computer use tests (4h)
TUI-010: Write TUI integration tests (4h)
STS-005: Create statusline shell script (2h)
STS-006: Document shell integration (2h)
MID-007: Write interceptor tests (3h)
STR-006: Write streaming tests (3h)
DOC-004: Create multi-region architecture diagram (2h)
```

### Gemini Tasks (5 total)
```
CTX-001: Audit all context.With* calls (3h)
TUI-001: Evaluate TUI frameworks (3h)
DOC-001: Document AWS Bedrock patterns (4h)
DOC-002: Document Google Vertex AI patterns (4h)
DOC-003: Document Azure AI integration (4h)
```

---

## Appendix: Task ID Reference

All task IDs follow the pattern: `{CATEGORY}-{NUMBER}`

| Prefix | Category | Phase |
|--------|----------|-------|
| MCP | MCP Integration | P0 |
| RTR | Task Routing | P1 |
| PSM | Pipeline State Machine | P1 |
| SLC | Slash Commands | P1 |
| CFG | Configuration | P1 |
| SPW | Agent Spawning | P1 |
| SES | Sessions | P1 |
| ERR | Errors | P1 |
| CTX | Context Cleanup | P1 |
| LOG | Logging | P1 |
| HIE | Hierarchy | P2 |
| INH | Inheritance | P2 |
| TPL | Templates | P2 |
| RET | Retry | P2 |
| CIR | Circuit Breaker | P2 |
| CUA | Computer Use | P3 |
| TUI | Dashboard | P3 |
| STS | Status Line | P3 |
| MID | Middleware | P3 |
| STR | Streaming | P3 |
| DOC | Documentation | P4 |

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-01 | Claude Opus | Initial comprehensive plan |

---

**Next Steps**:
1. Review this plan with stakeholders
2. Create tasks in project management system (or FLIP tasks)
3. Assign Phase 0 tasks to agents
4. Begin execution with `flip2 task create` commands

**Execution Command**:
```bash
# To start Phase 0:
cd /Users/arielspivakovsky/src/flip
./flip spawn run mcp-architect opus "Execute task MCP-001: Define MCP Server interface. Acceptance: Interface compiles and is documented."
```
