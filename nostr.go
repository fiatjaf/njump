package main

import (
	"context"
	"fmt"
	"math/rand"
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

	trustedPubKeys = []string{
		"7bdef7be22dd8e59f4600e044aa53a1cf975a9dc7d27df5833bc77db784a5805", // dtonon
		"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d", // fiatjaf
		"97c70a44366a6535c145b333f973ea86dfdc2d7a99da618c40c64705ad98e322", // hodlbod
		"ee11a5dff40c19a555f41fe42b48f00e618c91225622ae37b6c2bb67b76c4e49", // Michael Dilger
	}
)

func getRelay() string {
	if serial == 0 {
		serial = rand.Intn(len(everything))
	}
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
		authorRelays := relaysForPubkey(ctx, author, relays...)
		relays = append(relays, authorRelays...)
	}

	for len(relays) < 5 {
		relays = append(relays, getRelay())
	}

	relays = unique(relays)
	ctx, cancel := context.WithTimeout(ctx, time.Second*8)
	defer cancel()
	for event := range pool.SubManyEose(ctx, relays, nostr.Filters{filter}) {
		b, err := nson.Marshal(event)
		if err != nil {
			log.Error().Err(err).Stringer("event", event).Msg("error marshaling nson")
			return event, nil
		}
		cache.SetWithTTL(code, []byte(b), time.Hour*24*7)
		return event, nil
	}

	return nil, fmt.Errorf("couldn't find this %s", prefix)
}

func getLastNotes(ctx context.Context, code string, limit int) []*nostr.Event {

	if limit <= 0 {
		limit = 10
	}

	pp := sdk.InputToProfile(ctx, code)
	if pp == nil {
		return nil
	}

	pubkeyRelays := relaysForPubkey(ctx, pp.PublicKey, pp.Relays...)
	relays := append(pp.Relays, pubkeyRelays...)

	ctx, cancel := context.WithTimeout(ctx, time.Second*4)
	defer cancel()

	relays = append(relays, getRelay())
	relays = append(relays, getRelay())
	relays = unique(relays)
	events := pool.SubManyEose(ctx, relays, nostr.Filters{
		{
			Kinds:   []int{nostr.KindTextNote},
			Authors: []string{pp.PublicKey},
			Limit:   limit,
		},
	})
	lastNotes := make([]*nostr.Event, 0, 20)
	for event := range events {
		lastNotes = nostr.InsertEventIntoDescendingList(lastNotes, event)
	}

	return lastNotes
}

func relaysForPubkey(ctx context.Context, pubkey string, extraRelays ...string) []string {
	pubkeyRelays := make([]string, 0, 12)
	if ok := cache.GetJSON("io:"+pubkey, &pubkeyRelays); !ok {
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*1500)
		for _, relay := range sdk.FetchRelaysForPubkey(ctx, pool, pubkey, extraRelays...) {
			if relay.Outbox {
				pubkeyRelays = append(pubkeyRelays, relay.URL)
			}
		}
		cancel()
		if len(pubkeyRelays) > 0 {
			cache.SetJSONWithTTL("io:"+pubkey, pubkeyRelays, time.Hour*24*7)
		}
	}
	pubkeyRelays = unique(pubkeyRelays)
	return pubkeyRelays
}

func contactsForPubkey(ctx context.Context, pubkey string, extraRelays ...string) []string {
	pubkeyContacts := make([]string, 0, 100)
	relays := make([]string, 0, 12)
	if ok := cache.GetJSON("cc:"+pubkey, &pubkeyContacts); !ok {
		fmt.Printf("Searching contacts for %s\n", pubkey)
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*1500)

		pubkeyRelays := relaysForPubkey(ctx, pubkey, relays...)
		relays = append(relays, pubkeyRelays...)
		relays = append(relays, always...)
		relays = append(relays, profiles...)

		ch := pool.SubManyEose(ctx, relays, nostr.Filters{
			{
				Kinds:   []int{3},
				Authors: []string{pubkey},
				Limit:   2,
			},
		})

		for event := range ch {
			for _, tag := range event.Tags {
				if tag[0] == "p" {
					pubkeyContacts = append(pubkeyContacts, tag[1])
				}
			}
		}

		cancel()
		if len(pubkeyContacts) > 0 {
			cache.SetJSONWithTTL("cc:"+pubkey, pubkeyContacts, time.Hour*6)
		}
	}
	pubkeyContacts = unique(pubkeyContacts)
	return pubkeyContacts
}
