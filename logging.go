package main

import (
	"net/http"
	"strings"
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
