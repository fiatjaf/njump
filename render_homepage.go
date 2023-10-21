package main

import (
	"context"
	"net/http"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderHomepage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")

	npubsHex := cache.GetPaginatedkeys("pa", 1, 50)
	npubs := []string{}
	for i := 0; i < len(npubsHex); i++ {
		npub, _ := nip19.EncodePublicKey(npubsHex[i])
		npubs = append(npubs, npub)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()
	var lastEvents []*nostr.Event
	if relay, err := pool.EnsureRelay("nostr.wine"); err == nil {
		lastEvents, _ = relay.QuerySync(ctx, nostr.Filter{
			Kinds: []int{1},
			Limit: 50,
		})
	}
	lastNotes := []string{}
	relay := []string{"wss://nostr.wine"}
	for _, n := range lastEvents {
		nevent, _ := nip19.EncodeEvent(n.ID, relay, n.PubKey)
		lastNotes = append(lastNotes, nevent)
	}

	err := HomePageTemplate.Render(w, &HomePage{
		HeadCommonPartial: HeadCommonPartial{IsProfile: false},

		Host:      s.Domain,
		Npubs:     npubs,
		LastNotes: lastNotes,
	})
	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
}
