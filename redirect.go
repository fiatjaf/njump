package main

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func redirectToFavicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/njump/static/favicon/android-chrome-192x192.png", http.StatusFound)
}

func redirectToRandom(w http.ResponseWriter, r *http.Request) {
	// 50% of chance of picking a pubkey
	if ra := rand.Intn(2); ra == 0 {
		set := make([]string, 0, 50)
		for _, pubkey := range cache.GetPaginatedkeys("pa", 1, 50) {
			set = append(set, pubkey)
		}
		if s := len(set); s > 0 {
			pick := set[rand.Intn(s)]
			http.Redirect(w, r, "/p/"+pick, http.StatusFound)
			return
		}
	}

	// otherwise try to pick an event
	const RELAY = "wss://nostr.wine"
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()
	var lastEvents []*nostr.Event
	if relay, err := pool.EnsureRelay(RELAY); err == nil {
		lastEvents, _ = relay.QuerySync(ctx, nostr.Filter{
			Kinds: []int{1},
			Limit: 50,
		})
	}
	if s := len(lastEvents); s > 0 {
		pick := lastEvents[rand.Intn(s)]
		nevent, _ := nip19.EncodeEvent(pick.ID, []string{RELAY}, pick.PubKey)
		http.Redirect(w, r, "/"+nevent, http.StatusFound)
		return
	}

	// go to a hardcoded place
	http.Redirect(w, r, "/npub1sg6plzptd64u62a878hep2kev88swjh3tw00gjsfl8f237lmu63q0uf63m", http.StatusFound)
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
