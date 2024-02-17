package main

import (
	"context"
	"strings"
	"time"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
)

func updateArchives(ctx context.Context) {
	// do this so we don't run this every time we restart it locally

	time.Sleep(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
			loadNpubsArchive(ctx)
			loadRelaysArchive(ctx)
		}
	}
}

func deleteOldCachedEvents(ctx context.Context) {
	wdb := eventstore.RelayWrapper{Store: db}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour):
		}
		log.Debug().Msg("deleting old cached events")
		now := time.Now().Unix()
		for _, key := range cache.GetPaginatedKeys("ttl:", 1, 500) {
			spl := strings.Split(key, ":")
			if len(spl) != 2 {
				log.Error().Str("key", key).Msg("broken 'ttl:' key")
				continue
			}

			var expires int64
			if ok := cache.GetJSON(key, &expires); !ok {
				log.Error().Str("key", key).Msg("failed to get 'ttl:' key")
				continue
			}

			if expires < now {
				// time to delete this
				id := spl[1]
				res, _ := wdb.QuerySync(ctx, nostr.Filter{IDs: []string{id}})
				if len(res) > 0 {
					log.Debug().Msgf("deleting %s", res[0].ID)
					if err := db.DeleteEvent(ctx, res[0]); err != nil {
						log.Warn().Err(err).Stringer("event", res[0]).Msg("failed to delete")
					}
				}
				cache.Delete(key)
			}
		}
	}
}

func loadNpubsArchive(ctx context.Context) {
	log.Debug().Msg("refreshing the npubs archive")

	contactsArchive := make([]string, 0, 500)
	for _, pubkey := range s.TrustedPubKeys {
		ctx, cancel := context.WithTimeout(ctx, time.Second*4)
		pubkeyContacts := contactsForPubkey(ctx, pubkey)
		contactsArchive = append(contactsArchive, pubkeyContacts...)
		cancel()
	}

	for _, contact := range unique(contactsArchive) {
		log.Debug().Msgf("adding contact %s", contact)
		cache.SetWithTTL("pa:"+contact, nil, time.Hour*24*90)
	}
}

func loadRelaysArchive(ctx context.Context) {
	log.Debug().Msg("refreshing the relays archive")

	relaysArchive := make([]string, 0, 500)

	for _, pubkey := range s.TrustedPubKeys {
		ctx, cancel := context.WithTimeout(ctx, time.Second*4)
		pubkeyContacts := relaysForPubkey(ctx, pubkey, relayConfig.Profiles...)
		relaysArchive = append(relaysArchive, pubkeyContacts...)
		cancel()
	}

	for _, relay := range unique(relaysArchive) {
		for _, excluded := range relayConfig.ExcludedRelays {
			if strings.Contains(relay, excluded) {
				log.Debug().Msgf("skipping relay %s", relay)
				continue
			}
		}
		if strings.Contains(relay, "/npub1") {
			continue // skip relays with personalyzed query like filter.nostr.wine
		}
		log.Debug().Msgf("adding relay %s", relay)
		cache.SetWithTTL("ra:"+relay, nil, time.Hour*24*7)
	}
}
