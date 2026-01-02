# FLIP Structured Logging Schema

## Overview

This document defines the standardized logging schema for the FLIP multi-agent system. All logging must follow this schema to ensure consistent, queryable, and analyzable logs across all components.

- **JSON Format**: Production environments (structured, queryable, machine-readable)
- **Text Format**: Development environments (human-readable, line-oriented)

---

## 1. Core Required Fields

Every log entry MUST include these fields:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `timestamp` | ISO 8601 String | UTC timestamp when log was created | `2026-01-01T19:30:45.123Z` |
| `level` | String | Severity level: DEBUG, INFO, WARN, ERROR, FATAL | `INFO` |
| `message` | String | Primary log message (descriptive, actionable) | `Agent task completed successfully` |
| `component` | String | System component that created the log | `agent.coordinator`, `worker.claude1`, `pipeline.research` |

### Field Specifications

**timestamp**
- Must be ISO 8601 UTC format
- Include millisecond precision
- Example: `2026-01-01T19:30:45.123Z`

**level**
- `DEBUG`: Detailed diagnostic information (development only)
- `INFO`: General informational messages
- `WARN`: Warning conditions requiring attention
- `ERROR`: Error conditions but recoverable
- `FATAL`: Critical error, system may crash

**message**
- Concise but descriptive (50-200 characters typical)
- Should be actionable or diagnostic
- Avoid including raw data/stack traces in message field

**component**
- Hierarchical dot notation: `parent.child.grandchild`
- Examples: `coordinator`, `worker.claude`, `pipeline.research`, `api.websocket`

---

## 2. Context Fields

Optionally include these fields to track request flow and agent execution context:

| Field | Type | When to Include | Example |
|-------|------|-----------------|---------|
| `task_id` | String | When task is executing | `task-20260101-abc123` |
| `agent_id` | String | When agent is active | `claude-worker-1` or `gemini-analyzer` |
| `request_id` | String | When handling HTTP/API request | `req-xyz789` |
| `pipeline_id` | String | When part of pipeline execution | `pipeline-research-001` |
| `session_id` | String | When part of broader session | `session-user-001` |
| `parent_task_id` | String | For subtasks, reference parent | `task-parent-abc123` |

### Context Field Usage Patterns

```json
{
  "timestamp": "2026-01-01T19:30:45.123Z",
  "level": "INFO",
  "message": "Task execution started",
  "component": "coordinator",
  "task_id": "task-20260101-research-001",
  "agent_id": "gemini-gatherer",
  "pipeline_id": "pipeline-research-001",
  "session_id": "session-user-001"
}
```

---

## 3. Error Fields

Include these fields when logging errors (level: ERROR or FATAL):

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `error_code` | String | System-defined error code | `ERR_AGENT_TIMEOUT`, `ERR_API_RATE_LIMIT` |
| `error_category` | String | Error classification | `timeout`, `invalid_input`, `rate_limit`, `auth`, `resource` |
| `error_message` | String | Detailed error message | `Agent failed to respond within 30s` |
| `stack_trace` | String | Stack trace (development only, truncate in production) | Multi-line stack trace |
| `error_context` | Object | Additional error details | `{"retry_count": 3, "next_retry_in_ms": 5000}` |

### Error Log Examples

**Timeout Error:**
```json
{
  "timestamp": "2026-01-01T19:30:45.123Z",
  "level": "ERROR",
  "message": "Agent response timeout",
  "component": "coordinator",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "error_code": "ERR_AGENT_TIMEOUT",
  "error_category": "timeout",
  "error_message": "Agent failed to respond within 30000ms",
  "error_context": {
    "timeout_ms": 30000,
    "elapsed_ms": 30150,
    "retry_count": 2,
    "next_retry_in_ms": 5000
  }
}
```

**Invalid Input Error:**
```json
{
  "timestamp": "2026-01-01T19:30:46.234Z",
  "level": "WARN",
  "message": "Invalid task parameter",
  "component": "api.validation",
  "request_id": "req-xyz789",
  "error_code": "ERR_INVALID_INPUT",
  "error_category": "invalid_input",
  "error_message": "Task priority must be 'low', 'medium', or 'high'",
  "error_context": {
    "field": "priority",
    "provided_value": "critical",
    "allowed_values": ["low", "medium", "high"]
  }
}
```

