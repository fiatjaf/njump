package main

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func redirectToFavicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/njump/static/favicon/android-chrome-192x192.png", http.StatusFound)
}

func redirectToRandom(w http.ResponseWriter, r *http.Request) {
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
		set := make([]string, 0, 50)
		for _, pubkey := range cache.GetPaginatedKeys("pa", 1, 50) {
			set = append(set, pubkey)
		}
		if s := len(set); s > 0 {
			pick := set[rand.Intn(s)]
			npub, _ := nip19.EncodePublicKey(pick)
			target = "/" + npub
			return
		}
	}

	// otherwise try to pick an event
	const RELAY = "wss://nostr.wine"
	lastEvents := relayLastNotes(r.Context(), RELAY, false)
	if s := len(lastEvents); s > 0 {
		pick := lastEvents[rand.Intn(s)]
		nevent, _ := nip19.EncodeEvent(pick.ID, []string{RELAY}, pick.PubKey)
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
