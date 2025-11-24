package main

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/lmdb"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/sdk"
	bolt_kv "fiatjaf.com/nostr/sdk/kvstore/bbolt"
)

type RelayConfig struct {
	Everything []string `json:"everything"`
	Profiles   []string `json:"profiles"`
	JustIds    []string `json:"justIds"`
}

const DB_MAX_LIMIT = 500

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

	defaultTrustedPubKeys = []nostr.PubKey{
		nostr.MustPubKeyFromHex("7bdef7be22dd8e59f4600e044aa53a1cf975a9dc7d27df5833bc77db784a5805"), // dtonon
		nostr.MustPubKeyFromHex("3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"), // fiatjaf
		nostr.MustPubKeyFromHex("97c70a44366a6535c145b333f973ea86dfdc2d7a99da618c40c64705ad98e322"), // hodlbod
		nostr.MustPubKeyFromHex("ee11a5dff40c19a555f41fe42b48f00e618c91225622ae37b6c2bb67b76c4e49"), // Michael Dilger
		nostr.MustPubKeyFromHex("30c25d24b998c6b51253253fd66d7ceccc7e47ae3d8c540d2a914bec77e89b1d"), // ----
	}
)

func isEventBanned(id nostr.ID) (bool, string) {
	for evt := range sys.Store.QueryEvents(nostr.Filter{
		Kinds:   []nostr.Kind{5},
		Authors: s.trustedPubKeys,
		Tags:    nostr.TagMap{"e": []string{id.Hex()}},
	}, DB_MAX_LIMIT) {
		return true, evt.Content
	}
	return false, ""
}

func isPubkeyBanned(pk nostr.PubKey) (bool, string) {
	for evt := range sys.Store.QueryEvents(nostr.Filter{
		Kinds:   []nostr.Kind{5},
		Authors: s.trustedPubKeys,
		Tags:    nostr.TagMap{"p": []string{pk.Hex()}},
	}, DB_MAX_LIMIT) {
		return true, evt.Content
	}
	return false, ""
}

func initSystem() func() {
	db := &lmdb.LMDBBackend{
		Path: s.EventStorePath,
	}
	if err := db.Init(); err != nil {
		log.Fatal().Err(err).Msg("failed to init eventstore")
		return func() {}
	}

	kv, err := bolt_kv.NewStore(s.KVStorePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init kvstore")
		return func() {}
	}

	sys = sdk.NewSystem()
	sys.KVStore = kv
	sys.Store = db

	sys.RelayListRelays = sdk.NewRelayStream("wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social", "wss://relay.vertexlab.io", "wss://indexer.coracle.social")
	sys.FollowListRelays = sdk.NewRelayStream("wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social", "wss://relay.vertexlab.io", "wss://indexer.coracle.social")
	sys.MetadataRelays = sdk.NewRelayStream("wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social", "wss://relay.vertexlab.io", "wss://indexer.coracle.social")
	sys.FallbackRelays = sdk.NewRelayStream(
		"wss://offchain.pub",
		"wss://relay.damus.io",
		"wss://relay.primal.net",
		"wss://nostr.mom",
		"wss://nos.lol",
		"wss://relay.mostr.pub",
		"wss://nostr.wine",
	)
	sys.JustIDRelays = sdk.NewRelayStream(
		"wss://cache2.primal.net/v1",
		"wss://relay.nostr.band",
	)

	return db.Close
}

