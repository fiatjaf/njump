package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip05"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/nbd-wtf/go-nostr/sdk"
	"golang.org/x/exp/slices"
)

var (
	pool   = nostr.NewSimplePool(context.Background())
	serial int

	always = []string{
		"wss://relay.nostr.band",
	}
	everything = []string{
		"wss://nostr-pub.wellorder.net",
		"wss://relay.damus.io",
		"wss://relay.nostr.bg",
		"wss://nostr.wine",
		"wss://nos.lol",
		"wss://nostr.mom",
		"wss://atlas.nostr.land",
		"wss://relay.snort.social",
		"wss://offchain.pub",
		"wss://relay.primal.net",
		"wss://public.relaying.io",
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

	excludedRelays = []string{
		"wss://filter.nostr.wine", // paid
	}
)

type CachedEvent struct {
	Event  *nostr.Event `json:"e"`
	Relays []string     `json:"r"`
}

func getRelay() string {
	if serial == 0 {
		serial = rand.Intn(len(everything))
	}
	serial = (serial + 1) % len(everything)
	return everything[serial]
}

func getEvent(ctx context.Context, code string, relayHints []string) (*nostr.Event, []string, error) {
	if b, ok := cache.Get(code); ok {
		v := CachedEvent{}
		err := json.Unmarshal(b, &v)
		return v.Event, v.Relays, err
	}

	withRelays := false

	if len(relayHints) > 0 {
		withRelays = true
	}

	prefix, data, err := nip19.Decode(code)
	if err != nil {
		pp, _ := nip05.QueryIdentifier(ctx, code)
		if pp == nil {
			return nil, nil, fmt.Errorf("failed to decode %w", err)
		}
		data = *pp
	}

	var author string

	var filter nostr.Filter
	relays := make([]string, 0, 25)
	relays = append(relays, relayHints...)
	relays = append(relays, always...)

	switch v := data.(type) {
	case nostr.ProfilePointer:
		author = v.PublicKey
		filter.Authors = []string{v.PublicKey}
		filter.Kinds = []int{0}
		relays = append(relays, profiles...)
		relays = append(relays, v.Relays...)
		withRelays = true
	case nostr.EventPointer:
		author = v.Author
		filter.IDs = []string{v.ID}
		relays = append(relays, getRelay())
		relays = append(relays, getRelay())
		relays = append(relays, v.Relays...)
		withRelays = true
	case nostr.EntityPointer:
		author = v.PublicKey
		filter.Authors = []string{v.PublicKey}
		filter.Tags = nostr.TagMap{
			"d": []string{v.Identifier},
		}
		if v.Kind != 0 {
			filter.Kinds = append(filter.Kinds, v.Kind)
		}
		relays = append(relays, getRelay())
		relays = append(relays, getRelay())
		relays = append(relays, v.Relays...)
		withRelays = true
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
		if len(authorRelays) > 5 {
			authorRelays = authorRelays[:5]
		}
		relays = append(relays, authorRelays...)
	}

	for len(relays) < 5 {
		relays = append(relays, getRelay())
	}

	relays = unique(relays)
	ctx, cancel := context.WithTimeout(ctx, time.Second*8)
	defer cancel()

	// actually fetch the event here
	var result *nostr.Event
	var successRelays []string = nil
	if withRelays {
		successRelays = make([]string, 0, len(relays))
		countdown := 7.5
		go func() {
			for {
				time.Sleep(500 * time.Millisecond)
				if countdown <= 0 {
					cancel()
					break
				}
				countdown -= 0.5
			}
		}()

		for ie := range pool.SubManyEoseNonUnique(ctx, relays, nostr.Filters{filter}) {
			if pu, err := url.Parse(ie.Relay.URL); err == nil {
				successRelays = append(successRelays, pu.Host+pu.RawPath)
			}
			result = ie.Event
			countdown = min(countdown, 1)
		}
	} else {
		ie := pool.QuerySingle(ctx, relays, filter)
		if ie != nil {
			result = ie.Event
		}
	}

	if result == nil {
		return nil, nil, fmt.Errorf("couldn't find this %s", prefix)
	}

	cache.SetJSONWithTTL(code, CachedEvent{Event: result, Relays: successRelays}, time.Hour*24*7)
	return result, successRelays, nil
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

	ch := pool.SubManyEose(ctx, relays, nostr.Filters{
		{
			Kinds:   []int{nostr.KindTextNote},
			Authors: []string{pp.PublicKey},
			Limit:   limit,
		},
	})

	lastNotes := make([]*nostr.Event, 0, 20)
	for {
		select {
		case ie, more := <-ch:
			if !more {
				goto end
			}
			lastNotes = append(lastNotes, ie.Event)
		case <-ctx.Done():
			goto end
		}
	}

end:
	slices.SortFunc(lastNotes, func(a, b *nostr.Event) bool { return a.CreatedAt > b.CreatedAt })

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
		log.Debug().Msgf("searching contacts for %s", pubkey)
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)

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

		for {
			select {
			case evt, more := <-ch:
				if !more {
					goto end
				}
				for _, tag := range evt.Tags {
					if tag[0] == "p" {
						pubkeyContacts = append(pubkeyContacts, tag[1])
					}
				}
			case <-ctx.Done():
				goto end
			}
		}

	end:
		cancel()
		if len(pubkeyContacts) > 0 {
			cache.SetJSONWithTTL("cc:"+pubkey, pubkeyContacts, time.Hour*6)
		}
	}
	pubkeyContacts = unique(pubkeyContacts)
	return pubkeyContacts
}
