package main

import (
	"context"
	"slices"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip86"
)

func setupRelayManagement(relay *khatru.Relay) {
	relay.ManagementAPI.RejectAPICall = append(relay.ManagementAPI.RejectAPICall,
		func(ctx context.Context, mp nip86.MethodParams) (reject bool, msg string) {
			if slices.Contains(s.TrustedPubKeys, khatru.GetAuthed(ctx)) {
				return false, ""
			}
			return true, "you are not a trusted pubkey"
		},
	)
	relay.ManagementAPI.BanEvent = func(ctx context.Context, id, reason string) error {
		log.Info().Str("id", id).Str("reason", reason).Msg("banning event")
		ch, err := sys.Store.QueryEvents(ctx, nostr.Filter{IDs: []string{id}})
		if err != nil {
			return err
		}

		for evt := range ch {
			sys.Store.DeleteEvent(ctx, evt)
		}

		if err := internal.banEvent(id, reason); err != nil {
			return err
		}

		return nil
	}
	relay.ManagementAPI.AllowEvent = func(ctx context.Context, id, reason string) error {
		log.Info().Str("id", id).Str("reason", reason).Msg("unbanning event")
		if err := internal.unbanEvent(id); err != nil {
			return err
		}
		return nil
	}
	relay.ManagementAPI.BanPubKey = func(ctx context.Context, pk, reason string) error {
		log.Info().Str("pubkey", pk).Str("reason", reason).Msg("banning pubkey")
		ch, err := sys.Store.QueryEvents(ctx, nostr.Filter{Authors: []string{pk}, Limit: DB_MAX_LIMIT})
		if err != nil {
			return err
		}

		for evt := range ch {
			sys.Store.DeleteEvent(ctx, evt)
		}

		if err := internal.banPubkey(pk, reason); err != nil {
			return err
		}

		return nil
	}
	relay.ManagementAPI.AllowPubKey = func(ctx context.Context, id, reason string) error {
		log.Info().Str("id", id).Str("reason", reason).Msg("unbanning pubkey")
		if err := internal.unbanPubkey(id); err != nil {
			return err
		}
		return nil
	}
}
