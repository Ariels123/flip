# Feedback Loop + Auto-Retry Design

## Overview

Automatic quality improvement system that evaluates task results, provides feedback, and retries failed tasks with improvement suggestions incorporated.

## Architecture

```
Task Completion
      ↓
[Vibe Scorecard Evaluation]
      ↓
  Score >= Threshold? ────YES──→ ✅ Success (Store scorecard)
      ↓
     NO
      ↓
  Retry Count < Max? ────NO───→ ❌ Failed (Alert + Store scorecard)
      ↓
     YES
      ↓
[Extract Improvement Suggestions]
      ↓
[Retry Task with Feedback]
      ↓
[Task Completion] (loop back)
```

## Components

### 1. Automatic Evaluation Hook

**Trigger**: After task status changes to 'completed'

**Location**: `internal/daemon/daemon.go` - Add hook to executor's task completion handler

**Logic**:
```go
func (d *Daemon) onTaskCompleted(taskID string) {
    // Fetch task details
    task := d.fetchTask(taskID)

    // Evaluate result quality
    scorecard, err := d.vibeEvaluator.EvaluateTask(
        ctx,
        task.ID,
        task.Title,
        task.Result,
        task.RetryCount,
        task.PreviousScoreID,
    )

    // Save scorecard
    d.saveScorecard(scorecard)

    // Check if retry needed
    if scorecard.IsFail() && task.RetryCount < d.config.MaxRetries {
        d.retryTaskWithFeedback(task, scorecard)
    } else if scorecard.IsFail() {
        d.alertMaxRetriesExceeded(task, scorecard)
    }
}
```

### 2. Retry Decision Logic

**Criteria for Retry**:
- `scorecard.Status == StatusFail` (overall_score < threshold)
- `task.RetryCount < maxRetries` (default: 3)
- Task type supports retry (not all tasks can be retried)

**Skip Retry For**:
- Read-only tasks (queries, searches)
- Human-in-loop tasks
- External API calls (may not be idempotent)

### 3. Feedback Injection

**Approach**: Augment task prompt with previous scorecard feedback

**Implementation**:
```go
func (d *Daemon) buildRetryPrompt(originalTask Task, scorecard *VibeScoreCard) string {
    feedback := fmt.Sprintf(`
PREVIOUS ATTEMPT EVALUATION:
Overall Score: %.1f/10 (Threshold: %.1f)
Status: %s

FEEDBACK BY DIMENSION:
- Correctness (%.1f/10): %s
- Efficiency (%.1f/10): %s
- Maintainability (%.1f/10): %s
- Security (%.1f/10): %s

IMPROVEMENT SUGGESTIONS:
%s

ORIGINAL TASK:
%s

Please retry this task, addressing the feedback above. Focus specifically on the improvement suggestions.
`,
        scorecard.OverallScore,
        scorecard.QualityThreshold,
        scorecard.Status,
        scorecard.Correctness, scorecard.CorrectnessFeedback,
        scorecard.Efficiency, scorecard.EfficiencyFeedback,
        scorecard.Maintainability, scorecard.MaintainabilityFeedback,
        scorecard.Security, scorecard.SecurityFeedback,
        strings.Join(scorecard.ImprovementSuggestions, "\n- "),
        originalTask.Description,
    )

    return feedback
}
```

### 4. Retry Tracking

**Task Table Updates**:
```sql
ALTER TABLE tasks ADD COLUMN retry_count INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN original_task_id TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN previous_score_id TEXT DEFAULT '';
```

**Tracking Chain**:
```
Original Task (task_001)
    → Retry 1 (task_001_retry_1) [retry_count=1, original_task_id=task_001, previous_score_id=score_001]
    → Retry 2 (task_001_retry_2) [retry_count=2, original_task_id=task_001, previous_score_id=score_002]
    → Retry 3 (task_001_retry_3) [retry_count=3, original_task_id=task_001, previous_score_id=score_003]
```

**Vibe Scorecard Linking**:
Each scorecard stores:
- `previous_score_id`: Links to previous attempt's scorecard
- `retry_count`: Attempt number (0 = first attempt)

### 5. Configuration

**New Config Section** (`config.yaml`):
```yaml
flip2:
  feedback_loop:
    enabled: true
    max_retries: 3
    quality_threshold: 6.0
    retry_delay: 5s  # Wait before retrying
    exclude_task_types: ["search", "query", "human_task"]
    notify_on_max_retries: true
```

### 6. Alerting