func getEvent(ctx context.Context, code string) (*nostr.Event, error) {
	var pointer nostr.Pointer
	prefix, data, err := nip19.Decode(code)
	if err == nil {
		switch prefix {
		case "nevent":
			pointer = data.(nostr.EventPointer)
		case "naddr":
			pointer = data.(nostr.EntityPointer)
		case "note":
			pointer = nostr.EventPointer{ID: data.(nostr.ID)}
		default:
			return nil, fmt.Errorf("invalid code '%s'", code)
		}
	} else {
		if id, err := nostr.IDFromHex(code); err == nil {
			pointer = nostr.EventPointer{ID: id}
		} else {
			return nil, fmt.Errorf("failed to decode '%s': %w", code, err)
		}
	}

	// pre-ban before fetching
	var hasCheckedAuthor bool
	var hasCheckedID bool
	var preauthor nostr.PubKey
	switch p := pointer.(type) {
	case nostr.EventPointer:
		if banned, _ := isEventBanned(p.ID); banned {
			deleteEvent(p.ID)
			return nil, fmt.Errorf("event is banned")
		}
		hasCheckedID = true
		preauthor = p.Author
	case nostr.EntityPointer:
		preauthor = p.PublicKey
	case nostr.ProfilePointer:
		preauthor = p.PublicKey
	}
	if preauthor != nostr.ZeroPK {
		if banned, _ := isPubkeyBanned(preauthor); banned {
			deleteAllEventsFromPubKey(preauthor)
			return nil, fmt.Errorf("pubkey is banned")
		}
		hasCheckedAuthor = true
	}

	// first check localstore
	var event *nostr.Event
	for evt := range sys.Store.QueryEvents(pointer.AsFilter(), 1) {
		event = &evt
		break
	}

	// otherwise try the relays
	if event == nil {
		await(ctx)

		evt, _, err := sys.FetchSpecificEvent(ctx, pointer, sdk.FetchSpecificEventParameters{
			SkipLocalStore:   true,
			SaveToLocalStore: true,
		})
		if err != nil {
			return evt, err
		}
		event = evt
	}

	if event != nil {
		// do banned checks again if necessary
		if !hasCheckedAuthor {
			if banned, _ := isPubkeyBanned(event.PubKey); banned {
				deleteAllEventsFromPubKey(event.PubKey)
				return nil, fmt.Errorf("pubkey is banned")
			}
		}
		if !hasCheckedID {
			if banned, _ := isEventBanned(event.ID); banned {
				deleteEvent(event.ID)
				return nil, fmt.Errorf("event is banned")
			}
		}
	}

	return event, err
}

func getMetadata(ctx context.Context, event nostr.Event) sdk.ProfileMetadata {
	if event.Kind == 0 {
		spm, _ := sdk.ParseMetadata(event)
		return spm
	} else {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()
		return sys.FetchProfileMetadata(ctx, event.PubKey)
	}
}

func authorLastNotes(ctx context.Context, pubkey nostr.PubKey) (lastNotes []EnhancedEvent, justFetched bool) {
	limit := 100

	go sys.FetchProfileMetadata(ctx, pubkey) // fetch this before so the cache is filled for later

	filter := nostr.Filter{
		Kinds:   []nostr.Kind{nostr.KindTextNote},
		Authors: []nostr.PubKey{pubkey},
		Limit:   limit,
	}

	lastNotes = make([]EnhancedEvent, 0, filter.Limit)
	latestTimestamp := nostr.Timestamp(0)

	// fetch from local store if available
	next, done := iter.Pull(sys.Store.QueryEvents(filter, DB_MAX_LIMIT))
	evt, has := next()
	if has {
		lastNotes = append(lastNotes, NewEnhancedEvent(ctx, evt))
		latestTimestamp = evt.CreatedAt
		for evt, more := next(); more; evt, more = next() {
			lastNotes = append(lastNotes, NewEnhancedEvent(ctx, evt))
		}
	}
	done()

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

			ch := sys.Pool.FetchMany(ctx, relays, filter, nostr.SubscriptionOptions{Label: "authorlast"})
		out:
			for {
				select {
				case ie, more := <-ch:
					if !more {
						break out
					}

					ee := NewEnhancedEvent(ctx, ie.Event)
					ee.relays = appendUnique([]string{ie.Relay.URL}, sys.GetEventRelays(ie.Event.ID)...)
					lastNotes = append(lastNotes, ee)

					sys.Store.SaveEvent(ie.Event)

					// track this only the first time this event is downloaded for the profile page so we keep these fresh
					sys.TrackEventAccessTime(evt.ID)
				case <-ctx.Done():
					break out
				}
			}
		}()
	}

	return lastNotes, justFetched
}

func relayLastNotes(ctx context.Context, hostname string, limit int) iter.Seq[nostr.Event] {
	ctx, cancel := context.WithTimeout(ctx, time.Second*4)

	url := nostr.NormalizeURL(hostname)
	return func(yield func(nostr.Event) bool) {
		defer cancel()

		for evt := range sys.Store.QueryEvents(nostr.Filter{Kinds: []nostr.Kind{1, 1111}}, 99999) {
			if slices.Contains(sys.GetEventRelays(evt.ID), url) {
				limit--

				if !yield(evt) {
					return
				}
				if limit == 0 {
					return
				}
			}
		}

		if limit > 40 {
			await(ctx)

			limit = max(limit, 50)

			if relay, err := sys.Pool.EnsureRelay(hostname); err == nil {
				for evt := range relay.QueryEvents(nostr.Filter{
					Kinds: []nostr.Kind{1},
					Limit: limit,
				}) {
					sys.Store.SaveEvent(evt)

					// track this only the first time this event is downloaded for the relay page so we keep these fresh
					sys.TrackEventAccessTime(evt.ID)

					if !yield(evt) {
						return
					}
				}
			}
		}
	}
}

func relaysPretty(ctx context.Context, pubkey nostr.PubKey) []string {
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
