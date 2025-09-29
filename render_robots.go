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

User-agent: MJ12bot
Disallow: /

User-agent: PetalBot
Disallow: /

User-agent: *
Allow: /

`, s.Domain, s.Domain, s.Domain)
}