**Rate Limit Error:**
```json
{
  "timestamp": "2026-01-01T19:30:47.345Z",
  "level": "ERROR",
  "message": "API rate limit exceeded",
  "component": "api.client",
  "request_id": "req-abc123",
  "error_code": "ERR_RATE_LIMIT",
  "error_category": "rate_limit",
  "error_message": "Rate limit: 100 requests per minute",
  "error_context": {
    "limit": 100,
    "window_seconds": 60,
    "reset_at": "2026-01-01T19:31:47.000Z",
    "requests_in_window": 100
  }
}
```

---

## 4. Performance Fields

Include these fields when monitoring performance and costs:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `duration_ms` | Number | Execution time in milliseconds | `1250` |
| `cost_usd` | Number | LLM API cost in USD (if applicable) | `0.0045` |
| `tokens_used` | Object | Token usage breakdown | `{"input": 1500, "output": 450}` |
| `cache_hit` | Boolean | Whether result was from cache | `true` |
| `retry_count` | Number | Number of retry attempts | `2` |
| `resource_usage` | Object | Memory, CPU, or other metrics | `{"memory_mb": 256, "cpu_percent": 45}` |

### Performance Log Examples

**Task Completion with Metrics:**
```json
{
  "timestamp": "2026-01-01T19:31:00.567Z",
  "level": "INFO",
  "message": "Task completed successfully",
  "component": "coordinator",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "duration_ms": 28750,
  "cost_usd": 0.0087,
  "tokens_used": {
    "input": 2100,
    "output": 840,
    "cached": 0
  },
  "cache_hit": false
}
```

**Pipeline Execution Metrics:**
```json
{
  "timestamp": "2026-01-01T19:32:15.890Z",
  "level": "INFO",
  "message": "Pipeline execution completed",
  "component": "pipeline.research",
  "pipeline_id": "pipeline-research-001",
  "task_id": "task-pipeline-001",
  "duration_ms": 125000,
  "cost_usd": 0.0234,
  "tokens_used": {
    "input": 15000,
    "output": 4200,
    "cached": 0
  },
  "resource_usage": {
    "memory_mb": 512,
    "cpu_percent": 78
  }
}
```

**Cached Result:**
```json
{
  "timestamp": "2026-01-01T19:33:00.123Z",
  "level": "INFO",
  "message": "Result retrieved from cache",
  "component": "coordinator",
  "task_id": "task-20260101-cached",
  "agent_id": "cache-service",
  "duration_ms": 45,
  "cost_usd": 0.0,
  "cache_hit": true
}
```

---

## 5. Optional Fields

Additional fields for specific scenarios:

| Field | Type | Use Case | Example |
|-------|------|----------|---------|
| `user_id` | String | Track user interactions | `user-12345` |
| `environment` | String | Deployment environment | `development`, `staging`, `production` |
| `version` | String | Software version | `v5.2.1` |
| `metadata` | Object | Arbitrary structured data | `{"feature_flag": "new_research_v2", "ab_test": "control"}` |
| `tags` | Array | For filtering and searching | `["research", "gemini", "priority-high"]` |

---

## 6. Format Examples

### JSON Format (Production)

```json
{
  "timestamp": "2026-01-01T19:30:45.123Z",
  "level": "INFO",
  "message": "Agent task completed successfully",
  "component": "coordinator",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "request_id": "req-xyz789",
  "pipeline_id": "pipeline-research-001",
  "session_id": "session-user-001",
  "environment": "production",
  "version": "v5.2.1",
  "duration_ms": 28750,
  "cost_usd": 0.0087,
  "tokens_used": {
    "input": 2100,
    "output": 840
  },
  "cache_hit": false,
  "tags": ["research", "gemini", "priority-high"]
}
```

### Text Format (Development)

```
2026-01-01T19:30:45.123Z [INFO] coordinator: Agent task completed successfully
  task_id=task-20260101-001
  agent_id=claude-worker-1
  duration_ms=28750
  cost_usd=0.0087
  tokens_input=2100 tokens_output=840
```

### Batch Error Log (Development Text)

```
2026-01-01T19:30:46.234Z [ERROR] coordinator: Agent timeout
  task_id=task-20260101-002
  agent_id=gemini-analyzer
  error_code=ERR_AGENT_TIMEOUT
  timeout_ms=30000
  elapsed_ms=30150
  retry_count=2

2026-01-01T19:30:47.345Z [WARN] api.validation: Invalid parameter
  request_id=req-abc123
  error_code=ERR_INVALID_INPUT
  field=priority
```

---

## 7. Log Levels and Guidelines

