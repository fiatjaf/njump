package main

import (
	"net"
	"net/http"
	"strings"
)

func agentBlock(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		for _, bua := range []string{
			"Amazonbot",
			"semrush",
			"Bytespider",
			"AhrefsBot",
			"DataForSeoBot",
			"Yandex",
			"meta-externalagent",
			"DotBot",
			"ClaudeBot",
			"GPTBot",
		} {
			if strings.Contains(ua, bua) {
				// log.Debug().Str("ua", ua).Msg("user-agent blocked")
				http.Error(w, "", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func ipBlock(next http.HandlerFunc) http.HandlerFunc {
	ranges := make([]*net.IPNet, 0, 18)

	for _, line := range []string{
		// alicloud
		"47.52.0.0/16",
		"47.76.0.0/16",

		// cloudflare
		"173.245.48.0/20",
		"103.21.244.0/22",
		"103.22.200.0/22",
		"103.31.4.0/22",
		"141.101.64.0/18",
		"108.162.192.0/18",
		"190.93.240.0/20",
		"188.114.96.0/20",
		"197.234.240.0/22",
		"198.41.128.0/17",
		"162.158.0.0/15",
		"104.16.0.0/13",
		"104.24.0.0/14",
		"172.64.0.0/13",
		"131.0.72.0/22",
	} {
		_, ipnet, err := net.ParseCIDR(strings.TrimSpace(line))
		if err != nil {
			log.Error().Str("line", line).Err(err).Msg("failed to parse cloudflare ip range")
			continue
		}
		ranges = append(ranges, ipnet)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := net.ParseIP(actualIP(r))
		if ip != nil {
			for _, ipnet := range ranges {
				if ipnet.Contains(ip) {
					log.Debug().Stringer("ip", ip).Msg("ip blocked")
					http.Error(w, "", http.StatusForbidden)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
