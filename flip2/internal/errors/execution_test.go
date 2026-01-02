package errors

import (
	"fmt"
	"testing"
	"time"
)

func TestExecutionError(t *testing.T) {
	t.Run("timeout error", func(t *testing.T) {
		err := NewTimeout(5*time.Minute, nil)
		if err.Code != ErrTimeout {
			t.Errorf("expected code %s, got %s", ErrTimeout, err.Code)
		}
		if !err.Retryable {
			t.Error("timeout should be retryable")
		}
		if err.Error() == "" {
			t.Error("error message should not be empty")
		}
	})

	t.Run("quota exhausted", func(t *testing.T) {
		resetTime := time.Now().Add(1 * time.Hour)
		err := NewQuotaExhausted("API rate limit exceeded", resetTime)
		if err.Code != ErrQuotaExhausted {
			t.Errorf("expected code %s, got %s", ErrQuotaExhausted, err.Code)
		}
		if !err.Retryable {
			t.Error("quota exhausted should be retryable")
		}
		if err.RetryAfter.IsZero() {
			t.Error("retry after should be set")
		}
	})

	t.Run("not found", func(t *testing.T) {
		err := NewNotFound("claude", nil)
		if err.Code != ErrNotFound {
			t.Errorf("expected code %s, got %s", ErrNotFound, err.Code)
		}
		if err.Retryable {
			t.Error("not found should not be retryable")
		}
	})

	t.Run("execution failed", func(t *testing.T) {
		err := NewExecutionFailed("process crashed", true, nil)
		if err.Code != ErrExecutionFailed {
			t.Errorf("expected code %s, got %s", ErrExecutionFailed, err.Code)
		}
		if !err.Retryable {
			t.Error("execution failed with retryable=true should be retryable")
		}
	})

	t.Run("canceled", func(t *testing.T) {
		err := NewCanceled("user stopped execution")
		if err.Code != ErrCanceled {
			t.Errorf("expected code %s, got %s", ErrCanceled, err.Code)
		}
		if err.Retryable {
			t.Error("canceled should not be retryable")
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		err := NewPermissionDenied("/bin/claude", nil)
		if err.Code != ErrPermissionDenied {
			t.Errorf("expected code %s, got %s", ErrPermissionDenied, err.Code)
		}
		if err.Retryable {
			t.Error("permission denied should not be retryable")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		err := NewInvalidConfig("backend not found", nil)
		if err.Code != ErrInvalidConfig {
			t.Errorf("expected code %s, got %s", ErrInvalidConfig, err.Code)
		}
		if err.Retryable {
			t.Error("invalid config should not be retryable")
		}
	})

	t.Run("error unwrapping", func(t *testing.T) {
		cause := fmt.Errorf("underlying error")
		err := NewExecutionFailed("wrapper", false, cause)
		if err.Unwrap() != cause {
			t.Error("unwrap should return the cause")
		}
	})

	t.Run("is execution error", func(t *testing.T) {
		err := NewTimeout(5*time.Minute, nil)
		if !IsExecutionError(err) {
			t.Error("should recognize ExecutionError")
		}

		regularErr := fmt.Errorf("regular error")
		if IsExecutionError(regularErr) {
			t.Error("should not recognize regular error as ExecutionError")
		}
	})

	t.Run("as execution error", func(t *testing.T) {
		err := NewNotFound("resource", nil)
		execErr := AsExecutionError(err)
		if execErr == nil {
			t.Error("should cast to ExecutionError")
		}
		if execErr.Code != ErrNotFound {
			t.Errorf("expected code %s, got %s", ErrNotFound, execErr.Code)
		}

		regularErr := fmt.Errorf("regular error")
		if AsExecutionError(regularErr) != nil {
			t.Error("should return nil for non-ExecutionError")
		}
	})
}

func TestErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrTimeout,
		ErrQuotaExhausted,
		ErrNotFound,
		ErrExecutionFailed,
		ErrCanceled,
		ErrPermissionDenied,
		ErrInvalidConfig,
	}

	for _, code := range codes {
		if string(code) == "" {
			t.Errorf("error code should not be empty: %v", code)
		}
	}
}

