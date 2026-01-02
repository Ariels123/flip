package errors

import (
	"fmt"
	"testing"
	"time"
)

// TestRouteErrorTimeoutRetry verifies timeout errors route to retry.
func TestRouteErrorTimeoutRetry(t *testing.T) {
	err := NewTimeout(5*time.Minute, nil)
	decision := RouteError(err)

	if decision.Action != ActionRetry {
		t.Errorf("timeout should route to retry, got %s", decision.Action)
	}
	if decision.MaxRetries == 0 {
		t.Error("timeout should have max retries > 0")
	}
	if decision.RetryDelay == 0 {
		t.Error("timeout should have non-zero retry delay")
	}
}

// TestRouteErrorQuotaFallback verifies quota exhausted errors route to fallback.
func TestRouteErrorQuotaFallback(t *testing.T) {
	resetTime := time.Now().Add(1 * time.Hour)
	err := NewQuotaExhausted("rate limit exceeded", resetTime)
	decision := RouteError(err)

	if decision.Action != ActionFallback {
		t.Errorf("quota exhausted should route to fallback, got %s", decision.Action)
	}
	if decision.BackendFallback == "" {
		t.Error("quota fallback should specify a fallback backend")
	}
}

// TestRouteErrorNotFoundAbort verifies not found errors route to abort.
func TestRouteErrorNotFoundAbort(t *testing.T) {
	err := NewNotFound("executable", nil)
	decision := RouteError(err)

	if decision.Action != ActionAbort {
		t.Errorf("not found should route to abort, got %s", decision.Action)
	}
	if decision.MaxRetries > 0 {
		t.Errorf("not found should have no retries, got %d", decision.MaxRetries)
	}
}

// TestRouteErrorPermissionEscalate verifies permission denied errors route to escalate.
func TestRouteErrorPermissionEscalate(t *testing.T) {
	err := NewPermissionDenied("/bin/executable", nil)
	decision := RouteError(err)

	if decision.Action != ActionEscalate {
		t.Errorf("permission denied should route to escalate, got %s", decision.Action)
	}
	if decision.Reason == "" {
		t.Error("escalation decision should provide a reason")
	}
}

// TestRouteErrorExecutionFailedRetry verifies execution failed errors route to retry once.
func TestRouteErrorExecutionFailedRetry(t *testing.T) {
	err := NewExecutionFailed("process exited with code 1", true, nil)
	decision := RouteError(err)

	if decision.Action != ActionRetry {
		t.Errorf("execution failed should route to retry, got %s", decision.Action)
	}
	if decision.MaxRetries != 1 {
		t.Errorf("execution failed should allow exactly 1 retry, got %d", decision.MaxRetries)
	}
}

// TestRouteErrorInvalidConfigEscalate verifies invalid config errors route to escalate.
func TestRouteErrorInvalidConfigEscalate(t *testing.T) {
	err := NewInvalidConfig("missing backend URL", nil)
	decision := RouteError(err)

	if decision.Action != ActionEscalate {
		t.Errorf("invalid config should route to escalate, got %s", decision.Action)
	}
}

// TestRouteErrorCanceledAbort verifies canceled errors route to abort.
func TestRouteErrorCanceledAbort(t *testing.T) {
	err := NewCanceled("user requested cancellation")
	decision := RouteError(err)

	if decision.Action != ActionAbort {
		t.Errorf("canceled should route to abort, got %s", decision.Action)
	}
}

// TestRouteErrorNonExecutionError verifies non-ExecutionError types route to escalate.
func TestRouteErrorNonExecutionError(t *testing.T) {
	err := fmt.Errorf("some unknown error")
	decision := RouteError(err)

	if decision.Action != ActionEscalate {
		t.Errorf("unknown error should route to escalate, got %s", decision.Action)
	}
}

// TestRouteErrorWithRetryCountAtLimit verifies switching to abort when retry limit hit.
func TestRouteErrorWithRetryCountAtLimit(t *testing.T) {
	err := NewTimeout(5*time.Minute, nil)
	decision := RouteError(err)

	maxRetries := decision.MaxRetries

	// Try with retry count at the limit
	limitDecision := RouteErrorWithRetryCount(err, maxRetries)
	if limitDecision.Action != ActionAbort {
		t.Errorf("exceeded retry limit should route to abort, got %s", limitDecision.Action)
	}
}

