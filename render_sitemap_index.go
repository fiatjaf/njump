package main

import (
	"net/http"

	"fiatjaf.com/leafdb"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderSitemapIndex(w http.ResponseWriter, r *http.Request) {
	npubs := make([]string, 0, 5000)
	params := leafdb.AnyQuery("pubkey-archive")
	params.Limit = 5000
	for val := range internal.View(params) {
		pka := val.(*PubKeyArchive)
		npub, _ := nip19.EncodePublicKey(pka.Pubkey)
		npubs = append(npubs, npub)
	}

	if len(npubs) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "max-age=180")
	}

	w.Header().Add("content-type", "text/xml")
	w.Write([]byte(XML_HEADER))
	SitemapIndexTemplate.Render(w, &SitemapIndexPage{
		Host:  s.Domain,
		Npubs: npubs,
	})
}
