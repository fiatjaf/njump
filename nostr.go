package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip05"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/nbd-wtf/go-nostr/nson"
	"github.com/nbd-wtf/go-nostr/sdk"
)

var (
	pool   = nostr.NewSimplePool(context.Background())
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
		"wss://offchain.pub",
		"wss://nostr-pub.wellorder.net",
	}
	profiles = []string{
		"wss://purplepag.es",
	}
)

func getRelay() string {
	serial = (serial + 1) % len(everything)
	return everything[serial]
}

func getEvent(ctx context.Context, code string) (*nostr.Event, error) {
	if b, ok := cache.Get(code); ok {
		v := &nostr.Event{}
		err := nson.Unmarshal(string(b), v)
		return v, err
	}

	prefix, data, err := nip19.Decode(code)
	if err != nil {
		pp, _ := nip05.QueryIdentifier(ctx, code)
		if pp == nil {
			return nil, fmt.Errorf("failed to decode %w", err)
		}
		data = *pp
	}

	var author string

	var filter nostr.Filter
	relays := make([]string, 0, 25)
	relays = append(relays, always...)

	switch v := data.(type) {
	case nostr.ProfilePointer:
		author = v.PublicKey
		filter.Authors = []string{v.PublicKey}
		filter.Kinds = []int{0}
		relays = append(relays, profiles...)
		relays = append(relays, v.Relays...)
	case nostr.EventPointer:
		author = v.Author
		filter.IDs = []string{v.ID}
		relays = append(relays, getRelay())
		relays = append(relays, getRelay())
		relays = append(relays, v.Relays...)
	case nostr.EntityPointer:
		author = v.PublicKey
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
			author = v
			filter.Authors = []string{v}
			filter.Kinds = []int{0}
			relays = append(relays, profiles...)
		}
	}

	if author != "" {
		// fetch relays for author
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*1500)
		defer cancel()

		for _, relay := range sdk.FetchRelaysForPubkey(ctx, pool, author, relays...) {
			if relay.Outbox {
				relays = append(relays, relay.URL)
			}
		}
	}

	for len(relays) < 5 {
		relays = append(relays, getRelay())
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*8)
	defer cancel()
	for event := range pool.SubManyEose(ctx, relays, nostr.Filters{filter}) {
		if !s.DisableCache {
			b, err := nson.Marshal(event)
			if err != nil {
				log.Error().Err(err).Stringer("event", event).Msg("error marshaling nson")
				return event, nil
			}
			cache.SetWithTTL(code, []byte(b), time.Hour*24*7)
		}
		return event, nil
	}

	return nil, fmt.Errorf("couldn't find this %s", prefix)
}

func getLastNotes(ctx context.Context, code string) []*nostr.Event {
	pp := sdk.InputToProfile(ctx, code)
	if pp == nil {
		return nil
	}
	relays := pp.Relays

	{
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*1500)
		defer cancel()
		for _, relay := range sdk.FetchRelaysForPubkey(ctx, pool, pp.PublicKey, pp.Relays...) {
			if relay.Outbox {
				relays = append(relays, relay.URL)
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*4)
	defer cancel()

	relays = append(relays, getRelay())
	relays = append(relays, getRelay())
	events := pool.SubManyEose(ctx, relays, nostr.Filters{
		{
			Kinds:   []int{nostr.KindTextNote},
			Authors: []string{pp.PublicKey},
			Limit:   10,
		},
	})
	lastNotes := make([]*nostr.Event, 0, 20)
	for event := range events {
		lastNotes = nostr.InsertEventIntoDescendingList(lastNotes, event)
	}
	return lastNotes
}