func TestErrorCodeIsTransient(t *testing.T) {
	tests := []struct {
		code      ErrorCode
		transient bool
	}{
		{ErrTimeout, true},
		{ErrQuotaExhausted, true},
		{ErrNotFound, false},
		{ErrExecutionFailed, false},
		{ErrCanceled, false},
		{ErrPermissionDenied, false},
		{ErrInvalidConfig, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.IsTransient(); got != tt.transient {
				t.Errorf("IsTransient() = %v, want %v", got, tt.transient)
			}
		})
	}
}

func TestErrorCodeIsPermanent(t *testing.T) {
	tests := []struct {
		code      ErrorCode
		permanent bool
	}{
		{ErrTimeout, false},
		{ErrQuotaExhausted, false},
		{ErrNotFound, true},
		{ErrExecutionFailed, false},
		{ErrCanceled, true},
		{ErrPermissionDenied, true},
		{ErrInvalidConfig, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.IsPermanent(); got != tt.permanent {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.permanent)
			}
		})
	}
}

func TestErrorCodeRequiresRetry(t *testing.T) {
	tests := []struct {
		code         ErrorCode
		requiresRetry bool
	}{
		{ErrTimeout, true},
		{ErrQuotaExhausted, true},
		{ErrExecutionFailed, true}, // May be retryable, depends on instance
		{ErrNotFound, false},
		{ErrCanceled, false},
		{ErrPermissionDenied, false},
		{ErrInvalidConfig, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.RequiresRetry(); got != tt.requiresRetry {
				t.Errorf("RequiresRetry() = %v, want %v", got, tt.requiresRetry)
			}
		})
	}
}

func TestErrorCodeString(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected string
	}{
		{ErrTimeout, "Timeout"},
		{ErrQuotaExhausted, "Quota Exhausted"},
		{ErrNotFound, "Not Found"},
		{ErrExecutionFailed, "Execution Failed"},
		{ErrCanceled, "Canceled"},
		{ErrPermissionDenied, "Permission Denied"},
		{ErrInvalidConfig, "Invalid Configuration"},
		{ErrorCode("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestErrorCodeCategory(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		category ErrorCategory
	}{
		{ErrTimeout, CategoryTransient},
		{ErrQuotaExhausted, CategoryTransient},
		{ErrExecutionFailed, CategoryTransient}, // Default to transient for safety
		{ErrNotFound, CategoryPermanent},
		{ErrCanceled, CategoryPermanent},
		{ErrPermissionDenied, CategoryPermanent},
		{ErrInvalidConfig, CategoryConfiguration},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.Category(); got != tt.category {
				t.Errorf("Category() = %q, want %q", got, tt.category)
			}
		})
	}
}

func TestErrorCategoryMutuallyExclusive(t *testing.T) {
	// Verify that each error code belongs to exactly one category
	codes := []ErrorCode{
		ErrTimeout,
		ErrQuotaExhausted,
		ErrNotFound,
		ErrExecutionFailed,
		ErrCanceled,
		ErrPermissionDenied,
		ErrInvalidConfig,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			transient := code.IsTransient()
			permanent := code.IsPermanent()

			// Transient and permanent should be mutually exclusive
			if transient && permanent {
				t.Errorf("code %s cannot be both transient and permanent", code)
			}

			// Category should be consistent with transient/permanent flags
			category := code.Category()
			if transient && category != CategoryTransient {
				t.Errorf("transient code %s should have CategoryTransient, got %s", code, category)
			}
			if permanent && category == CategoryTransient {
				t.Errorf("permanent code %s should not have CategoryTransient", code)
			}
		})
	}
}

