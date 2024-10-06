package main

import (
	"context"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

var npubsArchive = make([]string, 0, 5000)

func updateArchives(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour * 3):
			log.Debug().Msg("refreshing the npubs archive")

			for _, pubkey := range s.TrustedPubKeys {
				ctx, cancel := context.WithTimeout(ctx, time.Second*4)
				follows := sys.FetchFollowList(ctx, pubkey)
				fla := &FollowListArchive{
					Source:  pubkey,
					Pubkeys: make([]string, 0, 2000),
				}
				for _, follow := range follows.Items {
					fla.Pubkeys = append(fla.Pubkeys, follow.Pubkey)
				}
				cancel()

				if err := internal.overwriteFollowListArchive(fla); err != nil {
					log.Fatal().Err(err).Msg("failed to overwrite archived pubkeys")
				}
			}
		}
	}
}

func deleteOldCachedEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour * 6):
			log.Debug().Msg("deleting old cached events")
			if ids, err := internal.deleteExpiredEvents(nostr.Now()); err != nil {
				log.Fatal().Err(err).Msg("failed to delete expired events")
			} else {
				if ch, err := sys.Store.QueryEvents(ctx, nostr.Filter{IDs: ids}); err != nil {
					log.Fatal().Err(err).Strs("ids", ids).Msg("fail to delete cached events")
				} else {
					for evt := range ch {
						if err := sys.Store.DeleteEvent(ctx, evt); err != nil {
							log.Error().Err(err).Stringer("event", evt).Msg("failed to delete this cached event")
						}
					}
				}
			}
		}
	}
}
