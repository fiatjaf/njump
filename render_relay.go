package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip10"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderRelayPage(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Path[1:]
	hostname := code[2:]
	typ := "relay"
	if strings.HasSuffix(hostname, ".xml") {
		hostname = hostname[:len(hostname)-4]
		typ = "relay_sitemap"
	}

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
	events_num := 1000
	if typ == "relay_sitemap" {
		events_num = 5000
	}
	var lastNotes []*nostr.Event
	if relay, err := pool.EnsureRelay(hostname); err == nil {
		lastNotes, _ = relay.QuerySync(ctx, nostr.Filter{
			Kinds: []int{1},
			Limit: events_num,
		})
	}
	renderableLastNotes := make([]LastNotesItem, len(lastNotes))
	lastEventAt := time.Now()
	relay := []string{"wss://" + hostname}
	for i, levt := range lastNotes {
		nevent, _ := nip19.EncodeEvent(levt.ID, relay, levt.PubKey)
		npub, _ := nip19.EncodePublicKey(levt.PubKey)
		npubShort := npub[:8] + "…" + npub[len(npub)-4:]
		renderableLastNotes[i] = LastNotesItem{
			Npub:       npub,
			NpubShort:  npubShort,
			Nevent:     nevent,
			Content:    levt.Content,
			CreatedAt:  time.Unix(int64(levt.CreatedAt), 0).Format("2006-01-02 15:04:05"),
			ModifiedAt: time.Unix(int64(levt.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00"),
			IsReply:    nip10.GetImmediateReply(levt.Tags) != nil,
		}
		if i == 0 {
			lastEventAt = time.Unix(int64(levt.CreatedAt), 0)
		}
	}

	params := map[string]any{
		"clients": []ClientReference{
			{Name: "Coracle", URL: "https://coracle.social/relays/" + hostname},
		},
		"type":       "relay",
		"info":       info,
		"hostname":   hostname,
		"proxy":      "https://" + hostname + "/njump/proxy?src=",
		"lastNotes":  renderableLastNotes,
		"modifiedAt": lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if len(renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	if err := tmpls.ExecuteTemplate(w, templateMapping[typ], params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}
}
