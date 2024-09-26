package main

import "net/http"

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		log.Debug().
			Str("ip", r.Header.Get("X-Forwarded-For")).
			Str("path", path).
			Str("user-agent", r.Header.Get("User-Agent")).
			Str("referer", r.Header.Get("Referer")).
			Msg("request")

		next.ServeHTTP(w, r)
	})
}