// TestRouteErrorWithRetryCountBelowLimit verifies retrying when below limit.
func TestRouteErrorWithRetryCountBelowLimit(t *testing.T) {
	err := NewTimeout(5*time.Minute, nil)

	// Try with retry count below the limit
	decision := RouteErrorWithRetryCount(err, 0)
	if decision.Action != ActionRetry {
		t.Errorf("within retry limit should route to retry, got %s", decision.Action)
	}

	// Try with retry count one below the limit
	maxRetries := RouteError(err).MaxRetries
	decision = RouteErrorWithRetryCount(err, maxRetries-1)
	if decision.Action != ActionRetry {
		t.Errorf("within retry limit should route to retry, got %s", decision.Action)
	}
}

// TestErrorActionString verifies error action string representation.
func TestErrorActionString(t *testing.T) {
	tests := []struct {
		action   ErrorAction
		expected string
	}{
		{ActionRetry, "retry"},
		{ActionFallback, "fallback"},
		{ActionAbort, "abort"},
		{ActionEscalate, "escalate"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestRoutingDecisionReasonPresent verifies all decisions have explanations.
func TestRoutingDecisionReasonPresent(t *testing.T) {
	errorCodes := []ErrorCode{
		ErrTimeout,
		ErrQuotaExhausted,
		ErrNotFound,
		ErrPermissionDenied,
		ErrExecutionFailed,
		ErrInvalidConfig,
		ErrCanceled,
	}

	for _, code := range errorCodes {
		t.Run(string(code), func(t *testing.T) {
			var err error
			switch code {
			case ErrTimeout:
				err = NewTimeout(5*time.Minute, nil)
			case ErrQuotaExhausted:
				err = NewQuotaExhausted("quota", time.Now().Add(1*time.Hour))
			case ErrNotFound:
				err = NewNotFound("resource", nil)
			case ErrPermissionDenied:
				err = NewPermissionDenied("resource", nil)
			case ErrExecutionFailed:
				err = NewExecutionFailed("failed", true, nil)
			case ErrInvalidConfig:
				err = NewInvalidConfig("invalid", nil)
			case ErrCanceled:
				err = NewCanceled("canceled")
			}

			decision := RouteError(err)
			if decision.Reason == "" {
				t.Error("routing decision must provide a reason")
			}
		})
	}
}

// TestDecisionMatrixCompleteness verifies all error codes have routing rules.
func TestDecisionMatrixCompleteness(t *testing.T) {
	matrix := ErrorCodeRoutingMap()

	expectedCodes := []ErrorCode{
		ErrTimeout,
		ErrQuotaExhausted,
		ErrNotFound,
		ErrPermissionDenied,
		ErrExecutionFailed,
		ErrInvalidConfig,
		ErrCanceled,
	}

	for _, code := range expectedCodes {
		if _, ok := matrix[code]; !ok {
			t.Errorf("error code %q missing from routing matrix", code)
		}
	}
}

// TestDecisionMatrixActionValidity verifies all actions are valid.
func TestDecisionMatrixActionValidity(t *testing.T) {
	validActions := map[ErrorAction]bool{
		ActionRetry:    true,
		ActionFallback: true,
		ActionAbort:    true,
		ActionEscalate: true,
	}

	matrix := ErrorCodeRoutingMap()
	for code, action := range matrix {
		if !validActions[action] {
			t.Errorf("error code %q has invalid action %q", code, action)
		}
	}
}

// TestTimeoutRetryParametersReasonable verifies timeout retry parameters are sensible.
func TestTimeoutRetryParametersReasonable(t *testing.T) {
	err := NewTimeout(30*time.Second, nil)
	decision := RouteError(err)

	if decision.RetryDelay < 100*time.Millisecond {
		t.Error("timeout retry delay should be at least 100ms")
	}
	if decision.RetryDelay > 10*time.Second {
		t.Error("timeout retry delay should be at most 10s for immediate retries")
	}
	if decision.MaxRetries < 1 {
		t.Error("timeout should allow at least 1 retry")
	}
	if decision.MaxRetries > 10 {
		t.Error("timeout should not allow excessive retries")
	}
}

// TestExecutionFailedRetryParametersReasonable verifies execution failed retry parameters.
func TestExecutionFailedRetryParametersReasonable(t *testing.T) {
	err := NewExecutionFailed("process crashed", true, nil)
	decision := RouteError(err)

	if decision.MaxRetries != 1 {
		t.Errorf("execution failed should have exactly 1 retry, got %d", decision.MaxRetries)
	}
	if decision.RetryDelay < 100*time.Millisecond {
		t.Error("execution failed retry delay should be at least 100ms")
	}
}

// TestDecisionMatrixRetryParametersPresent verifies retry actions have delay and max retries.
func TestDecisionMatrixRetryParametersPresent(t *testing.T) {
	decisions := DecisionMatrix()

	for i, decision := range decisions {
		if decision.Action == ActionRetry {
			if decision.RetryDelay == 0 {
				t.Errorf("decision %d (retry) should have non-zero retry delay", i)
			}
			if decision.MaxRetries == 0 {
				t.Errorf("decision %d (retry) should have non-zero max retries", i)
			}
		}
	}
}

// TestDecisionMatrixFallbackParametersPresent verifies fallback actions specify backend.
func TestDecisionMatrixFallbackParametersPresent(t *testing.T) {
	decisions := DecisionMatrix()

	for i, decision := range decisions {
		if decision.Action == ActionFallback {
			if decision.BackendFallback == "" {
				t.Errorf("decision %d (fallback) should specify a fallback backend", i)
			}
		}
	}
}

// TestRouteErrorConsistency verifies multiple calls to RouteError return consistent results.
func TestRouteErrorConsistency(t *testing.T) {
	err := NewTimeout(5*time.Minute, nil)

	decision1 := RouteError(err)
	decision2 := RouteError(err)

	if decision1.Action != decision2.Action {
		t.Error("RouteError should return consistent actions")
	}
	if decision1.RetryDelay != decision2.RetryDelay {
		t.Error("RouteError should return consistent retry delays")
	}
	if decision1.MaxRetries != decision2.MaxRetries {
		t.Error("RouteError should return consistent max retries")
	}
}

// TestRouteErrorHandlesNil verifies RouteError handles nil gracefully.
func TestRouteErrorHandlesNil(t *testing.T) {
	// RouteError should handle nil error gracefully
	// In practice, this shouldn't happen, but defensive programming is good
	decision := RouteError(nil)

	if decision.Action == "" {
		t.Error("RouteError should return a valid action even for nil")
	}
	if decision.Reason == "" {
		t.Error("RouteError should provide a reason")
	}
}

// TestErrorRoutingMatrix is a comprehensive test of all error code routing decisions.
// This serves as the "decision matrix test" requirement.
func TestErrorRoutingMatrix(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedAction ErrorAction
		description    string
	}{
		{
			name:           "Timeout",
			err:            NewTimeout(5*time.Minute, nil),
			expectedAction: ActionRetry,
			description:    "Timeout → Retry with backoff",
		},
		{
			name:           "QuotaExhausted",
			err:            NewQuotaExhausted("API rate limit", time.Now().Add(1*time.Hour)),
			expectedAction: ActionFallback,
			description:    "QuotaExhausted → Fallback to different backend",
		},
		{
			name:           "NotFound",
			err:            NewNotFound("executable", nil),
			expectedAction: ActionAbort,
			description:    "NotFound → Abort immediately",
		},
		{
			name:           "PermissionDenied",
			err:            NewPermissionDenied("/bin/executable", nil),
			expectedAction: ActionEscalate,
			description:    "PermissionDenied → Escalate to user",
		},
		{
			name:           "ExecutionFailedRetryable",
			err:            NewExecutionFailed("process error", true, nil),
			expectedAction: ActionRetry,
			description:    "ExecutionFailed → Retry once, then abort",
		},
		{
			name:           "ExecutionFailedAfterRetries",
			err:            NewExecutionFailed("process error", true, nil),
			expectedAction: ActionAbort,
			description:    "ExecutionFailed (retries exhausted) → Abort",
		},
		{
			name:           "InvalidConfig",
			err:            NewInvalidConfig("missing backend URL", nil),
			expectedAction: ActionEscalate,
			description:    "InvalidConfig → Escalate to user",
		},
		{
			name:           "Canceled",
			err:            NewCanceled("user cancellation"),
			expectedAction: ActionAbort,
			description:    "Canceled → Abort immediately",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For ExecutionFailedAfterRetries, use retry count to push into abort
			var decision RoutingDecision
			if tt.name == "ExecutionFailedAfterRetries" {
				// First get the normal decision to know max retries
				initialDecision := RouteError(tt.err)
				// Then call with retry count at/exceeding the limit
				decision = RouteErrorWithRetryCount(tt.err, initialDecision.MaxRetries)
			} else {
				decision = RouteError(tt.err)
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expectedAction, decision.Action)
			}
		})
	}
}
