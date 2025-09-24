package main

import (
	"context"
	"slices"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/khatru"
	"fiatjaf.com/nostr/nip86"
)

func setupRelayManagement(relay *khatru.Relay) {
	relay.ManagementAPI.OnAPICall = func(ctx context.Context, mp nip86.MethodParams) (reject bool, msg string) {
		for _, authed := range khatru.GetConnection(ctx).AuthedPublicKeys {
			if slices.Contains(s.trustedPubKeys, authed) {
				return false, ""
			}
		}
		return true, "you are not a trusted pubkey"
	}
	relay.ManagementAPI.BanEvent = func(ctx context.Context, id nostr.ID, reason string) error {
		log.Info().Str("id", id.Hex()).Str("reason", reason).Msg("banning event")
		sys.Store.DeleteEvent(id)

		authed, ok := khatru.GetAuthed(ctx)
		if !ok {
			panic("not authed")
		}

		evt := nostr.Event{
			Kind:      5,
			Tags:      nostr.Tags{{"e", id.Hex()}},
			Content:   reason,
			PubKey:    authed,
			CreatedAt: nostr.Now(),
		}
		evt.ID = evt.GetID()
		return sys.Store.SaveEvent(evt)
	}
	relay.ManagementAPI.AllowEvent = func(ctx context.Context, id nostr.ID, reason string) error {
		log.Info().Str("id", id.Hex()).Str("reason", reason).Msg("unbanning event")
		for evt := range sys.Store.QueryEvents(nostr.Filter{
			Kinds:   []nostr.Kind{5},
			Tags:    nostr.TagMap{"e": []string{id.Hex()}},
			Authors: s.trustedPubKeys,
		}, DB_MAX_LIMIT) {
			sys.Store.DeleteEvent(evt.ID)
		}
		return nil
	}
	relay.ManagementAPI.BanPubKey = func(ctx context.Context, pk nostr.PubKey, reason string) error {
		log.Info().Str("pubkey", pk.Hex()).Str("reason", reason).Msg("banning pubkey")

		for evt := range sys.Store.QueryEvents(nostr.Filter{Authors: []nostr.PubKey{pk}}, DB_MAX_LIMIT) {
			sys.Store.DeleteEvent(evt.ID)
		}

		authed, ok := khatru.GetAuthed(ctx)
		if !ok {
			panic("not authed")
		}

		evt := nostr.Event{
			Kind:      5,
			Tags:      nostr.Tags{{"p", pk.Hex()}},
			Content:   reason,
			PubKey:    authed,
			CreatedAt: nostr.Now(),
		}
		evt.ID = evt.GetID()
		return sys.Store.SaveEvent(evt)
	}
	relay.ManagementAPI.AllowPubKey = func(ctx context.Context, pk nostr.PubKey, reason string) error {
		log.Info().Str("pk", pk.Hex()).Str("reason", reason).Msg("unbanning pubkey")
		for evt := range sys.Store.QueryEvents(nostr.Filter{
			Kinds:   []nostr.Kind{5},
			Tags:    nostr.TagMap{"p": []string{pk.Hex()}},
			Authors: s.trustedPubKeys,
		}, DB_MAX_LIMIT) {
			sys.Store.DeleteEvent(evt.ID)
		}
		return nil
	}
}
