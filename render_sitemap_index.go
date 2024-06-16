package main

import (
	"net/http"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderSitemapIndex(w http.ResponseWriter, r *http.Request) {
	npubs := make([]string, 0, 5000)
	keys := cache.GetPaginatedKeys("pa:", 1, 5000)
	for _, key := range keys {
		npub, _ := nip19.EncodePublicKey(key[3:])
		npubs = append(npubs, npub)
	}

	if len(npubs) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	w.Header().Add("content-type", "text/xml")
	w.Write([]byte(XML_HEADER))
	SitemapIndexTemplate.Render(w, &SitemapIndexPage{
		Host:  s.Domain,
		Npubs: npubs,
	})
}