### DEBUG
- Use in development environments only
- Include detailed diagnostic information
- Example: function entry/exit, variable values

```json
{
  "timestamp": "2026-01-01T19:30:44.000Z",
  "level": "DEBUG",
  "message": "Parsing task configuration",
  "component": "coordinator.parser",
  "task_id": "task-20260101-001",
  "metadata": {
    "config_keys": ["prompt", "model", "temperature"],
    "validation_state": "in_progress"
  }
}
```

### INFO
- General operational information
- Task starts/completions
- State changes
- Successful operations

```json
{
  "timestamp": "2026-01-01T19:30:45.123Z",
  "level": "INFO",
  "message": "Worker agent connected",
  "component": "coordinator",
  "agent_id": "claude-worker-1"
}
```

### WARN
- Potentially problematic situations
- Retries, timeouts, degraded performance
- Recoverable errors
- Deprecated API usage

```json
{
  "timestamp": "2026-01-01T19:30:46.234Z",
  "level": "WARN",
  "message": "High latency detected",
  "component": "api.client",
  "request_id": "req-xyz789",
  "duration_ms": 15000,
  "error_context": {
    "expected_ms": 5000,
    "percentile": 95
  }
}
```

### ERROR
- Error conditions but system continues
- Agent failures, API errors, validation failures
- Unrecovered retries
- Data consistency issues

```json
{
  "timestamp": "2026-01-01T19:30:47.345Z",
  "level": "ERROR",
  "message": "Agent failed to process task",
  "component": "coordinator",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "error_code": "ERR_AGENT_FAILURE",
  "error_category": "agent_error",
  "error_message": "Agent returned invalid response format"
}
```

### FATAL
- System-critical errors
- Unrecoverable conditions
- Data loss situations
- Security violations

```json
{
  "timestamp": "2026-01-01T19:30:48.456Z",
  "level": "FATAL",
  "message": "Database connection lost",
  "component": "database",
  "error_code": "ERR_DB_CONNECTION",
  "error_category": "resource",
  "error_message": "Cannot establish connection to primary database",
  "error_context": {
    "host": "db.internal",
    "port": 5432,
    "connection_attempts": 5
  }
}
```

---

## 8. Component Hierarchy

Use dot-notation for component names following this hierarchy:

```
coordinator                 # Main coordinator
  ├── agent-manager       # Agent lifecycle management
  ├── task-manager        # Task queuing and scheduling
  ├── parser              # Configuration/prompt parsing
  └── signal-handler      # Signal receiving/sending

worker                      # Worker agent base
  ├── claude              # Claude-specific worker
  ├── gemini              # Gemini-specific worker
  └── antigravity         # Human-in-loop handler

pipeline                    # Pipeline execution
  ├── research            # Research pipeline
  ├── data-analyze        # Data analysis pipeline
  └── code-review         # Code review pipeline

api                         # API layer
  ├── websocket           # WebSocket communication
  ├── http                # HTTP endpoints
  ├── client              # Outbound API client
  └── validation          # Request validation

database                    # Data persistence
  ├── signals             # Signal storage
  ├── tasks               # Task storage
  └── cache               # Result caching

system                      # System-level
  ├── startup             # Initialization
  ├── shutdown            # Cleanup
  ├── health              # Health checks
  └── monitoring          # Performance monitoring
```

---

## 9. Querying Examples

### Query logs by component
```bash
# Find all coordinator logs
SELECT * FROM logs WHERE component LIKE 'coordinator%'

# Find all API errors
SELECT * FROM logs WHERE component LIKE 'api%' AND level = 'ERROR'
```

### Query logs by time range
```bash
# Last 24 hours of errors
SELECT * FROM logs
WHERE level IN ('ERROR', 'FATAL')
AND timestamp > NOW() - INTERVAL '24 hours'
```

### Query logs by task
```bash
# All logs for a specific task
SELECT * FROM logs WHERE task_id = 'task-20260101-001'

# Timeline of a pipeline execution
SELECT * FROM logs
WHERE pipeline_id = 'pipeline-research-001'
ORDER BY timestamp ASC
```

### Cost and performance analysis
```bash
# Total cost by agent
SELECT agent_id, SUM(cost_usd) as total_cost
FROM logs
WHERE tokens_used IS NOT NULL
GROUP BY agent_id

# Average task duration by component
SELECT component, AVG(duration_ms) as avg_ms, COUNT(*) as count
FROM logs
WHERE duration_ms IS NOT NULL
GROUP BY component
```

