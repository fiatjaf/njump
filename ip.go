package main

import (
	"net/http"
	"strings"
)

func actualIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return cf
	} else if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	} else {
		return r.RemoteAddr
	}
}
