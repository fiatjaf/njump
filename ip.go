package main

import (
	"net"
	"net/http"
	"strings"
)

var trustProxyHeaders bool

func configureProxyTrust(trust bool) {
	trustProxyHeaders = trust
}

func actualIP(r *http.Request) string {
	if trustProxyHeaders {
		if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
			if ip := net.ParseIP(cf); ip != nil {
				return ip.String()
			}
		}

		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			client := strings.TrimSpace(strings.Split(xff, ",")[0])
			if ip := net.ParseIP(client); ip != nil {
				return ip.String()
			}
		}
	}

	if ip := extractPeerIP(r.RemoteAddr); ip != "" {
		return ip
	}

	return r.RemoteAddr
}

func extractPeerIP(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		remoteAddr = host
	}

	if ip := net.ParseIP(remoteAddr); ip != nil {
		return ip.String()
	}

	return ""
}
