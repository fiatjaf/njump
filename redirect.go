package main

import (
	"fmt"
	"math/rand"
	"net/http"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
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
			fmt.Fprintf(w, "%s", target[1:])
		case "GET":
			http.Redirect(w, r, target, http.StatusFound)
		}
	}()

	// 50% of chance of picking a pubkey
	if ra := rand.Intn(2); ra == 0 {
		if len(npubsArchive) > 0 {
			for pk := range npubsArchive {
				if rand.Intn(12) < 2 {
					target = "/" + nip19.EncodeNpub(pk)
					return
				}
			}
		}
	}

	// otherwise try to pick an event
	const RELAY = "wss://nostr.wine"
	for evt := range relayLastNotes(ctx, RELAY, 1) {
		target = "/" + nip19.EncodeNevent(evt.ID, []string{RELAY}, evt.PubKey)
		return
	}

	// go to a hardcoded place
	target = "/npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m"
	return
}

func redirectFromPSlash(w http.ResponseWriter, r *http.Request) {
	pk, err := nostr.PubKeyFromHexCheap(r.URL.Path[3:])
	if err != nil {
		http.Error(w, "invalid public key hex", 400)
		return
	}
	http.Redirect(w, r, "/"+nip19.EncodeNpub(pk), http.StatusFound)
	return
}

func redirectFromESlash(w http.ResponseWriter, r *http.Request) {
	id, err := nostr.IDFromHex(r.URL.Path[3:])
	if err != nil {
		http.Error(w, "invalid public key hex", 400)
		return
	}
	http.Redirect(w, r, "/"+nip19.EncodeNevent(id, nil, nostr.ZeroPK), http.StatusFound)
	return
}
