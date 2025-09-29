package main

import (
	"bytes"
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

	var buf bytes.Buffer
	buf.WriteString(XML_HEADER)
	err := SitemapIndexTemplate.Render(&buf, &SitemapIndexPage{
		Host:  s.Domain,
		Npubs: npubs,
	})
	if err == nil {
		w.Write(buf.Bytes())
	} else {
		log.Warn().Err(err).Msg("error rendering sitemap index template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