func TestWrapTimeout(t *testing.T) {
	t.Run("with no cause", func(t *testing.T) {
		err := WrapTimeout(nil, "api_call")
		if err.Code != ErrTimeout {
			t.Errorf("expected code %s, got %s", ErrTimeout, err.Code)
		}
		if !err.Retryable {
			t.Error("timeout should be retryable")
		}
		if err.Cause != nil {
			t.Error("cause should be nil")
		}
		if err.Message != "timeout during api_call operation" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})

	t.Run("with cause error", func(t *testing.T) {
		cause := fmt.Errorf("context deadline exceeded")
		err := WrapTimeout(cause, "database_query")
		if err.Code != ErrTimeout {
			t.Errorf("expected code %s, got %s", ErrTimeout, err.Code)
		}
		if !err.Retryable {
			t.Error("timeout should be retryable")
		}
		if err.Cause != cause {
			t.Error("cause should be preserved")
		}
		if err.Unwrap() != cause {
			t.Error("unwrap should return the cause")
		}
	})
}

func TestWrapQuotaExceeded(t *testing.T) {
	t.Run("with no cause", func(t *testing.T) {
		err := WrapQuotaExceeded(nil, "anthropic")
		if err.Code != ErrQuotaExhausted {
			t.Errorf("expected code %s, got %s", ErrQuotaExhausted, err.Code)
		}
		if !err.Retryable {
			t.Error("quota exceeded should be retryable")
		}
		if err.Cause != nil {
			t.Error("cause should be nil")
		}
		if err.Message != "quota exceeded for anthropic backend" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})

	t.Run("with cause error", func(t *testing.T) {
		cause := fmt.Errorf("rate limit: 429 Too Many Requests")
		err := WrapQuotaExceeded(cause, "openai")
		if err.Code != ErrQuotaExhausted {
			t.Errorf("expected code %s, got %s", ErrQuotaExhausted, err.Code)
		}
		if !err.Retryable {
			t.Error("quota exceeded should be retryable")
		}
		if err.Cause != cause {
			t.Error("cause should be preserved")
		}
	})
}

func TestWrapNotFound(t *testing.T) {
	t.Run("with no cause", func(t *testing.T) {
		err := WrapNotFound(nil, "executable claude")
		if err.Code != ErrNotFound {
			t.Errorf("expected code %s, got %s", ErrNotFound, err.Code)
		}
		if err.Retryable {
			t.Error("not found should not be retryable")
		}
		if err.Cause != nil {
			t.Error("cause should be nil")
		}
		if err.Message != "executable claude not found" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})

	t.Run("with cause error", func(t *testing.T) {
		cause := fmt.Errorf("no such file or directory")
		err := WrapNotFound(cause, "config file")
		if err.Code != ErrNotFound {
			t.Errorf("expected code %s, got %s", ErrNotFound, err.Code)
		}
		if err.Retryable {
			t.Error("not found should not be retryable")
		}
		if err.Cause != cause {
			t.Error("cause should be preserved")
		}
	})
}

func TestWrapExecutionError(t *testing.T) {
	t.Run("with no cause", func(t *testing.T) {
		err := WrapExecutionError(nil, "process crashed unexpectedly")
		if err.Code != ErrExecutionFailed {
			t.Errorf("expected code %s, got %s", ErrExecutionFailed, err.Code)
		}
		if !err.Retryable {
			t.Error("execution error should be retryable by default")
		}
		if err.Cause != nil {
			t.Error("cause should be nil")
		}
		if err.Message != "process crashed unexpectedly" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})

	t.Run("with cause error", func(t *testing.T) {
		cause := fmt.Errorf("segment violation")
		err := WrapExecutionError(cause, "subprocess failed")
		if err.Code != ErrExecutionFailed {
			t.Errorf("expected code %s, got %s", ErrExecutionFailed, err.Code)
		}
		if !err.Retryable {
			t.Error("execution error should be retryable by default")
		}
		if err.Cause != cause {
			t.Error("cause should be preserved")
		}
	})
}

