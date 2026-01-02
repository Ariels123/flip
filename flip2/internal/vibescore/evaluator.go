package vibescore

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// LLMBackend interface for LLM invocation
type LLMBackend interface {
	Execute(ctx context.Context, prompt string, opts interface{}) (*LLMResponse, error)
	Name() string
}

// LLMResponse from backend
type LLMResponse struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Latency      time.Duration
}

// Evaluator performs LLM-as-Judge quality evaluation
type Evaluator struct {
	backend  LLMBackend
	logger   *slog.Logger
	threshold float64
}

// NewEvaluator creates a new quality evaluator
func NewEvaluator(backend LLMBackend, threshold float64, logger *slog.Logger) *Evaluator {
	if logger == nil {
		logger = slog.Default()
	}
	if threshold == 0 {
		threshold = 6.0 // Default threshold
	}
	return &Evaluator{
		backend:  backend,
		logger:   logger.With("component", "vibescore-evaluator"),
		threshold: threshold,
	}
}

// EvaluateTask performs quality evaluation on a completed task
func (e *Evaluator) EvaluateTask(ctx context.Context, taskID, taskTitle, taskResult string, retryCount int, previousScoreID string) (*VibeScoreCard, error) {
	e.logger.Info("Evaluating task", "task_id", taskID, "retry_count", retryCount)

	// Build evaluation prompt
	prompt := e.buildEvaluationPrompt(taskTitle, taskResult)

	// Invoke LLM
	response, err := e.backend.Execute(ctx, prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("LLM execution failed: %w", err)
	}

	// Parse response into scorecard
	scorecard, err := e.parseEvaluationResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse evaluation: %w", err)
	}

	// Fill in metadata
	scorecard.TaskID = taskID
	scorecard.Evaluator = e.backend.Name()
	scorecard.EvaluatorModel = response.Model
	scorecard.EvaluatedAt = time.Now()
	scorecard.QualityThreshold = e.threshold
	scorecard.RetryCount = retryCount
	scorecard.PreviousScoreID = previousScoreID

	// Calculate overall score
	scorecard.OverallScore = (scorecard.Correctness + scorecard.Efficiency + scorecard.Maintainability + scorecard.Security) / 4.0

	// Determine status
	if scorecard.OverallScore >= e.threshold {
		scorecard.Status = StatusPass
	} else {
		scorecard.Status = StatusFail
	}

	e.logger.Info("Evaluation complete",
		"task_id", taskID,
		"overall_score", scorecard.OverallScore,
		"status", scorecard.Status,
		"evaluator", scorecard.Evaluator,
	)

	return scorecard, nil
}

// buildEvaluationPrompt creates the LLM-as-Judge prompt
func (e *Evaluator) buildEvaluationPrompt(taskTitle, taskResult string) string {
	return fmt.Sprintf(`You are an expert code quality evaluator. Evaluate the following task completion on 4 dimensions using a 0-10 scale.

TASK: %s

RESULT:
%s

Evaluate on these dimensions:
1. CORRECTNESS (0-10): Does the code work correctly? Are there bugs or logic errors?
2. EFFICIENCY (0-10): Is the code performant? Any unnecessary complexity or slow operations?
3. MAINTAINABILITY (0-10): Is the code clean, readable, and well-organized? Easy to modify later?
4. SECURITY (0-10): Are there security vulnerabilities? Proper input validation and error handling?

Provide your evaluation in this EXACT JSON format:
{
  "correctness": 8.5,
  "correctness_feedback": "Brief explanation of correctness score",
  "efficiency": 7.0,
  "efficiency_feedback": "Brief explanation of efficiency score",
  "maintainability": 9.0,
  "maintainability_feedback": "Brief explanation of maintainability score",
  "security": 8.0,
  "security_feedback": "Brief explanation of security score",
  "summary_feedback": "Overall summary in 2-3 sentences",
  "improvement_suggestions": [
    "Specific suggestion 1",
    "Specific suggestion 2"
  ]
}

Be objective and constructive. Focus on actionable feedback.`, taskTitle, taskResult)
}

// parseEvaluationResponse extracts structured scorecard from LLM response
func (e *Evaluator) parseEvaluationResponse(content string) (*VibeScoreCard, error) {
	// Extract JSON from response (may have markdown code blocks)
	jsonStr := e.extractJSON(content)

	// Parse into temporary struct
	var parsed struct {
		Correctness             float64  `json:"correctness"`
		CorrectnessFeedback     string   `json:"correctness_feedback"`
		Efficiency              float64  `json:"efficiency"`
		EfficiencyFeedback      string   `json:"efficiency_feedback"`
		Maintainability         float64  `json:"maintainability"`
		MaintainabilityFeedback string   `json:"maintainability_feedback"`
		Security                float64  `json:"security"`
		SecurityFeedback        string   `json:"security_feedback"`
		SummaryFeedback         string   `json:"summary_feedback"`
		ImprovementSuggestions  []string `json:"improvement_suggestions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	// Create scorecard
	scorecard := &VibeScoreCard{
		Correctness:             parsed.Correctness,
		CorrectnessFeedback:     parsed.CorrectnessFeedback,
		Efficiency:              parsed.Efficiency,
		EfficiencyFeedback:      parsed.EfficiencyFeedback,
		Maintainability:         parsed.Maintainability,
		MaintainabilityFeedback: parsed.MaintainabilityFeedback,
		Security:                parsed.Security,
		SecurityFeedback:        parsed.SecurityFeedback,
		SummaryFeedback:         parsed.SummaryFeedback,
		ImprovementSuggestions:  parsed.ImprovementSuggestions,
	}

	return scorecard, nil
}

// extractJSON finds and extracts JSON from response (handles markdown code blocks)
func (e *Evaluator) extractJSON(content string) string {
	// Remove markdown code blocks if present
	content = strings.TrimSpace(content)

	// Check for ```json blocks
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Check for ``` blocks
	if strings.HasPrefix(content, "```") {
		start := strings.Index(content, "\n") + 1
		end := strings.Index(content[start:], "```")
		if end > 0 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Find JSON object by braces
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
	}

	return content
}
