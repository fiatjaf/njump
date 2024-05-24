package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/fiatjaf/eventstore"
	"github.com/fiatjaf/set"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip05"
	"github.com/nbd-wtf/go-nostr/nip19"
	sdk "github.com/nbd-wtf/nostr-sdk"
)

type RelayConfig struct {
	Everything     []string `json:"everything"`
	Profiles       []string `json:"profiles"`
	JustIds        []string `json:"justIds"`
	ExcludedRelays []string `json:"excludeRelays"`
}

func (r *RelayConfig) Valid() bool {
	if len(r.Everything) == 0 || len(r.Profiles) == 0 || len(r.JustIds) == 0 || len(r.ExcludedRelays) == 0 {
		return false
	}
	return true
}

var (
	pool   = nostr.NewSimplePool(context.Background())
	sys    = sdk.System(sdk.WithPool(pool))
	serial int

	relayConfig = RelayConfig{
		Everything: []string{
			"wss://nostr-pub.wellorder.net",
			"wss://relay.damus.io",
			"wss://relay.nostr.bg",
			"wss://nostr.wine",
			"wss://nos.lol",
			"wss://nostr.mom",
			"wss://nostr.land",
			"wss://relay.snort.social",
			"wss://offchain.pub",
			"wss://relay.primal.net",
			"wss://relay.nostr.band",
			"wss://public.relaying.io",
		},
		Profiles: []string{
			"wss://purplepag.es",
			"wss://relay.noswhere.com",
			"wss://relay.nos.social",
			"wss://relay.snort.social",
		},
		JustIds: []string{
			"wss://cache2.primal.net/v1",
			"wss://relay.noswhere.com",
		},
		ExcludedRelays: []string{
			"wss://filter.nostr.wine", // paid
		},
	}

	defaultTrustedPubKeys = []string{
		"7bdef7be22dd8e59f4600e044aa53a1cf975a9dc7d27df5833bc77db784a5805", // dtonon
		"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d", // fiatjaf
		"97c70a44366a6535c145b333f973ea86dfdc2d7a99da618c40c64705ad98e322", // hodlbod
		"ee11a5dff40c19a555f41fe42b48f00e618c91225622ae37b6c2bb67b76c4e49", // Michael Dilger
	}
)

type CachedEvent struct {
	Event  *nostr.Event `json:"e"`
	Relays []string     `json:"r"`
}

func getEvent(ctx context.Context, code string, relayHints []string) (*nostr.Event, []string, error) {
	wdb := eventstore.RelayWrapper{Store: db}

	withRelays := false
	if len(relayHints) > 0 {
		withRelays = true
	}
	priorityRelays := set.NewSliceSet(relayHints...)

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

	switch v := data.(type) {
	case nostr.ProfilePointer:
		author = v.PublicKey
		filter.Authors = []string{v.PublicKey}
		filter.Kinds = []int{0}
		relays = append(relays, relayConfig.Profiles...)
		relays = append(relays, v.Relays...)
		priorityRelays.Add(v.Relays...)
		withRelays = true
	case nostr.EventPointer:
		author = v.Author
		filter.IDs = []string{v.ID}
		relays = append(relays, v.Relays...)
		relays = append(relays, relayConfig.JustIds...)
		priorityRelays.Add(v.Relays...)
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
		relays = append(relays, getRandomRelay(), getRandomRelay())
		relays = append(relays, v.Relays...)
		priorityRelays.Add(v.Relays...)
		withRelays = true
	case string:
		if prefix == "note" {
			filter.IDs = []string{v}
			relays = append(relays, getRandomRelay())
			relays = append(relays, relayConfig.JustIds...)
		} else if prefix == "npub" {
			author = v
			filter.Authors = []string{v}
			filter.Kinds = []int{0}
			relays = append(relays, relayConfig.Profiles...)
		}
	}

	// try to fetch in our internal eventstore first
	if res, _ := wdb.QuerySync(ctx, filter); len(res) != 0 {
		evt := res[0]
		scheduleEventExpiration(evt.ID, time.Hour*24*7)
		return evt, getRelaysForEvent(evt.ID), nil
	}

	// otherwise fetch from external relays
	if author != "" {
		// fetch relays for author
		authorRelays := relaysForPubkey(ctx, author, relays...)
		if len(authorRelays) > 3 {
			authorRelays = authorRelays[:3]
		}
		relays = append(relays, authorRelays...)
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
			successRelays = append(successRelays, ie.Relay.URL)
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
		log.Debug().Str("code", code).Msg("couldn't find")
		return nil, nil, fmt.Errorf("couldn't find this %s, did you include relay or author hints in it?", prefix)
	}

	// save stuff in cache and in internal store
	wdb.Publish(ctx, *result)
	// save relays if we got them
	allRelays := attachRelaysToEvent(result.ID, successRelays...)
	// put priority relays first so they get used in nevent and nprofile
	slices.SortFunc(allRelays, func(a, b string) int {
		if priorityRelays.Has(a) && !priorityRelays.Has(b) {
			return -1
		} else if priorityRelays.Has(b) && !priorityRelays.Has(a) {
			return 1
		}
		return 0
	})
	// keep track of what we have to delete later
	scheduleEventExpiration(result.ID, time.Hour*24*7)

	return result, allRelays, nil
}

