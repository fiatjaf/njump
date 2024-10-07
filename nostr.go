package main

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"sync"
	"time"

	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/nbd-wtf/go-nostr/sdk"
	cache_memory "github.com/nbd-wtf/go-nostr/sdk/cache/memory"
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

func initSystem() func() {
	db := &lmdb.LMDBBackend{
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
		filter.Authors = []string{v.PublicKey}
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
	if res, _ := sys.StoreRelay.QuerySync(ctx, filter); len(res) != 0 {
		evt := res[0]
		return evt, internal.getRelaysForEvent(evt.ID), nil
	}

	if author != "" {
		// fetch relays for author
		authorRelays := sys.FetchOutboxRelays(ctx, author, 3)
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

		for ie := range sys.Pool.SubManyEoseNonUnique(
			subManyCtx,
			relays,
			nostr.Filters{filter},
			nostr.WithLabel("fetching-"+prefix),
		) {
			fetchProfileOnce.Do(func() {
				go sys.FetchProfileMetadata(ctx, ie.PubKey)
			})

			successRelays = append(successRelays, ie.Relay.URL)
			if result == nil || ie.CreatedAt > result.CreatedAt {
				result = ie.Event
			}
			countdown = min(countdown, 1)
		}
	}

	if result == nil {
		return nil, nil, fmt.Errorf("couldn't find this %s, did you include relay or author hints in it?", prefix)
	}

	// save stuff in cache and in internal store
	sys.StoreRelay.Publish(ctx, *result)
	// save relays if we got them
	allRelays := internal.attachRelaysToEvent(result.ID, successRelays...)
	// put priority relays first so they get used in nevent and nprofile
	slices.SortFunc(allRelays, func(a, b string) int {
		vpa, _ := priorityRelays[a]
		vpb, _ := priorityRelays[b]
		return vpb - vpa
	})

	return result, allRelays, nil
}

func authorLastNotes(ctx context.Context, pubkey string) []EnhancedEvent {
	limit := 100
	go sys.FetchProfileMetadata(ctx, pubkey) // fetch this before so the cache is filled for later

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindTextNote},
		Authors: []string{pubkey},
		Limit:   limit,
	}

	lastNotes := make([]EnhancedEvent, 0, filter.Limit)

	// fetch from local store if available
	ch, err := sys.Store.QueryEvents(ctx, filter)
	if err == nil {
		for evt := range ch {
			lastNotes = append(lastNotes, NewEnhancedEvent(ctx, evt))
		}
	}

	if len(lastNotes) < 5 {
		// if we didn't get enough notes (or if we didn't even query the local store), wait for the external relays
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		relays := sys.FetchOutboxRelays(ctx, pubkey, 3)

		for len(relays) < 3 {
			relays = unique(append(relays, getRandomRelay()))
		}

		ch := sys.Pool.SubManyEose(ctx, relays, nostr.Filters{filter}, nostr.WithLabel("authorlast"))
	out:
		for {
			select {
			case ie, more := <-ch:
				if !more {
					break out
				}

				ee := NewEnhancedEvent(ctx, ie.Event)
				ee.relays = unique(append([]string{ie.Relay.URL}, internal.getRelaysForEvent(ie.Event.ID)...))
				lastNotes = append(lastNotes, ee)

				sys.Store.SaveEvent(ctx, ie.Event)
				internal.attachRelaysToEvent(ie.Event.ID, ie.Relay.URL)
			case <-ctx.Done():
				break out
			}
		}
	}

	// sort before returning
	slices.SortFunc(lastNotes, func(a, b EnhancedEvent) int { return int(b.CreatedAt - a.CreatedAt) })
	return lastNotes
}

func relayLastNotes(ctx context.Context, hostname string, limit int) iter.Seq[*nostr.Event] {
	ctx, cancel := context.WithTimeout(ctx, time.Second*4)

	return func(yield func(*nostr.Event) bool) {
		defer cancel()

		for id := range internal.getEventsInRelay(hostname) {
			res, _ := sys.StoreRelay.QuerySync(ctx, nostr.Filter{IDs: []string{id}})
			if len(res) == 0 {
				internal.notCached(id)
				continue
			}
			limit--
			if !yield(res[0]) {
				return
			}
			if limit == 0 {
				return
			}
		}

		if limit > 0 {
			limit = max(limit, 50)

			if relay, err := sys.Pool.EnsureRelay(hostname); err == nil {
				ch, err := relay.QueryEvents(ctx, nostr.Filter{
					Kinds: []int{1},
					Limit: limit,
				})
				if err != nil {
					log.Error().Err(err).Stringer("relay", relay).Msg("failed to fetch relay notes")
					return
				}

				for evt := range ch {
					sys.StoreRelay.Publish(ctx, *evt)
					internal.attachRelaysToEvent(evt.ID, hostname)
					if !yield(evt) {
						return
					}
				}
			}
		}
	}
}

func relaysPretty(ctx context.Context, pubkey string) []string {
	s := make([]string, 0, 3)
	for _, url := range sys.FetchOutboxRelays(ctx, pubkey, 3) {
		trimmed := trimProtocolAndEndingSlash(url)
		if slices.Contains(s, trimmed) {
			continue
		}
		s = append(s, trimmed)
	}
	return s
}
