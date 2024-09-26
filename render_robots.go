package main

import (
	"fmt"
	"net/http"
)

func renderRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")
	fmt.Fprintf(w, `
User-agent: Amazonbot
Disallow: /

User-agent: SemrushBot
Disallow: /

User-agent: meta-externalagent
Disallow: /

User-agent: DataForSeoBot
Disallow: /

User-agent: dotbot
Disallow: /

User-agent: *
Allow: /

Sitemap: https://%s/npubs-archive.xml
Sitemap: https://%s/npubs-sitemaps.xml
Sitemap: https://%s/relays-archive.xml
`, s.Domain, s.Domain, s.Domain)
}
