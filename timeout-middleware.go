package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// timeoutResponseWriter wraps http.ResponseWriter to prevent writes after timeout
type timeoutResponseWriter struct {
	http.ResponseWriter
	mu          sync.Mutex
	timedOut    bool
	wroteHeader bool
}

func (tw *timeoutResponseWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.wroteHeader = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutResponseWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if !tw.wroteHeader {
		tw.wroteHeader = true
	}
	return tw.ResponseWriter.Write(b)
}

func (tw *timeoutResponseWriter) markTimedOut() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.timedOut = true
}

func timeoutMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		tw := &timeoutResponseWriter{ResponseWriter: w}
		done := make(chan struct{})
		panicChan := make(chan interface{}, 1)

		go func() {
			defer func() {
				if err := recover(); err != nil {
					panicChan <- err
				}
			}()

			next.ServeHTTP(tw, r.WithContext(ctx))
			close(done)
		}()

		select {
		case p := <-panicChan:
			// Panic occurred, re-panic in main goroutine
			panic(p)
		case <-done:
			// Request completed successfully
			return
		case <-ctx.Done():
			// Timeout reached
			tw.markTimedOut()
			if ctx.Err() == context.DeadlineExceeded {
				// Only write retry page if handler hasn't written anything yet
				tw.mu.Lock()
				hasWritten := tw.wroteHeader
				defer tw.mu.Unlock()

				if !hasWritten {
					w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
					w.Header().Set("Pragma", "no-cache")
					w.Header().Set("Expires", "0")
					w.Header().Set("X-Robots-Tag", "noindex, nofollow")
					w.WriteHeader(http.StatusOK)

					retryTemplate(RetryPageParams{
						HeadParams: HeadParams{},
					}).Render(r.Context(), w)
				}
			}
		}
	}
}
