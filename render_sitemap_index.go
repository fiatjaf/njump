package main

import (
	"net/http"

	"fiatjaf.com/nostr/nip19"
)

func renderSitemapIndex(w http.ResponseWriter, r *http.Request) {
	npubs := make([]string, 0, 5000)
	i := 0
	for pk := range npubsArchive {
		npubs = append(npubs, nip19.EncodeNpub(pk))
		i++
		if i == 5000 {
			break
		}
	}

	if len(npubs) != 0 {
		w.Header().Set("Cache-Control", "public, immutable, s-maxage=86400, max-age=86400")
	} else {
		w.Header().Set("Cache-Control", "s-maxage=180, max-age=180")
	}

	w.Header().Add("content-type", "text/xml")
	w.Write([]byte(XML_HEADER))
	SitemapIndexTemplate.Render(w, &SitemapIndexPage{
		Host:  s.Domain,
		Npubs: npubs,
	})
}
