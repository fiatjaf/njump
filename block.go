package main

import (
	"io"
	"net"
	"net/http"
	"strings"
	"time"
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

func cloudflareBlock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := net.ParseIP(actualIP(r))
		if ip != nil {
			for _, ipnet := range cloudflareRanges {
				if ipnet.Contains(ip) {
					// cloudflare is not allowed
					log.Debug().Stringer("ip", ip).Msg("cloudflare (attacker) ip blocked")
					http.Redirect(w, r, "https://njump.me/", 302)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

var cloudflareRanges []*net.IPNet

func updateCloudflareRangesRoutine() {
	for {
		newRanges := make([]*net.IPNet, 0, 30)

		for _, url := range []string{
			"https://www.cloudflare.com/ips-v6/",
			"https://www.cloudflare.com/ips-v4/",
		} {
			resp, err := http.Get(url)
			if err != nil {
				log.Error().Err(err).Msg("failed to fetch cloudflare ips")
				continue
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				_, ipnet, err := net.ParseCIDR(strings.TrimSpace(line))
				if err != nil {
					log.Error().Str("line", line).Err(err).Msg("failed to parse cloudflare ip range")
					continue
				}
				newRanges = append(newRanges, ipnet)
			}
		}
		if len(newRanges) > 0 {
			cloudflareRanges = newRanges
		}

		time.Sleep(time.Hour * 24)
	}
}
