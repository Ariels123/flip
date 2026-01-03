package recovery

import (
	"log/slog"
	"runtime/debug"
)

// SafelyGo starts a goroutine with panic recovery.
// name is used for logging context if a panic occurs.
func SafelyGo(logger *slog.Logger, name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace
				logger.Error("PANIC RECOVERED",
					"goroutine", name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}