func TestWrapCanceled(t *testing.T) {
	t.Run("with no cause", func(t *testing.T) {
		err := WrapCanceled(nil)
		if err.Code != ErrCanceled {
			t.Errorf("expected code %s, got %s", ErrCanceled, err.Code)
		}
		if err.Retryable {
			t.Error("canceled should not be retryable")
		}
		if err.Cause != nil {
			t.Error("cause should be nil")
		}
		if err.Message != "execution was canceled" {
			t.Errorf("unexpected message: %s", err.Message)
		}
	})

	t.Run("with cause error", func(t *testing.T) {
		cause := fmt.Errorf("context canceled")
		err := WrapCanceled(cause)
		if err.Code != ErrCanceled {
			t.Errorf("expected code %s, got %s", ErrCanceled, err.Code)
		}
		if err.Retryable {
			t.Error("canceled should not be retryable")
		}
		if err.Cause != cause {
			t.Error("cause should be preserved")
		}
		if err.Unwrap() != cause {
			t.Error("unwrap should return the cause")
		}
	})
}

func TestWrapperErrorUnwrapping(t *testing.T) {
	t.Run("wrap timeout", func(t *testing.T) {
		cause := fmt.Errorf("underlying timeout")
		err := WrapTimeout(cause, "test")
		if err.Unwrap() != cause {
			t.Error("unwrap should return the original cause")
		}
	})

	t.Run("wrap quota exceeded", func(t *testing.T) {
		cause := fmt.Errorf("underlying quota")
		err := WrapQuotaExceeded(cause, "test")
		if err.Unwrap() != cause {
			t.Error("unwrap should return the original cause")
		}
	})

	t.Run("wrap not found", func(t *testing.T) {
		cause := fmt.Errorf("underlying not found")
		err := WrapNotFound(cause, "test")
		if err.Unwrap() != cause {
			t.Error("unwrap should return the original cause")
		}
	})

	t.Run("wrap execution error", func(t *testing.T) {
		cause := fmt.Errorf("underlying execution")
		err := WrapExecutionError(cause, "test details")
		if err.Unwrap() != cause {
			t.Error("unwrap should return the original cause")
		}
	})

	t.Run("wrap canceled", func(t *testing.T) {
		cause := fmt.Errorf("underlying canceled")
		err := WrapCanceled(cause)
		if err.Unwrap() != cause {
			t.Error("unwrap should return the original cause")
		}
	})
}

func TestWrapperErrorCodes(t *testing.T) {
	tests := []struct {
		name         string
		fn           func() *ExecutionError
		expectedCode ErrorCode
	}{
		{"WrapTimeout", func() *ExecutionError { return WrapTimeout(nil, "op") }, ErrTimeout},
		{"WrapQuotaExceeded", func() *ExecutionError { return WrapQuotaExceeded(nil, "backend") }, ErrQuotaExhausted},
		{"WrapNotFound", func() *ExecutionError { return WrapNotFound(nil, "resource") }, ErrNotFound},
		{"WrapExecutionError", func() *ExecutionError { return WrapExecutionError(nil, "details") }, ErrExecutionFailed},
		{"WrapCanceled", func() *ExecutionError { return WrapCanceled(nil) }, ErrCanceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err.Code != tt.expectedCode {
				t.Errorf("expected code %s, got %s", tt.expectedCode, err.Code)
			}
		})
	}
}

func TestWrapperErrorRetryability(t *testing.T) {
	tests := []struct {
		name       string
		fn         func() *ExecutionError
		retryable  bool
	}{
		{"WrapTimeout", func() *ExecutionError { return WrapTimeout(nil, "op") }, true},
		{"WrapQuotaExceeded", func() *ExecutionError { return WrapQuotaExceeded(nil, "backend") }, true},
		{"WrapNotFound", func() *ExecutionError { return WrapNotFound(nil, "resource") }, false},
		{"WrapExecutionError", func() *ExecutionError { return WrapExecutionError(nil, "details") }, true},
		{"WrapCanceled", func() *ExecutionError { return WrapCanceled(nil) }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err.Retryable != tt.retryable {
				t.Errorf("expected retryable=%v, got %v", tt.retryable, err.Retryable)
			}
		})
	}
}
