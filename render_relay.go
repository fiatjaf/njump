package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderRelayPage(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Path[1:]

	hostname := code

	fmt.Println("hostname", hostname)

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	// relay metadata
	info, _ := nip11.Fetch(r.Context(), hostname)
	if info == nil {
		info = &nip11.RelayInformationDocument{
			Name: hostname,
		}
	}

	// last notes
	var lastNotes []*nostr.Event
	if relay, err := pool.EnsureRelay(hostname); err == nil {
		lastNotes, _ = relay.QuerySync(ctx, nostr.Filter{
			Kinds: []int{1},
			Limit: 50,
		})
	}
	renderableLastNotes := make([]*Event, len(lastNotes))
	for i, n := range lastNotes {
		nevent, _ := nip19.EncodeEvent(n.ID, []string{}, n.PubKey)
		date := time.Unix(int64(n.CreatedAt), 0).Format("2006-01-02 15:04:05")
		renderableLastNotes[i] = &Event{
			Nevent:       nevent,
			Content:      n.Content,
			CreatedAt:    date,
			ParentNevent: getParentNevent(n),
		}
	}

	params := map[string]any{
		"clients": []ClientReference{
			{Name: "Coracle", URL: "https://coracle.social/relays/" + hostname},
		},
		"type":      "relay",
		"info":      info,
		"hostname":  hostname,
		"proxy":     "https://" + hostname + "/njump/proxy?src=",
		"lastNotes": renderableLastNotes,
	}

	// +build !nocache
	w.Header().Set("Cache-Control", "max-age=604800")
	
	if err := tmpl.ExecuteTemplate(w, templateMapping["relay"], params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}
}