---

## 10. Implementation Best Practices

### Do's
- Always include timestamp and level
- Use consistent component names
- Include task_id for all task-related logs
- Log at appropriate levels (INFO for normal ops, ERROR for failures)
- Include duration_ms for performance-critical operations
- Use error_code for systematic error handling
- Include context fields to track request flow

### Don'ts
- Don't log sensitive data (passwords, API keys, PII)
- Don't include raw stack traces in production logs
- Don't use inconsistent timestamp formats
- Don't omit error codes for error logs
- Don't log the same event multiple times in a function
- Don't include binary data or very large objects

### Sensitive Data Handling
```json
// BAD - includes API key
{
  "level": "ERROR",
  "message": "API call failed",
  "api_key": "sk-1234567890abcdef",
  "request": "https://api.example.com/v1/models"
}

// GOOD - redacts sensitive data
{
  "level": "ERROR",
  "message": "API call failed",
  "api_key": "sk-[REDACTED]",
  "request_url": "https://api.example.com/v1/models"
}
```

---

## 11. Structured Field Examples by Component

### Coordinator Component

**Task Assignment:**
```json
{
  "timestamp": "2026-01-01T19:30:00.000Z",
  "level": "INFO",
  "message": "Task assigned to worker",
  "component": "coordinator.task-manager",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "metadata": {
    "priority": "high",
    "estimated_duration_s": 60,
    "retry_policy": "exponential_backoff"
  }
}
```

### Worker Component

**Task Processing:**
```json
{
  "timestamp": "2026-01-01T19:30:15.000Z",
  "level": "INFO",
  "message": "Task processing started",
  "component": "worker.claude",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "metadata": {
    "model": "claude-opus-4-5",
    "temperature": 0.7,
    "max_tokens": 4096
  }
}
```

**Task Result:**
```json
{
  "timestamp": "2026-01-01T19:31:30.000Z",
  "level": "INFO",
  "message": "Task completed successfully",
  "component": "worker.claude",
  "task_id": "task-20260101-001",
  "agent_id": "claude-worker-1",
  "duration_ms": 75000,
  "cost_usd": 0.0125,
  "tokens_used": {
    "input": 2500,
    "output": 1200
  },
  "metadata": {
    "result_size_bytes": 45678,
    "quality_score": 0.95
  }
}
```

### Pipeline Component

**Pipeline Execution:**
```json
{
  "timestamp": "2026-01-01T19:30:00.000Z",
  "level": "INFO",
  "message": "Pipeline execution started",
  "component": "pipeline.research",
  "pipeline_id": "pipeline-research-001",
  "session_id": "session-user-001",
  "metadata": {
    "pipeline_type": "research",
    "stages": ["gather", "analyze", "synthesize"],
    "current_stage": "gather"
  }
}
```

### API Component

**HTTP Request:**
```json
{
  "timestamp": "2026-01-01T19:30:10.000Z",
  "level": "INFO",
  "message": "HTTP request received",
  "component": "api.http",
  "request_id": "req-xyz789",
  "metadata": {
    "method": "POST",
    "endpoint": "/api/tasks",
    "status_code": 201,
    "response_time_ms": 250
  }
}
```

---

## 12. Log Rotation and Retention

### Development Environment
- Keep logs in memory for current session
- Write to local file in text format
- Retain 7 days of logs locally
- No compression needed

### Production Environment
- Write all logs to centralized logging service
- JSON format with ISO 8601 timestamps
- Retention policy:
  - INFO level: 30 days
  - WARN level: 60 days
  - ERROR/FATAL level: 90 days
- Compress logs older than 7 days
- Archive to cold storage after 90 days

---

## Summary Table

| Aspect | Required | Optional | Notes |
|--------|----------|----------|-------|
| Timestamp | Yes | - | ISO 8601 UTC format |
| Level | Yes | - | DEBUG, INFO, WARN, ERROR, FATAL |
| Message | Yes | - | Concise, actionable |
| Component | Yes | - | Hierarchical dot notation |
| Task ID | - | Yes | For task-related logs |
| Agent ID | - | Yes | For agent operations |
| Request ID | - | Yes | For API requests |
| Pipeline ID | - | Yes | For pipeline execution |
| Error Code | - | Yes* | Required when level=ERROR or FATAL |
| Duration | - | Yes | For performance tracking |
| Cost | - | Yes | For LLM operations |
| Tokens | - | Yes | For LLM operations |

