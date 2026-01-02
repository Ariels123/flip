package errors

import (
	"fmt"
	"time"
)

// ErrorAction represents the action to take in response to an error.
type ErrorAction string

const (
	// ActionRetry indicates the operation should be retried, possibly with backoff.
	ActionRetry ErrorAction = "retry"

	// ActionFallback indicates an alternative strategy or backend should be used.
	ActionFallback ErrorAction = "fallback"

	// ActionAbort indicates the operation should stop immediately without further retry.
	ActionAbort ErrorAction = "abort"

	// ActionEscalate indicates the error requires user or operator intervention.
	ActionEscalate ErrorAction = "escalate"
)

// String returns the human-readable name of the error action.
func (a ErrorAction) String() string {
	return string(a)
}

// RoutingDecision encapsulates the decision made by the error router.
type RoutingDecision struct {
	// Action is the recommended action to take.
	Action ErrorAction

	// Reason is a human-readable explanation of why this action was chosen.
	Reason string

	// RetryDelay is the recommended delay before retrying (only relevant for ActionRetry).
	RetryDelay time.Duration

	// MaxRetries is the maximum number of retries recommended (only relevant for ActionRetry).
	MaxRetries int

	// BackendFallback is the fallback backend name if ActionFallback is chosen.
	// Empty string means no specific fallback backend is recommended.
	BackendFallback string
}

// errorRoutingRules defines the routing rules for different error codes.
// These rules determine what action to take based on the error type.
type errorRoutingRules struct {
	code        ErrorCode
	action      ErrorAction
	reason      string
	retryDelay  time.Duration
	maxRetries  int
	fallback    string
	needsRetry  bool
}

// decisionMatrix defines the routing decision matrix for all error codes.
// This matrix encodes the business logic for error handling:
//   - Timeout errors retry with exponential backoff
//   - Quota errors fallback to alternative backends
//   - Not found errors abort immediately
//   - Permission errors escalate to user
//   - Execution failures retry once then abort
//   - Config errors escalate to user
//   - Canceled operations abort immediately
var decisionMatrix = []errorRoutingRules{
	{
		code:       ErrTimeout,
		action:     ActionRetry,
		reason:     "Timeout is a transient error; retrying with backoff may succeed",
		retryDelay: 1 * time.Second,
		maxRetries: 3,
		fallback:   "",
		needsRetry: true,
	},
	{
		code:       ErrQuotaExhausted,
		action:     ActionFallback,
		reason:     "Quota exhausted; switch to alternative backend or rate-limited queue",
		retryDelay: 0,
		maxRetries: 0,
		fallback:   "alternative",
		needsRetry: true,
	},
	{
		code:       ErrNotFound,
		action:     ActionAbort,
		reason:     "Resource or executable not found; fixing configuration is required",
		retryDelay: 0,
		maxRetries: 0,
		fallback:   "",
		needsRetry: false,
	},
	{
		code:       ErrPermissionDenied,
		action:     ActionEscalate,
		reason:     "Permission denied; user or operator intervention is required",
		retryDelay: 0,
		maxRetries: 0,
		fallback:   "",
		needsRetry: false,
	},
	{
		code:       ErrExecutionFailed,
		action:     ActionRetry,
		reason:     "Execution failed; attempting single retry before abort",
		retryDelay: 500 * time.Millisecond,
		maxRetries: 1,
		fallback:   "",
		needsRetry: true,
	},
	{
		code:       ErrInvalidConfig,
		action:     ActionEscalate,
		reason:     "Invalid configuration; user intervention required to fix config",
		retryDelay: 0,
		maxRetries: 0,
		fallback:   "",
		needsRetry: false,
	},
	{
		code:       ErrCanceled,
		action:     ActionAbort,
		reason:     "Operation was explicitly canceled; no retry or fallback",
		retryDelay: 0,
		maxRetries: 0,
		fallback:   "",
		needsRetry: false,
	},
}

// RouteError determines the action to take based on an error.
// Returns a RoutingDecision that specifies what action should be taken.
//
// The routing logic follows these principles:
//   - Transient errors (timeout, quota) are retried with appropriate strategies
//   - Permanent errors (not found, permission denied) escalate or abort
//   - Configuration errors require user intervention (escalate)
//   - Canceled operations are aborted
func RouteError(err error) RoutingDecision {
	// If the error is not an ExecutionError, default to abort with escalation
	execErr, ok := err.(*ExecutionError)
	if !ok {
		return RoutingDecision{
			Action: ActionEscalate,
			Reason: "Unknown error type; escalating for investigation",
		}
	}

	// Look up the routing rule for this error code
	for _, rule := range decisionMatrix {
		if rule.code == execErr.Code {
			return RoutingDecision{
				Action:          rule.action,
				Reason:          rule.reason,
				RetryDelay:      rule.retryDelay,
				MaxRetries:      rule.maxRetries,
				BackendFallback: rule.fallback,
			}
		}
	}

	// If no specific rule found, default to escalation
	return RoutingDecision{
		Action: ActionEscalate,
		Reason: fmt.Sprintf("Unknown error code %q; escalating for investigation", execErr.Code),
	}
}

// RouteErrorWithRetryCount determines the action to take considering the current retry count.
// This is useful when the router is called multiple times for the same error.
//
// Behavior:
//   - If retry count exceeds max retries, recommends abort instead of retry
//   - Otherwise follows standard routing rules
func RouteErrorWithRetryCount(err error, currentRetryCount int) RoutingDecision {
	decision := RouteError(err)

	// If the decision is to retry but we've hit the max retry limit, switch to abort
	if decision.Action == ActionRetry && currentRetryCount >= decision.MaxRetries {
		return RoutingDecision{
			Action: ActionAbort,
			Reason: fmt.Sprintf("Maximum retry attempts (%d) exceeded", decision.MaxRetries),
		}
	}

	return decision
}

// DecisionMatrix returns the complete routing decision matrix.
// This is useful for testing, documentation, and debugging routing logic.
func DecisionMatrix() []RoutingDecision {
	var decisions []RoutingDecision
	for _, rule := range decisionMatrix {
		decisions = append(decisions, RoutingDecision{
			Action:          rule.action,
			Reason:          rule.reason,
			RetryDelay:      rule.retryDelay,
			MaxRetries:      rule.maxRetries,
			BackendFallback: rule.fallback,
		})
	}
	return decisions
}

// ErrorCodeRoutingMap returns a mapping of error codes to their routing actions.
// Useful for quick lookups and test assertions.
func ErrorCodeRoutingMap() map[ErrorCode]ErrorAction {
	m := make(map[ErrorCode]ErrorAction)
	for _, rule := range decisionMatrix {
		m[rule.code] = rule.action
	}
	return m
}