func authorLastNotes(ctx context.Context, pubkey string, isSitemap bool) []*nostr.Event {
	limit := 100
	store := true
	useLocalStore := true
	if isSitemap {
		limit = 50000
		store = false
		useLocalStore = false
	}

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindTextNote},
		Authors: []string{pubkey},
		Limit:   limit,
	}
	var lastNotes []*nostr.Event

	// fetch from local store if available
	if useLocalStore {
		lastNotes, _ = eventstore.RelayWrapper{Store: db}.QuerySync(ctx, filter)
	}
	if len(lastNotes) < 5 {
		// if we didn't get enough notes (or if we didn't even query the local store), wait for the external relays
		lastNotes = make([]*nostr.Event, 0, filter.Limit)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		relays := limitAt(relaysForPubkey(ctx, pubkey), 5)
		for len(relays) < 4 {
			relays = unique(append(relays, getRandomRelay()))
		}
		ch := pool.SubManyEose(ctx, relays, nostr.Filters{filter})
	out:
		for {
			select {
			case ie, more := <-ch:
				if !more {
					break out
				}
				lastNotes = append(lastNotes, ie.Event)
				if store {
					db.SaveEvent(ctx, ie.Event)
					attachRelaysToEvent(ie.Event.ID, ie.Relay.URL)
					scheduleEventExpiration(ie.Event.ID, time.Hour*24)
				}
			case <-ctx.Done():
				break out
			}
		}
	}

	// sort before returning
	slices.SortFunc(lastNotes, func(a, b *nostr.Event) int { return int(b.CreatedAt - a.CreatedAt) })
	return lastNotes
}

func relayLastNotes(ctx context.Context, relayUrl string, isSitemap bool) []*nostr.Event {
	key := ""
	limit := 1000
	if isSitemap {
		key = "rlns:" + nostr.NormalizeURL(relayUrl)
		limit = 5000
	} else {
		key = "rln:" + nostr.NormalizeURL(relayUrl)
	}

	lastNotes := make([]*nostr.Event, 0, limit)
	if ok := cache.GetJSON(key, &lastNotes); ok {
		return lastNotes
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*4)
	defer cancel()

	if relay, err := pool.EnsureRelay(relayUrl); err == nil {
		lastNotes, _ = relay.QuerySync(ctx, nostr.Filter{
			Kinds: []int{1},
			Limit: limit,
		})
	}

	slices.SortFunc(lastNotes, func(a, b *nostr.Event) int { return int(b.CreatedAt - a.CreatedAt) })
	if len(lastNotes) > 0 {
		cache.SetJSONWithTTL(key, lastNotes, time.Hour*24)
	}
	return lastNotes
}

func relaysForPubkey(ctx context.Context, pubkey string, extraRelays ...string) []string {
	pubkeyRelays := make([]string, 0, 12)
	if ok := cache.GetJSON("io:"+pubkey, &pubkeyRelays); !ok {
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*1500)
		defer cancel()
		pubkeyRelays = sys.FetchOutboxRelays(ctx, pubkey)
		if len(pubkeyRelays) > 0 {
			cache.SetJSONWithTTL("io:"+pubkey, pubkeyRelays, time.Hour*24*7)
		}
	}
	return unique(pubkeyRelays)
}

func contactsForPubkey(ctx context.Context, pubkey string, extraRelays ...string) []string {
	pubkeyContacts := make([]string, 0, 300)
	relays := make([]string, 0, 12)
	if ok := cache.GetJSON("cc:"+pubkey, &pubkeyContacts); !ok {
		log.Debug().Msgf("searching contacts for %s", pubkey)
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)

		pubkeyRelays := relaysForPubkey(ctx, pubkey, relays...)
		relays = append(relays, pubkeyRelays...)
		relays = append(relays, relayConfig.Profiles...)

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
	return unique(pubkeyContacts)
}
