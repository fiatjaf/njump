package main

import (
	"net/http"

	"fiatjaf.com/leafdb"
	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

func renderSitemapIndex(w http.ResponseWriter, r *http.Request) {
	npubs := make([]string, 0, 5000)
	params := leafdb.AnyQuery("pubkey-archive")
	params.Limit = 5000
	for val := range internal.View(params) {
		if pka, err := nostr.PubKeyFromHex(val.(*PubKeyArchive).Pubkey); err == nil {
			npubs = append(npubs, nip19.EncodeNpub(pka))
		}
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
