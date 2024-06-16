package main

import (
	"fmt"
	"net/http"
)

func renderRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")
	fmt.Fprintf(w, `User-agent: *
Allow: /

Sitemap: https://%s/npubs-archive.xml
Sitemap: https://%s/npubs-sitemaps.xml
Sitemap: https://%s/relays-archive.xml
`, s.Domain, s.Domain, s.Domain)
}
