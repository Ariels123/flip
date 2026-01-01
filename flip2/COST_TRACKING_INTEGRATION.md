# Cost Tracking Integration - Complete

## Overview
The cost tracking system has been successfully integrated with the FLIP2 daemon to automatically record LLM API costs.

## Components Integrated

### 1. Daemon (`internal/daemon/daemon.go`)
- Added `costTracker *costtracker.Tracker` field to Daemon struct (line 63)
- Initialization happens in `initializeFLIP2API()` (line 1401-1404)
- PocketBase store is created and passed to the tracker
- Cost tracker is passed to API handlers during initialization (line 1418)

### 2. API Handlers (`internal/api/handlers.go`)
- Updated `NewAPIHandlers()` to accept cost tracker parameter (line 27)
- Added `costTracker` field to APIHandlers struct (line 23)
- Integrated cost recording after LLM execution in `HandleInvokeLLM()` (lines 401-422)
- Added cost in response to API calls (line 430)

### 3. Cost Recording Flow
When an LLM API call is made through `/api/llm/invoke`:

1. Request is received with prompt and optional backend/model
2. Backend executes the LLM call
3. Response includes: content, model, input_tokens, output_tokens, cost_usd
4. Cost is recorded via `costTracker.RecordCost()` with:
   - agent_id (from X-Agent-ID header or "api")
   - model name
   - task_id (empty string for direct API calls)
   - input_tokens
   - output_tokens
   - cost_usd
5. Cost record is saved to PocketBase `costs` collection
6. Response is returned to client

### 4. Database Schema
The `costs` collection (created by migration `8_add_costs_collection.go`) includes:
- agent_id (text, required)
- model (text, required)
- input_tokens (number, required, min: 0)
- output_tokens (number, required, min: 0)
- cost_usd (number, required, min: 0)
- task_id (text, optional)
- timestamp (date, required)

Indexes for efficient queries:
- idx_costs_agent_timestamp
- idx_costs_model_timestamp
- idx_costs_timestamp

### 5. API Endpoints for Cost Analytics

**Get Cost Summary:**
```bash
GET /api/stats/summary?days=7
```
Returns total costs, tokens, and record count for the specified period.

**Get Costs by Agent:**
```bash
GET /api/stats/costs/agent/{agent_id}?days=30
```
Returns all cost records for a specific agent.

**Get Costs by Model:**
```bash
GET /api/stats/costs/model/{model}?days=30
```
Returns all cost records for a specific model.

## Error Handling
- Cost recording errors are logged but don't fail the LLM request
- This ensures cost tracking is non-critical to core functionality
- If cost tracker is unavailable, LLM requests still succeed

## Token Cost Calculation
Costs are calculated using model-specific pricing in `internal/llm/process.go`:

### Claude Models:
- claude-sonnet-4: $3.00 input / $15.00 output (per 1M tokens)
- claude-opus-4: $15.00 input / $75.00 output (per 1M tokens)
- claude-3-5-sonnet: $3.00 input / $15.00 output (per 1M tokens)
- claude-3-5-haiku: $0.25 input / $1.25 output (per 1M tokens)

### Gemini Models:
- gemini-2.5-flash: $0.075 input / $0.30 output (per 1M tokens)
- gemini-2.5-pro: $1.25 input / $10.00 output (per 1M tokens)

## Testing
To verify the integration works:

1. Start the daemon:
```bash
./flip2d start
```

2. Make an LLM API call:
```bash
curl -X POST http://localhost:8090/api/llm/invoke \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -H "X-Agent-ID: test-agent" \
  -d '{
    "prompt": "Hello, world!",
    "backend": "gemini"
  }'
```

3. Check cost summary:
```bash
curl http://localhost:8090/api/stats/summary?days=1 \
  -H "X-API-Key: your-api-key"
```

Expected response:
```json
{
  "total_cost_usd": 0.00015,
  "total_input_tokens": 3,
  "total_output_tokens": 5,
  "record_count": 1
}
```

## Success Criteria
✅ Cost tracker initialized in daemon
✅ Costs recorded after every LLM API call
✅ Code compiles successfully
✅ No race conditions introduced
✅ Error handling preserves API functionality
✅ Cost analytics endpoints available

## Files Modified
- `/Users/arielspivakovsky/src/flip/flip2/internal/daemon/daemon.go`
- `/Users/arielspivakovsky/src/flip/flip2/internal/api/handlers.go`
- `/Users/arielspivakovsky/src/flip/flip2/internal/api/routes.go`

## Files Used (No Changes Required)
- `/Users/arielspivakovsky/src/flip/flip2/internal/costtracker/costtracker.go`
- `/Users/arielspivakovsky/src/flip/flip2/internal/costtracker/pbstore.go`
- `/Users/arielspivakovsky/src/flip/flip2/pb_migrations/8_add_costs_collection.go`