**Max Retries Exceeded Alert**:
```go
func (d *Daemon) alertMaxRetriesExceeded(task Task, finalScorecard *VibeScoreCard) {
    alert := Alert{
        Name: "task_max_retries_exceeded",
        Severity: "high",
        Message: fmt.Sprintf(
            "Task '%s' failed after %d attempts. Final score: %.1f/10",
            task.Title,
            task.RetryCount,
            finalScorecard.OverallScore,
        ),
        Context: map[string]interface{}{
            "task_id": task.ID,
            "retry_count": task.RetryCount,
            "final_score": finalScorecard.OverallScore,
            "scorecard_id": finalScorecard.ID,
        },
    }

    d.alertManager.CreateAlert(alert)
}
```

## Database Schema Changes

### Tasks Table Migration

```go
// pb_migrations/13_add_feedback_loop_fields.go
func init() {
    m.Register(func(app core.App) error {
        collection, err := app.FindCollectionByNameOrId("tasks")
        if err != nil {
            return err
        }

        // Add retry tracking fields
        collection.Fields.Add(&core.NumberField{
            Name: "retry_count",
            Min:  types.Pointer(0.0),
            Max:  types.Pointer(10.0),
        })

        collection.Fields.Add(&core.TextField{
            Name: "original_task_id",
        })

        collection.Fields.Add(&core.TextField{
            Name: "previous_score_id",
        })

        collection.Fields.Add(&core.BoolField{
            Name: "auto_retry_enabled",
        })

        return app.Save(collection)
    }, func(app core.App) error {
        // Rollback logic
        return nil
    })
}
```

## Implementation Steps

### Phase 1: Basic Evaluation Hook (1-2 hours)
1. Add task completion hook in executor
2. Trigger Vibe Scorecard evaluation after task completes
3. Save scorecard to database
4. Log evaluation results

### Phase 2: Retry Logic (2-3 hours)
1. Implement retry decision logic (check score, retry count)
2. Build feedback-augmented prompt
3. Create retry task with updated prompt
4. Link retry to original task and previous scorecard

### Phase 3: Configuration & Limits (1 hour)
1. Add config section for feedback loop
2. Implement max retry limit
3. Add exclude list for task types
4. Add retry delay mechanism

### Phase 4: Alerting & Monitoring (1 hour)
1. Create max retries exceeded alert
2. Add dashboard panel for retry statistics
3. Log retry attempts and outcomes

### Phase 5: Testing (2 hours)
1. Create intentionally low-quality test tasks
2. Verify automatic evaluation triggers
3. Confirm retries occur with feedback
4. Validate max retry limit works
5. Check alert triggers correctly

## Success Metrics

- **Auto-Improvement Rate**: % of tasks that pass after retry
- **Retry Distribution**: How many tasks need 1, 2, 3+ retries
- **Score Progression**: Average score improvement per retry
- **Time to Success**: Average time from first attempt to passing
- **Max Retry Rate**: % of tasks that hit max retry limit

## Example Flow

### Scenario: Low-Quality Code Task

**Attempt 1**:
- Task: "Create a function to validate email addresses"
- Result: `func ValidateEmail(email string) bool { return true }`
- Evaluation: Correctness=3/10, Overall=4.5/10 (FAIL)
- Feedback: "Function always returns true, doesn't validate email format"

**Attempt 2** (Auto-Retry):
- Augmented Prompt: "[Previous feedback] + Original task"
- Result: `func ValidateEmail(email string) bool { return strings.Contains(email, "@") }`
- Evaluation: Correctness=6/10, Overall=6.5/10 (PASS)
- Outcome: ✅ Task succeeded after 1 retry

### Scenario: Persistent Low Quality

**Attempt 1**: Score=4.2/10 (FAIL) → Retry 1
**Attempt 2**: Score=5.1/10 (FAIL) → Retry 2
**Attempt 3**: Score=5.5/10 (FAIL) → Max retries exceeded
**Alert**: "Task XYZ failed after 3 attempts, human intervention required"

## Future Enhancements

1. **Adaptive Prompts**: Learn which feedback phrases are most effective
2. **Smart Retries**: Use different LLM backends for retries (e.g., Opus for hard tasks)
3. **Human Escalation**: Automatically request human review after max retries
4. **Feedback Caching**: Reuse common feedback patterns across similar tasks
5. **A/B Testing**: Compare retry strategies to optimize success rate

---

**Status**: Design Complete
**Next**: Implement Phase 1 (Basic Evaluation Hook)
