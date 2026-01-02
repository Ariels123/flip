# Vibe Scorecard Package

Quality evaluation system for assessing task outputs across four dimensions: correctness, efficiency, maintainability, and security.

## Quick Start

```go
package main

import (
    "time"
    "flip2/internal/vibescore"
)

// Create a scorecard
card := &vibescore.VibeScoreCard{
    TaskID:              "task_123",
    Correctness:         8.5,
    Efficiency:          7.0,
    Maintainability:     8.0,
    Security:            8.5,
    Evaluator:           "claude",
    EvaluatorModel:      "claude-opus-4-5",
    EvaluatedAt:         time.Now(),
    CorrectnessFeedback: "All tests pass",
    SummaryFeedback:     "High-quality implementation",
    QualityThreshold:    6.0,
}

// Calculate and determine status
card.OverallScore = card.CalculateOverallScore()
card.Status = card.DetermineStatus()

if card.IsPass() {
    // Score is >= 6.0, accept result
}
```

## Core Types

### VibeScoreCard

Main struct representing a complete quality evaluation.

**Key Methods:**

- `CalculateOverallScore()` - Compute average of 4 dimensions
- `DetermineStatus()` - Set status based on score vs threshold
- `IsPass()` / `IsFail()` / `NeedsReview()` - Check status
- `HasIssues()` - True if any dimension < 5.0
- `GetDimensionScores()` - Map of all scores
- `GetFeedbackMap()` - Map of all feedback
- `ToSummary()` - Convert to lightweight ScoreSummary

### Status

Enum: `"pass" | "fail" | "needs_review"`

```go
if card.Status == vibescore.StatusPass {
    // overall_score >= quality_threshold
}
```

### Evaluator

Which LLM performed the evaluation.

```go
const (
    EvaluatorClaude      Evaluator = "claude"
    EvaluatorGemini      Evaluator = "gemini"
    EvaluatorAntigravity Evaluator = "antigravity"
    EvaluatorCustom      Evaluator = "custom"
)
```

## Scoring Dimensions

### 1. Correctness (0-10)
Does it work correctly and meet requirements?
- 10: Perfect
- 5: Mostly works
- 0: Broken

### 2. Efficiency (0-10)
Is it performant and optimized?
- 10: Optimal complexity
- 5: Acceptable performance
- 0: Extremely slow

### 3. Maintainability (0-10)
Is it clean and maintainable?
- 10: Excellent documentation and structure
- 5: Adequate quality
- 0: Unreadable code

### 4. Security (0-10)
Are there security vulnerabilities?
- 10: No security issues
- 5: Some concerns
- 0: Critical vulnerabilities

## Usage Examples

### Basic Evaluation

```go
card := &vibescore.VibeScoreCard{
    TaskID:         "task_001",
    Correctness:    9.0,
    Efficiency:     8.5,
    Maintainability: 8.0,
    Security:       9.5,
    Evaluator:      "claude",
    EvaluatedAt:    time.Now(),
}

card.OverallScore = card.CalculateOverallScore() // 8.75
card.Status = card.DetermineStatus()             // StatusPass
```

### Feedback Collection

```go
card := &vibescore.VibeScoreCard{
    // ... scores ...
    CorrectnessFeedback:     "All edge cases handled correctly",
    EfficiencyFeedback:      "O(n log n) complexity is optimal",
    MaintainabilityFeedback: "Clear variable names and good structure",
    SecurityFeedback:        "Input validation and SQL parameterization present",
    SummaryFeedback:         "High-quality implementation",
    ImprovementSuggestions: []string{
        "Add type hints for better IDE support",
        "Consider async/await for I/O operations",
    },
}
```

### Retry Tracking

```go
// First attempt
firstScore := &vibescore.VibeScoreCard{
    // ... scores ...
    Correctness: 4.0, // Failed test cases
}
firstScore.OverallScore = firstScore.CalculateOverallScore() // 4.5
firstScore.Status = firstScore.DetermineStatus()             // StatusFail

// Store in database, get ID...
firstScoreID := "score_001"

// Retry with feedback
retryScore := &vibescore.VibeScoreCard{
    TaskID:          "task_001",
    Correctness:     8.5,
    Efficiency:      8.0,
    Maintainability: 8.5,
    Security:        8.0,
    RetryCount:      1,
    PreviousScoreID: firstScoreID,
    SummaryFeedback: "Fixed failing test cases and improved error handling",
}
retryScore.OverallScore = retryScore.CalculateOverallScore() // 8.25
retryScore.Status = retryScore.DetermineStatus()             // StatusPass
```

