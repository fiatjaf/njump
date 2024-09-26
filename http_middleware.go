package main

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/favicon") {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		log.Debug().
			Str("ip", actualIP(r)).
			Str("path", path).
			Str("user-agent", r.Header.Get("User-Agent")).
			Str("referer", r.Header.Get("Referer")).
			Msg("request")

		next.ServeHTTP(w, r)
	})
}

var (
	queue              = [26]sync.Mutex{}
	concurrentRequests = [26]atomic.Uint32{}
)

func queueMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) <= 30 {
			next.ServeHTTP(w, r)
			return
		}

		willQueue := false
		for _, prefix := range []string{"/nevent1", "/image", "/naddr1", "/npub1", "/nprofile1", "/note1", "/embed"} {
			if strings.HasPrefix(r.URL.Path, prefix) {
				willQueue = true
				break
			}
		}

		if !willQueue {
			next.ServeHTTP(w, r)
			return
		}

		qidx := stupidHash(r.URL.Path)
		curr := concurrentRequests[qidx].Load()
		isFirst := curr == 0

		// add 1 and lock (or wait for the lock)
		concurrentRequests[qidx].Add(1)
		queue[qidx].Lock()

		if isFirst {
			next.ServeHTTP(w, r)

			// we are the first requesting this, so we have the duty to reset it to zero later
			concurrentRequests[qidx].Store(0)
			queue[qidx].Unlock()
			return
		}

		// if we are not the first to request this we will wait for the underlying page to be loaded
		// then we will be redirect to open it again, so hopefully we will hit the cloudflare cache this time
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}

		time.Sleep(time.Millisecond * 90)
		http.Redirect(w, r, path, http.StatusFound)

		queue[qidx].Unlock()
	}
}

// stupidHash doesn't care very much about collisions
func stupidHash(s string) int {
	return int(s[3]+s[7]+s[18]+s[29]) % 26
}
