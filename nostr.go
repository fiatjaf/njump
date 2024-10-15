package main

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"time"

	"github.com/fiatjaf/eventstore/lmdb"
	"github.com/nbd-wtf/go-nostr"
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

func getEvent(ctx context.Context, code string, withRelays bool) (*nostr.Event, []string, error) {
	evt, relays, err := sys.FetchSpecificEvent(ctx, code, withRelays)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't find this event, did you include accurate relay or author hints in it?")
	}

	if !withRelays {
		return evt, nil, nil
	}

	if relays == nil {
		return evt, internal.getRelaysForEvent(evt.ID), nil
	}

	// save relays if we got them
	allRelays := internal.attachRelaysToEvent(evt.ID, relays...)

	return evt, allRelays, nil
}

func authorLastNotes(ctx context.Context, pubkey string) (lastNotes []EnhancedEvent, justFetched bool) {
	limit := 100

	go sys.FetchProfileMetadata(ctx, pubkey) // fetch this before so the cache is filled for later

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindTextNote},
		Authors: []string{pubkey},
		Limit:   limit,
	}

	lastNotes = make([]EnhancedEvent, 0, filter.Limit)
	latestTimestamp := nostr.Timestamp(0)

	// fetch from local store if available
	ch, err := sys.Store.QueryEvents(ctx, filter)
	if err == nil {
		evt, has := <-ch
		if has {
			lastNotes = append(lastNotes, NewEnhancedEvent(ctx, evt))
			latestTimestamp = evt.CreatedAt
			for evt = range ch {
				lastNotes = append(lastNotes, NewEnhancedEvent(ctx, evt))
			}
		}
	}

	if (len(lastNotes) < limit/10) ||
		(len(lastNotes) < limit/5 && latestTimestamp > nostr.Now()-60*60*24*2) ||
		(len(lastNotes) < limit/2 && latestTimestamp < nostr.Now()-60*60*24*2) {
		// if we didn't get enough notes then try to fetch from external relays (but do not wait for it)
		justFetched = true

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
			defer cancel()

			relays := sys.FetchOutboxRelays(ctx, pubkey, 3)
			for len(relays) < 3 {
				relays = appendUnique(relays, sys.FallbackRelays.Next())
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
					ee.relays = appendUnique([]string{ie.Relay.URL}, internal.getRelaysForEvent(ie.Event.ID)...)
					lastNotes = append(lastNotes, ee)

					sys.Store.SaveEvent(ctx, ie.Event)
					internal.attachRelaysToEvent(ie.Event.ID, ie.Relay.URL)
				case <-ctx.Done():
					break out
				}
			}
		}()
	}

	return lastNotes, justFetched
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
