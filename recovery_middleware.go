package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

// Catches panics and logs them while returning a proper HTTP response
func recoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// Create detailed error entry for the panic
				panicErr := fmt.Errorf("panic: %v", recovered)
				metadata := map[string]string{
					"panic_type":  fmt.Sprintf("%T", recovered),
					"panic_value": fmt.Sprintf("%+v", recovered),
				}

				// Log the panic with full context
				TrackError("panic", "HTTP handler panic recovered", panicErr, r, metadata)

				// Also log to standard logger with stack trace for immediate debugging
				log.Error().
					Interface("panic", recovered).
					Str("path", r.URL.Path).
					Str("method", r.Method).
					Str("ip", actualIP(r)).
					Bytes("stack", debug.Stack()).
					Msg("panic recovered in HTTP handler")

				// Return a proper error response to the client
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// Logs an application error with full context
func LoggedError(err error, context string, r *http.Request, metadata map[string]string) {
	if err == nil {
		return
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}
	if context != "" {
		metadata["context"] = context
	}

	TrackAppError(fmt.Sprintf("Application error: %s", context), err, r, metadata)
}
