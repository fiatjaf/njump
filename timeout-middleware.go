package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// timeoutResponseWriter wraps http.ResponseWriter to prevent writes after timeout
// the handler gets its own header map (like http.TimeoutHandler) so that it never
// shares a map with the timeout path, which writes headers from another goroutine
type timeoutResponseWriter struct {
	http.ResponseWriter
	h           http.Header
	mu          sync.Mutex
	timedOut    bool
	wroteHeader bool
}

func (tw *timeoutResponseWriter) Header() http.Header { return tw.h }

// must be called with tw.mu held
func (tw *timeoutResponseWriter) writeHeaderLocked(code int) {
	tw.wroteHeader = true
	dst := tw.ResponseWriter.Header()
	for k, vv := range tw.h {
		dst[k] = vv
	}
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutResponseWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.writeHeaderLocked(code)
}

func (tw *timeoutResponseWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if !tw.wroteHeader {
		tw.writeHeaderLocked(http.StatusOK)
	}
	return tw.ResponseWriter.Write(b)
}

// marks the writer as timed out and reports whether the handler had already
// started the response, atomically so the handler can't start it in between
func (tw *timeoutResponseWriter) markTimedOut() (wroteHeader bool) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.timedOut = true
	return tw.wroteHeader
}

func timeoutMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		tw := &timeoutResponseWriter{ResponseWriter: w, h: make(http.Header)}
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
			hasWritten := tw.markTimedOut()
			if ctx.Err() == context.DeadlineExceeded {
				// Only write retry page if handler hasn't written anything yet
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
