package main

import (
	"fmt"
	"math/rand"
	"net/http"

	"fiatjaf.com/leafdb"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func redirectToFavicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/njump/static/favicon/android-chrome-192x192.png", http.StatusFound)
}

func redirectToRandom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var target string
	defer func() {
		switch r.Method {
		case "POST":
			fmt.Fprintf(w, target[1:])
		case "GET":
			http.Redirect(w, r, target, http.StatusFound)
		}
	}()

	// 50% of chance of picking a pubkey
	if ra := rand.Intn(2); ra == 0 {
		params := leafdb.AnyQuery("pubkey-archive")
		params.Skip = rand.Intn(50)
		for val := range internal.View(params) {
			pka := val.(*PubKeyArchive)
			npub, _ := nip19.EncodePublicKey(pka.Pubkey)
			target = "/" + npub
			return
		}
	}

	// otherwise try to pick an event
	const RELAY = "wss://nostr.wine"
	for evt := range relayLastNotes(ctx, RELAY, 1) {
		nevent, _ := nip19.EncodeEvent(evt.ID, []string{RELAY}, evt.PubKey)
		target = "/" + nevent
		return
	}

	// go to a hardcoded place
	target = "/npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m"
	return
}

func redirectFromPSlash(w http.ResponseWriter, r *http.Request) {
	code, _ := nip19.EncodePublicKey(r.URL.Path[3:])
	http.Redirect(w, r, "/"+code, http.StatusFound)
	return
}

func redirectFromESlash(w http.ResponseWriter, r *http.Request) {
	code, _ := nip19.EncodeEvent(r.URL.Path[3:], []string{}, "")
	http.Redirect(w, r, "/"+code, http.StatusFound)
	return
}