### Checking Issues

```go
card := &vibescore.VibeScoreCard{
    Correctness:     5.0,
    Efficiency:      4.0, // Below 5.0
    Maintainability: 7.0,
    Security:        6.0,
}

if card.HasIssues() {
    // At least one dimension is below 5.0
    // Log for investigation
}
```

### Getting Summary

```go
card := &vibescore.VibeScoreCard{
    // ... full card ...
}

summary := card.ToSummary()
// {
//   TaskID: "task_001",
//   OverallScore: 8.0,
//   Status: "pass",
//   EvaluatedAt: <time>,
//   Evaluator: "claude",
//   DimensionScores: {
//     "correctness": 8.5,
//     "efficiency": 7.0,
//     "maintainability": 8.0,
//     "security": 8.5,
//   }
// }
```

## Database Integration

### Create Record

```go
// After populating VibeScoreCard fields:
card.OverallScore = card.CalculateOverallScore()
card.Status = card.DetermineStatus()

// Save to PocketBase collection "vibescore"
response, err := app.Record("vibescore", card)
```

### Query Latest Evaluation

```go
// Get latest scorecard for a task
records, err := app.FindRecordsByFilter(
    "vibescore",
    "task_id = ?",
    "evaluated_at",
    100,
    1,
    "task_123",
)

if len(records) > 0 {
    latestScore := records[0] // Already sorted by evaluated_at
}
```

### Query by Status

```go
// Find all failed evaluations
failedRecords, err := app.FindRecordsByFilter(
    "vibescore",
    "status = ?",
    "overall_score",
    100,
    0,
    vibescore.StatusFail,
)
```

## Threshold Configuration

Default quality threshold: **6.0/10**

### Custom Thresholds

```go
// Strict for critical tasks
card := &vibescore.VibeScoreCard{
    // ... scores ...
    QualityThreshold: 8.0,
}
card.Status = card.DetermineStatus() // Will fail unless score >= 8.0

// Permissive for experiments
card.QualityThreshold = 4.0
card.Status = card.DetermineStatus() // Will pass unless score < 4.0
```

## Testing

### Unit Tests

```go
func TestOverallScoreCalculation(t *testing.T) {
    card := &vibescore.VibeScoreCard{
        Correctness:     8.0,
        Efficiency:      8.0,
        Maintainability: 8.0,
        Security:        8.0,
    }

    score := card.CalculateOverallScore()
    if score != 8.0 {
        t.Fatalf("expected 8.0, got %v", score)
    }
}

func TestPassStatus(t *testing.T) {
    card := &vibescore.VibeScoreCard{
        OverallScore:    7.0,
        QualityThreshold: 6.0,
    }

    status := card.DetermineStatus()
    if status != vibescore.StatusPass {
        t.Fatalf("expected pass, got %v", status)
    }
}
```

## Constants

```go
// Status values
const (
    StatusPass        Status = "pass"         // Score >= threshold
    StatusFail        Status = "fail"         // Score < threshold
    StatusNeedsReview Status = "needs_review" // Manual review needed
)

// Evaluator types
const (
    EvaluatorClaude      Evaluator = "claude"
    EvaluatorGemini      Evaluator = "gemini"
    EvaluatorAntigravity Evaluator = "antigravity"
    EvaluatorCustom      Evaluator = "custom"
)
```

## Schema Reference

See `/docs/VIBESCORE_DESIGN.md` for complete schema documentation including:
- All field definitions
- Database indexes
- Sample queries
- API endpoint specifications

## Related Documentation

- **Design Doc:** `docs/VIBESCORE_DESIGN.md`
- **Roadmap:** `FLIP2_ROADMAP_2025-12-31.md` (Phase 6, Section 2)
- **Migration:** `pb_migrations/12_add_vibescore_collection.go`

## Future Enhancements

- Automated LLM-based evaluation service
- Agent performance dashboards
- Automatic retry loop integration
- Quality trend analysis
- Feedback-based agent improvement
