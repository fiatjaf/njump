package main

import (
	"net/http"
	"strings"
)

func agentBlock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.UserAgent()
		if strings.Contains(userAgent, "Amazonbot") {
			// Drop the request by returning a 403 Forbidden response
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}
