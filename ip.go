package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

// trustedProxyCIDRs contains ranges for the reverse proxies we expect in front of njump.
// We only trust forwarding headers when the TCP peer belongs to one of these networks.
var trustedProxyCIDRs = []string{
	// Cloudflare IPv4 ranges
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
	// Cloudflare IPv6 ranges
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

var (
	trustedProxyNets []*net.IPNet
	trustedProxyOnce sync.Once
)

func loadTrustedProxyNets() {
	trustedProxyNets = make([]*net.IPNet, 0, len(trustedProxyCIDRs))
	for _, cidr := range trustedProxyCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			// Keep best-effort behaviour; skip invalid entries but log the misconfiguration.
			log.Warn().Str("cidr", cidr).Err(err).Msg("failed to parse trusted proxy CIDR")
			continue
		}
		trustedProxyNets = append(trustedProxyNets, ipnet)
	}
}

func actualIP(r *http.Request) string {
	peerIP := extractPeerIP(r.RemoteAddr)
	if peerIP == nil {
		// If anything looks off, fall back to the original remote address so we keep the traceability.
		return r.RemoteAddr
	}

	if isTrustedProxy(peerIP) {
		if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
			if ip := net.ParseIP(cf); ip != nil {
				return ip.String()
			}
		}

		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// RFC 7239: the left-most address is the original caller.
			client := strings.TrimSpace(strings.Split(xff, ",")[0])
			if ip := net.ParseIP(client); ip != nil {
				return ip.String()
			}
		}
	}

	return peerIP.String()
}

func extractPeerIP(remoteAddr string) net.IP {
	if remoteAddr == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	// If SplitHostPort fails (e.g. no port component), try direct parsing as a last resort.
	return net.ParseIP(remoteAddr)
}

func isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}

	trustedProxyOnce.Do(loadTrustedProxyNets)
	for _, cidr := range trustedProxyNets {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
