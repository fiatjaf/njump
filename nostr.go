package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/fiatjaf/eventstore/badger"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	sdk "github.com/nbd-wtf/nostr-sdk"
	cache_memory "github.com/nbd-wtf/nostr-sdk/cache/memory"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type RelayConfig struct {
	Everything []string `json:"everything"`
	Profiles   []string `json:"profiles"`
	JustIds    []string `json:"justIds"`
}

var (
	sys    *sdk.System
	serial int

	relayConfig = RelayConfig{
		Everything: nil, // use the defaults from nostr-sdk
		Profiles:   nil, // use the defaults from nostr-sdk
		JustIds: []string{
			"wss://cache2.primal.net/v1",
			"wss://relay.noswhere.com",
			"wss://relay.damus.io",
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

func initSystem() func() {
	db := &badger.BadgerBackend{
		Path: s.EventStorePath,
	}
	db.Init()

	sys = sdk.NewSystem(
		sdk.WithMetadataCache(cache_memory.New32[sdk.ProfileMetadata](10000)),
		sdk.WithRelayListCache(cache_memory.New32[sdk.RelayList](10000)),
		sdk.WithStore(db),
	)

	return db.Close
}

func getEvent(ctx context.Context, code string) (*nostr.Event, []string, error) {
	ctx, span := tracer.Start(ctx, "get-event", trace.WithAttributes(attribute.String("code", code)))
	defer span.End()

	// this is for deciding what relays will go on nevent and nprofile later
	priorityRelays := make(map[string]int)

	prefix, data, err := nip19.Decode(code)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode %w", err)
	}

	author := ""
	authorRelaysPosition := 0

	var filter nostr.Filter
	relays := make([]string, 0, 10)

	switch v := data.(type) {
	case nostr.EventPointer:
		author = v.Author
		filter.IDs = []string{v.ID}
		relays = append(relays, v.Relays...)
		relays = append(relays, relayConfig.JustIds...)
		authorRelaysPosition = len(v.Relays) // ensure author relays are checked after hinted relays
		for _, r := range v.Relays {
			priorityRelays[r] = 2
		}
	case nostr.EntityPointer:
		author = v.PublicKey
		filter.Tags = nostr.TagMap{
			"d": []string{v.Identifier},
		}
		if v.Kind != 0 {
			filter.Kinds = append(filter.Kinds, v.Kind)
		}
		relays = append(relays, v.Relays...)
		authorRelaysPosition = len(v.Relays) // ensure author relays are checked after hinted relays
	case string:
		if prefix == "note" {
			filter.IDs = []string{v}
			relays = append(relays, relayConfig.JustIds...)
		}
	}

	// try to fetch in our internal eventstore first
	ctx, span = tracer.Start(ctx, "query-eventstore")
	if res, _ := sys.StoreRelay.QuerySync(ctx, filter); len(res) != 0 {
		evt := res[0]

		// keep this event in cache for a while more
		// unless it's a metadata event
		// (people complaining about njump keeping their metadata will try to load their metadata all the time)
		if evt.Kind != 0 {
			scheduleEventExpiration(evt.ID, time.Hour*24*7)
		}

		return evt, getRelaysForEvent(evt.ID), nil
	}
	span.End()

	if author != "" {
		// fetch relays for author
		ctx, span = tracer.Start(ctx, "fetch-outbox-relays")
		authorRelays := sys.FetchOutboxRelays(ctx, author, 3)
		span.End()
		relays = slices.Insert(relays, authorRelaysPosition, authorRelays...)
		for _, r := range authorRelays {
			priorityRelays[r] = 1
		}
	}

	for len(relays) < 5 {
		relays = append(relays, getRandomRelay())
	}

	relays = unique(relays)

	var result *nostr.Event
	var successRelays []string = nil

	{
		// actually fetch the event here
		subManyCtx, cancel := context.WithTimeout(ctx, time.Second*8)
		defer cancel()

		// keep track of where we have actually found the event so we can show that
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

		fetchProfileOnce := sync.Once{}

		ctx, span = tracer.Start(subManyCtx, "sub-many-eose-non-unique")
		for ie := range sys.Pool.SubManyEoseNonUnique(subManyCtx, relays, nostr.Filters{filter}) {
			fetchProfileOnce.Do(func() {
				go sys.FetchProfileMetadata(ctx, ie.PubKey)
			})

			successRelays = append(successRelays, ie.Relay.URL)
			if result == nil || ie.CreatedAt > result.CreatedAt {
				result = ie.Event
			}
			countdown = min(countdown, 1)
		}
		span.End()
	}

	if result == nil {
		log.Debug().Str("code", code).Msg("couldn't find")
		return nil, nil, fmt.Errorf("couldn't find this %s, did you include relay or author hints in it?", prefix)
	}

	// save stuff in cache and in internal store
	ctx, span = tracer.Start(ctx, "save-local")
	sys.StoreRelay.Publish(ctx, *result)
	span.End()
	// save relays if we got them
	allRelays := attachRelaysToEvent(result.ID, successRelays...)
	// put priority relays first so they get used in nevent and nprofile
	slices.SortFunc(allRelays, func(a, b string) int {
		vpa, _ := priorityRelays[a]
		vpb, _ := priorityRelays[b]
		return vpb - vpa
	})
	// keep track of what we have to delete later
	scheduleEventExpiration(result.ID, time.Hour*24*7)

	return result, allRelays, nil
}

func authorLastNotes(ctx context.Context, pubkey string, isSitemap bool) []EnhancedEvent {
	ctx, span := tracer.Start(ctx, "author-last-notes")
	defer span.End()

	var limit int
	var store bool
	var useLocalStore bool

	if isSitemap {
		limit = 50000
		store = false
		useLocalStore = false
	} else {
		limit = 100
		store = true
		useLocalStore = true
		go sys.FetchProfileMetadata(ctx, pubkey) // fetch this before so the cache is filled for later
	}

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindTextNote},
		Authors: []string{pubkey},
		Limit:   limit,
	}

	lastNotes := make([]EnhancedEvent, 0, filter.Limit)

	// fetch from local store if available
	if useLocalStore {
		ch, err := sys.Store.QueryEvents(ctx, filter)
		if err == nil {
			for evt := range ch {
				lastNotes = append(lastNotes, NewEnhancedEvent(ctx, evt))
				if store {
					sys.Store.SaveEvent(ctx, evt)
					scheduleEventExpiration(evt.ID, time.Hour*24)
				}
			}
		}
	}
	if len(lastNotes) < 5 {
		// if we didn't get enough notes (or if we didn't even query the local store), wait for the external relays
		ctx, span = tracer.Start(ctx, "querying-external")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		_, span2 := tracer.Start(ctx, "fetch-outbox-relays-for-author-last-notes")
		relays := sys.FetchOutboxRelays(ctx, pubkey, 3)
		span2.End()

		for len(relays) < 3 {
			relays = unique(append(relays, getRandomRelay()))
		}

		ctx, span2 = tracer.Start(ctx, "sub-many-eose")
		ch := sys.Pool.SubManyEose(ctx, relays, nostr.Filters{filter})
		span2.End()
	out:
		for {
			select {
			case ie, more := <-ch:
				if !more {
					break out
				}

				ee := NewEnhancedEvent(ctx, ie.Event)
				ee.relays = unique(append([]string{ie.Relay.URL}, getRelaysForEvent(ie.Event.ID)...))
				lastNotes = append(lastNotes, ee)

				if store {
					sys.Store.SaveEvent(ctx, ie.Event)
					attachRelaysToEvent(ie.Event.ID, ie.Relay.URL)
					scheduleEventExpiration(ie.Event.ID, time.Hour*24)
				}
			case <-ctx.Done():
				break out
			}
		}
		span.End()
	}

	// sort before returning
	slices.SortFunc(lastNotes, func(a, b EnhancedEvent) int { return int(b.CreatedAt - a.CreatedAt) })
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

	if relay, err := sys.Pool.EnsureRelay(relayUrl); err == nil {
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

func contactsForPubkey(ctx context.Context, pubkey string) []string {
	pubkeyContacts := make([]string, 0, 300)
	relays := make([]string, 0, 12)
	if ok := cache.GetJSON("cc:"+pubkey, &pubkeyContacts); !ok {
		log.Debug().Msgf("searching contacts for %s", pubkey)
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)

		pubkeyRelays := sys.FetchOutboxRelays(ctx, pubkey, 3)
		relays = append(relays, pubkeyRelays...)
		relays = append(relays, sys.MetadataRelays...)

		ch := sys.Pool.SubManyEose(ctx, relays, nostr.Filters{
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

func relaysPretty(ctx context.Context, pubkey string) []string {
	s := make([]string, 0, 3)
	ctx, span := tracer.Start(ctx, "author-relays-pretty")
	defer span.End()
	for _, url := range sys.FetchOutboxRelays(ctx, pubkey, 3) {
		trimmed := trimProtocolAndEndingSlash(url)
		if slices.Contains(s, trimmed) {
			continue
		}
		s = append(s, trimmed)
	}
	return s
}
