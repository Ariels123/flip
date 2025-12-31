# FLIP Roadmap - Feature Priorities

*Generated: 2025-12-11 based on multi-model architecture review*

## Consensus from Reviews

### Claude Opus Analysis (Deep Review)
| Priority | Feature | Complexity | Value |
|----------|---------|------------|-------|
| 1 | Vibe Scorecard (Quality Evaluation) | Medium | Critical |
| 2 | Capability Registry | Low-Medium | High |
| 3 | Feedback Loop + Auto-Retry | Medium | High |
| 4 | TDD Enforcement | Medium-High | High |
| 5 | Cost Tracking | Low | Medium |

### Gemini Analysis (Quick Assessment)
| Rank | Feature | Impact | Complexity | Value Ratio |
|------|---------|--------|------------|-------------|
| 1 | Capability Matching | 9/10 | 2/10 | 4.5 |
| 2 | Cost Tracking | 7/10 | 2/10 | 3.5 |
| 3 | Vibe Scorecard | 8/10 | 4/10 | 2.0 |
| 4 | TDD Enforcement | 8/10 | 7/10 | 1.1 |
| 5 | RAG Integration | 6/10 | 7/10 | 0.9 |

---

## Combined Priority List

### Phase 1: Quick Wins (Low Complexity, High Value)
1. **Capability Registry** - Both reviewers agree this is easy and impactful
   - Add capabilities field to agents table
   - Add `./flip register <agent> --capabilities code,test,browser`
   - Auto-route tasks based on required capabilities

2. **Cost Tracking** - Gemini rates 3.5 value ratio, Opus says Low complexity
   - Add cost_usd, tokens columns to task results
   - Create /metrics/cost endpoint
   - Budget alerts

### Phase 2: Quality Foundation (Medium Complexity, Critical Value)
3. **Vibe Scorecard** - Both agree this is foundational
   - LLM-as-Judge evaluation (0-10 scores)
   - Dimensions: correctness, efficiency, maintainability, security
   - Store scores for analytics

4. **Feedback Loop + Auto-Retry** - Depends on Vibe Scorecard
   - Retry when quality < threshold
   - Include previous attempt context in retry
   - Max N retries with backoff

### Phase 3: Process Discipline (Higher Complexity)
5. **TDD Enforcement** - Both rate as valuable but complex
   - QA agent generates failing tests first
   - Implementation must pass tests
   - Tests = executable requirements

### Future Consideration
6. **RAG Integration** - NOW FEASIBLE via AriClaudRAG
   - Existing Milvus infrastructure already running (24K+ bookmarks indexed)
   - Reuse `rag_retriever.py` from `/Users/arielspivakovsky/src/LocalRagAndSearch/AriClaudRAG/`
   - Key components: all-MiniLM-L6-v2 embeddings, hybrid retrieval, Redis caching
   - **Integration path:** Wrap as microservice, ingest FLIP docs, inject context into prompts
   - Estimated effort: 6 days (vs original "heavy" estimate)

---

## Implementation Status

- [x] Phase 1.1: Capability Registry ✅ DONE
- [ ] Phase 1.2: RAG Integration ← **PROMOTED** (Antigravity recommendation)
- [ ] Phase 1.3: Cost Tracking
- [ ] Phase 2.1: Vibe Scorecard
- [ ] Phase 2.2: Feedback Loop
- [ ] Phase 3: TDD Enforcement

### Antigravity's Recommendation (2025-12-11)
> "RAG gives Agents 'Project Memory' - highest value multiplier. Cost Tracking is objective quick win. Vibe Scorecard is subjective - defer."

---

## Key Insight

> "FLIP v5 already has strong infrastructure (WebSocket hub, worker pools, knowledge persistence). What it lacks are the **quality assurance mechanisms** that ensure agent outputs meet standards before affecting downstream work."
> — Claude Opus Architecture Review

The consensus: Start with **Capability Registry** (quickest win), then build **Vibe Scorecard** (foundational for quality), then add **Feedback Loop** (autonomous improvement).
