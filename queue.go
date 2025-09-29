package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

type queuedReq struct {
	w http.ResponseWriter
	r *http.Request
}

var sem = semaphore.NewWeighted(int64(52))

func semaphoreMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if len(r.URL.Path) <= 30 || strings.HasPrefix(r.URL.Path, "/njump/static") {
			next.ServeHTTP(w, r)
			return
		}

		var cost int64 = 0
		for _, item := range []struct {
			prefix string
			cost   int64
		}{{"/njump", 3}, {"/nevent1", 1}, {"/image", 3}, {"/naddr1", 1}, {"/npub1", 2}, {"/nprofile1", 2}, {"/note1", 1}, {"/embed", 2}} {
			if strings.HasPrefix(r.URL.Path, item.prefix) {
				cost = item.cost
				break
			}
		}

		if cost == 0 {
			next.ServeHTTP(w, r)
			return
		}

		if err := sem.Acquire(ctx, cost); err != nil {
			log.Warn().Err(err).Str("path", r.URL.Path).Str("ip", actualIP(r)).Msg("canceled request on semaphore")
			http.Error(w, "server overloaded, try again later", 529)
			return
		}

		defer sem.Release(cost)
		next.ServeHTTP(w, r)
	}
}

var (
	queue              = [26]sync.Mutex{}
	concurrentRequests = [26]atomic.Uint32{}
)

func queueMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if len(r.URL.Path) <= 30 || strings.HasPrefix(r.URL.Path, "/njump/static") {
			next.ServeHTTP(w, r)
			return
		}

		willQueue := false
		for _, prefix := range []string{"/njump", "/nevent1", "/image", "/naddr1", "/npub1", "/nprofile1", "/note1", "/embed"} {
			if strings.HasPrefix(r.URL.Path, prefix) {
				willQueue = true
				break
			}
		}

		if !willQueue {
			next.ServeHTTP(w, r)
			return
		}

		qidx := stupidHash(r.URL.Path) % len(concurrentRequests)
		// add 1
		count := concurrentRequests[qidx].Add(1)
		isFirst := count == 1
		if count > 2 {
			log.Debug().Str("path", r.URL.Path).Uint32("count", count).Int("qidx", qidx).Str("ip", actualIP(r)).
				Msg("too many concurrent requests")
			goto notthefirst
		}

		// lock (or wait for the lock)
		queue[qidx].Lock()

		if isFirst {
			// we are the first requesting this, so we have the duty to reset it to zero later
			defer concurrentRequests[qidx].Store(0)
			defer queue[qidx].Unlock()
			// defer these calls because if there is a panic on ServeHTTP the server will catch it

			newCtx, cancel := context.WithTimeout(r.Context(), time.Second*30)
			defer cancel()

			done := make(chan struct{})
			go func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						// Log the panic with full context
						panicErr := fmt.Errorf("panic: %v", recovered)
						metadata := map[string]string{
							"panic_type":  fmt.Sprintf("%T", recovered),
							"panic_value": fmt.Sprintf("%+v", recovered),
						}

						// Log the panic with full context
						TrackError("panic", "HTTP handler panic recovered in queue goroutine", panicErr, r, metadata)

						// Also log to standard logger with stack trace for immediate debugging
						log.Error().
							Interface("panic", recovered).
							Str("path", r.URL.Path).
							Str("method", r.Method).
							Str("ip", actualIP(r)).
							Msg("panic recovered in queue goroutine")

						// Return a proper error response to the client
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					}
					close(done)
				}()
				next.ServeHTTP(w, r.WithContext(newCtx))
			}()

			select {
			case <-done:
			case <-ctx.Done():
			}
			return
		}

		queue[qidx].Unlock()

		// if we are not the first to request this we will wait for the underlying page to be loaded
		// then we will be redirect to open it again, so hopefully we will hit the cloudflare cache this time
	notthefirst:
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}

		select {
		case <-time.After(time.Second * 9):
			http.Redirect(w, r, path, http.StatusFound)
		case <-ctx.Done():
		}
	}
}

// stupidHash doesn't care very much about collisions
func stupidHash(s string) int {
	return int(s[3] + s[7] + s[18] + s[29])
}
