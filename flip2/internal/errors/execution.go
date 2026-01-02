package errors

import (
	"fmt"
	"time"
)

// ErrorCategory represents the classification of an error code.
type ErrorCategory string

const (
	// CategoryTransient indicates the error is temporary and may succeed on retry.
	CategoryTransient ErrorCategory = "transient"

	// CategoryPermanent indicates the error will persist without external intervention.
	CategoryPermanent ErrorCategory = "permanent"

	// CategoryConfiguration indicates the error is due to invalid configuration.
	CategoryConfiguration ErrorCategory = "configuration"
)

// ErrorCode represents a type-safe error code for execution failures.
type ErrorCode string

const (
	// ErrTimeout indicates the execution exceeded its time limit.
	// This error is retryable if the timeout can be extended.
	ErrTimeout ErrorCode = "timeout"

	// ErrQuotaExhausted indicates API quota or rate limits were exceeded.
	// This error is retryable after the quota reset period.
	ErrQuotaExhausted ErrorCode = "quota_exhausted"

	// ErrNotFound indicates the executable, binary, or resource was not found.
	// This error is not retryable without fixing the configuration.
	ErrNotFound ErrorCode = "not_found"

	// ErrExecutionFailed indicates a general execution failure.
	// This may be retryable depending on the underlying cause.
	ErrExecutionFailed ErrorCode = "execution_error"

	// ErrCanceled indicates the execution was explicitly canceled.
	// This error is not retryable as it was intentional.
	ErrCanceled ErrorCode = "canceled"

	// ErrPermissionDenied indicates insufficient permissions to execute.
	// This error is not retryable without fixing permissions.
	ErrPermissionDenied ErrorCode = "permission_denied"

	// ErrInvalidConfig indicates invalid or missing configuration.
	// This error is not retryable without fixing the configuration.
	ErrInvalidConfig ErrorCode = "invalid_config"
)

// IsTransient returns true if the error is temporary and may succeed on retry.
// Transient errors include timeouts and quota exhaustion.
func (c ErrorCode) IsTransient() bool {
	return c == ErrTimeout || c == ErrQuotaExhausted
}

// IsPermanent returns true if the error will persist without external intervention.
// Permanent errors include not found, permission denied, invalid config, and canceled operations.
func (c ErrorCode) IsPermanent() bool {
	return c == ErrNotFound || c == ErrPermissionDenied || c == ErrInvalidConfig || c == ErrCanceled
}

// RequiresRetry returns true if the error should trigger a retry attempt.
// This considers both transient errors and execution failures that are marked as retryable.
func (c ErrorCode) RequiresRetry() bool {
	// Transient errors always require retry
	if c.IsTransient() {
		return true
	}
	// ErrExecutionFailed may be retryable depending on the specific error instance
	// The Retryable field on ExecutionError should be checked for this code
	return c == ErrExecutionFailed
}

// String returns the human-readable name of the error code.
func (c ErrorCode) String() string {
	switch c {
	case ErrTimeout:
		return "Timeout"
	case ErrQuotaExhausted:
		return "Quota Exhausted"
	case ErrNotFound:
		return "Not Found"
	case ErrExecutionFailed:
		return "Execution Failed"
	case ErrCanceled:
		return "Canceled"
	case ErrPermissionDenied:
		return "Permission Denied"
	case ErrInvalidConfig:
		return "Invalid Configuration"
	default:
		return string(c)
	}
}

// Category returns the classification of the error code.
// Error categories help determine handling strategies:
// - Transient: temporary failures that may succeed on retry
// - Permanent: failures requiring external intervention
// - Configuration: failures due to invalid or missing configuration
func (c ErrorCode) Category() ErrorCategory {
	switch c {
	case ErrTimeout, ErrQuotaExhausted:
		return CategoryTransient
	case ErrNotFound, ErrPermissionDenied, ErrCanceled:
		return CategoryPermanent
	case ErrInvalidConfig:
		return CategoryConfiguration
	case ErrExecutionFailed:
		// Execution failures can be either transient or permanent
		// The specific category depends on the underlying cause
		// Default to transient as it's safer to retry
		return CategoryTransient
	default:
		return CategoryPermanent
	}
}

// ExecutionError represents a structured error from execution operations.
// It provides type-safe error codes, retry semantics, and cause tracking.
type ExecutionError struct {
	// Code is the error type identifier for programmatic handling.
	Code ErrorCode

	// Message is a human-readable description of the error.
	Message string

	// Retryable indicates whether retrying the operation might succeed.
	Retryable bool

	// Cause is the underlying error that triggered this execution error.
	// May be nil for synthetic errors without an underlying cause.
	Cause error

	// RetryAfter indicates when a retry might succeed (for quota errors).
	// Zero value means no specific retry time is known.
	RetryAfter time.Time
}

// Error implements the error interface.
func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for error chain unwrapping.
func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// NewTimeout creates a timeout error.
// Timeout errors are considered retryable if the timeout duration can be extended.
func NewTimeout(duration time.Duration, cause error) *ExecutionError {
	return &ExecutionError{
		Code:      ErrTimeout,
		Message:   fmt.Sprintf("execution timeout after %v", duration),
		Retryable: true,
		Cause:     cause,
	}
}

