package main

import (
	"context"
	"fmt"
	"time"

	"github.com/die-net/lrucache"
	"github.com/mailru/easyjson"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

var (
	pool   = nostr.NewSimplePool(context.Background())
	cache  = lrucache.New(50, 60*60)
	serial int

	always = []string{
		"wss://relay.nostr.band",
	}
	everything = []string{
		"wss://relay.nostr.bg",
		"wss://relay.damus.io",
		"wss://nostr.wine",
		"wss://nos.lol",
		"wss://nostr.mom",
		"wss://atlas.nostr.land",
		"wss://relay.snort.social",
		"wss://eden.nostr.land",
		"wss://nostr-pub.wellorder.net",
	}
	profiles = []string{
		"wss://purplepag.es",
		"wss://rbr.bio",
	}
)

func getRelay() string {
	serial = (serial + 1) % len(everything)
	return everything[serial]
}

func getEvent(ctx context.Context, code string) (*nostr.Event, error) {
	if v, ok := cache.Get(code); ok {
		evt := &nostr.Event{}
		easyjson.Unmarshal(v, evt)
		return evt, nil
	}

	prefix, data, err := nip19.Decode(code)
	if err != nil {
		return nil, fmt.Errorf("failed to decode %w", err)
	}

	var filter nostr.Filter
	relays := make([]string, 0, 7)
	relays = append(relays, always...)

	switch v := data.(type) {
	case nostr.ProfilePointer:
		filter.Authors = []string{v.PublicKey}
		filter.Kinds = []int{0}
		relays = append(relays, profiles...)
		relays = append(relays, v.Relays...)
	case nostr.EventPointer:
		filter.IDs = []string{v.ID}
		relays = append(relays, getRelay())
		relays = append(relays, getRelay())
		relays = append(relays, v.Relays...)
	case nostr.EntityPointer:
		filter.Authors = []string{v.PublicKey}
		filter.Tags = nostr.TagMap{
			"d": []string{v.Identifier},
		}
		relays = append(relays, getRelay())
		relays = append(relays, getRelay())
		relays = append(relays, v.Relays...)
	case string:
		if prefix == "note" {
			filter.IDs = []string{v}
			relays = append(relays, getRelay())
			relays = append(relays, getRelay())
			relays = append(relays, getRelay())
		} else if prefix == "npub" {
			filter.Authors = []string{v}
			filter.Kinds = []int{0}
			relays = append(relays, profiles...)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*4)
	defer cancel()
	for event := range pool.SubManyEose(ctx, relays, nostr.Filters{filter}) {
		cache.Set(code, []byte(event.String()))
		return event, nil
	}

	return nil, fmt.Errorf("couldn't find this %s", prefix)
}

func getLastNotes(ctx context.Context, npub string) ([]nostr.Event, error) {
	var filter nostr.Filters
	relays := make([]string, 0, 7)
	relays = append(relays, always...)
	lastNotes := make([]nostr.Event, 0)
	if _, v, err := nip19.Decode(npub); err == nil {
		pub := v.(string)
		filter = nostr.Filters{
			{
				Kinds:   []int{nostr.KindTextNote},
				Authors: []string{pub},
				Limit:   20,
			},
		}
	} else {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*4)
	defer cancel()
	events := pool.SubManyEose(ctx, relays, filter)
	for event := range events {
		lastNotes = append(lastNotes, *event)
	}
	return lastNotes, nil
}
