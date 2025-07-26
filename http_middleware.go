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
			return
		}

		// lock (or wait for the lock)
		queue[qidx].Lock()

		if isFirst {
			// we are the first requesting this, so we have the duty to reset it to zero later
			defer concurrentRequests[qidx].Store(0)
			defer queue[qidx].Unlock()
			// defer these calls because if there is a panic on ServeHTTP the server will catch it

			next.ServeHTTP(w, r)
			return
		}

		// if we are not the first to request this we will wait for the underlying page to be loaded
		// then we will be redirect to open it again, so hopefully we will hit the cloudflare cache this time
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}

		queue[qidx].Unlock()

		time.Sleep(time.Millisecond * 90)
		http.Redirect(w, r, path, http.StatusFound)
	}
}

// stupidHash doesn't care very much about collisions
func stupidHash(s string) int {
	return int(s[3] + s[7] + s[18] + s[29])
}