// NewQuotaExhausted creates a quota exhaustion error.
// Quota errors are retryable after the specified reset time.
func NewQuotaExhausted(message string, resetTime time.Time) *ExecutionError {
	return &ExecutionError{
		Code:       ErrQuotaExhausted,
		Message:    message,
		Retryable:  true,
		RetryAfter: resetTime,
	}
}

// NewNotFound creates a not-found error for missing executables or resources.
// Not-found errors are not retryable without fixing the configuration.
func NewNotFound(resource string, cause error) *ExecutionError {
	return &ExecutionError{
		Code:      ErrNotFound,
		Message:   fmt.Sprintf("%s not found", resource),
		Retryable: false,
		Cause:     cause,
	}
}

// NewExecutionFailed creates a general execution failure error.
// The retryable flag should be set based on the nature of the failure.
func NewExecutionFailed(message string, retryable bool, cause error) *ExecutionError {
	return &ExecutionError{
		Code:      ErrExecutionFailed,
		Message:   message,
		Retryable: retryable,
		Cause:     cause,
	}
}

// NewCanceled creates a cancellation error.
// Canceled operations are not retryable as they were intentionally stopped.
func NewCanceled(message string) *ExecutionError {
	return &ExecutionError{
		Code:      ErrCanceled,
		Message:   message,
		Retryable: false,
	}
}

// NewPermissionDenied creates a permission error.
// Permission errors are not retryable without fixing file/executable permissions.
func NewPermissionDenied(resource string, cause error) *ExecutionError {
	return &ExecutionError{
		Code:      ErrPermissionDenied,
		Message:   fmt.Sprintf("permission denied: %s", resource),
		Retryable: false,
		Cause:     cause,
	}
}

// NewInvalidConfig creates an invalid configuration error.
// Configuration errors are not retryable without fixing the config.
func NewInvalidConfig(message string, cause error) *ExecutionError {
	return &ExecutionError{
		Code:      ErrInvalidConfig,
		Message:   message,
		Retryable: false,
		Cause:     cause,
	}
}

// IsExecutionError checks if an error is an ExecutionError.
func IsExecutionError(err error) bool {
	_, ok := err.(*ExecutionError)
	return ok
}

// AsExecutionError attempts to cast an error to ExecutionError.
// Returns nil if the error is not an ExecutionError.
func AsExecutionError(err error) *ExecutionError {
	if execErr, ok := err.(*ExecutionError); ok {
		return execErr
	}
	return nil
}

// WrapTimeout wraps an error as a timeout error.
// This helper function is useful when an operation exceeded its time limit.
// The resulting error is marked as retryable.
func WrapTimeout(err error, operation string) *ExecutionError {
	msg := fmt.Sprintf("timeout during %s operation", operation)
	if err != nil && err.Error() != "" {
		msg = fmt.Sprintf("timeout during %s operation: %v", operation, err)
	}
	return &ExecutionError{
		Code:      ErrTimeout,
		Message:   msg,
		Retryable: true,
		Cause:     err,
	}
}

// WrapQuotaExceeded wraps an error as a quota exhaustion error.
// This helper function is useful when an API or backend quota was exceeded.
// The resulting error is marked as retryable.
func WrapQuotaExceeded(err error, backend string) *ExecutionError {
	msg := fmt.Sprintf("quota exceeded for %s backend", backend)
	if err != nil && err.Error() != "" {
		msg = fmt.Sprintf("quota exceeded for %s backend: %v", backend, err)
	}
	return &ExecutionError{
		Code:      ErrQuotaExhausted,
		Message:   msg,
		Retryable: true,
		Cause:     err,
	}
}

// WrapNotFound wraps an error as a not-found error.
// This helper function is useful when a resource, executable, or file was not found.
// The resulting error is marked as non-retryable.
func WrapNotFound(err error, resource string) *ExecutionError {
	msg := fmt.Sprintf("%s not found", resource)
	if err != nil && err.Error() != "" {
		msg = fmt.Sprintf("%s not found: %v", resource, err)
	}
	return &ExecutionError{
		Code:      ErrNotFound,
		Message:   msg,
		Retryable: false,
		Cause:     err,
	}
}

// WrapExecutionError wraps an error as a general execution error.
// This helper function is useful for wrapping unexpected execution failures.
// The resulting error is marked as retryable by default.
func WrapExecutionError(err error, details string) *ExecutionError {
	msg := details
	if err != nil && err.Error() != "" {
		msg = fmt.Sprintf("%s: %v", details, err)
	}
	return &ExecutionError{
		Code:      ErrExecutionFailed,
		Message:   msg,
		Retryable: true,
		Cause:     err,
	}
}

// WrapCanceled wraps an error as a cancellation error.
// This helper function is useful when an operation was explicitly canceled.
// The resulting error is marked as non-retryable.
func WrapCanceled(err error) *ExecutionError {
	msg := "execution was canceled"
	if err != nil && err.Error() != "" {
		msg = fmt.Sprintf("execution was canceled: %v", err)
	}
	return &ExecutionError{
		Code:      ErrCanceled,
		Message:   msg,
		Retryable: false,
		Cause:     err,
	}
}
