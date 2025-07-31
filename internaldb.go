package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"iter"
	"slices"
	"time"

	"fiatjaf.com/leafdb"
	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/sdk"
	"google.golang.org/protobuf/proto"
)

const (
	TypeCachedEvent       leafdb.DataType = 0
	TypeFollowListArchive leafdb.DataType = 3
	TypePubKeyArchive     leafdb.DataType = 4
	TypeEventInRelay      leafdb.DataType = 5
	TypeBannedEvent       leafdb.DataType = 6
	TypeBannedPubkey      leafdb.DataType = 7
)

func NewInternalDB(path string) (*InternalDB, error) {
	ldb, err := leafdb.New(path, leafdb.Options[proto.Message]{
		Encode: func(t leafdb.DataType, msg proto.Message) ([]byte, error) {
			return proto.Marshal(msg)
		},
		Decode: func(t leafdb.DataType, buf []byte) (proto.Message, error) {
			var v proto.Message
			switch t {
			case TypeCachedEvent:
				v = &CachedEvent{}
			case TypeFollowListArchive:
				v = &FollowListArchive{}
			case TypePubKeyArchive:
				v = &PubKeyArchive{}
			case TypeEventInRelay:
				v = &ID{}
			case TypeBannedEvent:
				v = &BannedEvent{}
			case TypeBannedPubkey:
				v = &BannedPubkey{}
			default:
				return nil, fmt.Errorf("what is this? %v", t)
			}
			err := proto.Unmarshal(buf, v)
			return v, err
		},
		Indexes: map[string]leafdb.IndexDefinition[proto.Message]{
			"expiring-when": {
				Version: 1,
				Types:   []leafdb.DataType{TypeCachedEvent},
				Emit: func(t leafdb.DataType, data proto.Message, emit func([]byte)) {
					ee := data.(*CachedEvent)
					emit(binary.BigEndian.AppendUint32(nil, uint32(ee.Expiry)))
				},
			},
			"cached-id": {
				Version: 1,
				Types:   []leafdb.DataType{TypeCachedEvent},
				Emit: func(t leafdb.DataType, data proto.Message, emit func([]byte)) {
					ee := data.(*CachedEvent)
					internal, err := hex.DecodeString(ee.Id[0:16])
					if err != nil {
						log.Fatal().Err(err).Str("id", ee.Id).Msg("failed to decode event id hex")
						return
					}
					emit(internal)
				},
			},
			"follow-list-by-source": {
				Version: 1,
				Types:   []leafdb.DataType{TypeFollowListArchive},
				Emit: func(t leafdb.DataType, value proto.Message, emit func([]byte)) {
					fla := value.(*FollowListArchive)
					pkb, _ := hex.DecodeString(fla.Source[0:16])
					emit(pkb)
				},
			},
			"banned-event": {
				Version: 1,
				Types:   []leafdb.DataType{TypeBannedEvent},
				Emit: func(t leafdb.DataType, value proto.Message, emit func([]byte)) {
					ban := value.(*BannedEvent)
					emit(ban.Id[0:8])
				},
			},
			"banned-pubkey": {
				Version: 1,
				Types:   []leafdb.DataType{TypeBannedPubkey},
				Emit: func(t leafdb.DataType, value proto.Message, emit func([]byte)) {
					ban := value.(*BannedPubkey)
					emit(ban.Pk[0:8])
				},
			},
		},
		Views: map[string]leafdb.ViewDefinition[proto.Message]{
			"pubkey-archive": {
				Version: 1,
				Types:   []leafdb.DataType{TypeFollowListArchive},
				Emit: func(t leafdb.DataType, value proto.Message, emit func(idxkey []byte, t leafdb.DataType, value proto.Message)) {
					fla := value.(*FollowListArchive)
					for _, pubkey := range fla.Pubkeys {
						emit([]byte{1}, TypePubKeyArchive, &PubKeyArchive{Pubkey: pubkey})
					}
				},
			},
			"events-in-relay": {
				Version: 1,
				Types:   []leafdb.DataType{TypeCachedEvent},
				Emit: func(t leafdb.DataType, value proto.Message, emit func(idxkey []byte, t leafdb.DataType, value proto.Message)) {
					ee := value.(*CachedEvent)
					for _, r := range ee.Relays {
						emit([]byte(trimProtocolAndEndingSlash(r)), TypeEventInRelay, &ID{Id: ee.Id})
					}
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return &InternalDB{ldb}, err
}

type InternalDB struct {
	*leafdb.DB[proto.Message]
}

func (internal *InternalDB) scheduleEventExpiration(eventId nostr.ID) {
	if err := internal.UpdateQuery(
		leafdb.PrefixQuery("cached-id", eventId[0:8]),
		func(t leafdb.DataType, data proto.Message) (proto.Message, error) {
			ee := data.(*CachedEvent)
			ee.Expiry = time.Now().Add(time.Hour * 24 * 7).Unix()
			return ee, nil
		},
	); err != nil {
		log.Fatal().Err(err).Msg("failed to update scheduled expirations")
	}
}

func (internal *InternalDB) deleteExpiredEvents(now nostr.Timestamp) (eventIds []nostr.ID, err error) {
	deleted, err := internal.DB.DeleteQuery(leafdb.QueryParams{
		Index:    "expiring-when",
		StartKey: []byte{0},
		EndKey:   binary.BigEndian.AppendUint32(nil, uint32(now)),
	})
	if err != nil {
		return nil, err
	}

	ids := make([]nostr.ID, len(deleted))
	for i, d := range deleted {
		if id, err := nostr.IDFromHex(d.Value.(*CachedEvent).Id); err == nil {
			ids[i] = id
		}
	}
	return ids, nil
}

func (internal *InternalDB) notCached(id nostr.ID) error {
	_, err := internal.DB.DeleteQuery(leafdb.ExactQuery("cached-id", id[0:8]))
	return err
}

func (internal *InternalDB) overwriteFollowListArchive(fla *FollowListArchive) error {
	_, err := internal.DB.AddOrReplace("follow-list-by-source", TypeFollowListArchive, fla)
	return err
}

func (internal *InternalDB) attachRelaysToEvent(eventId nostr.ID, relays ...string) (allRelays []string) {
	if _, err := internal.DB.Upsert("cached-id", eventId[0:8], TypeCachedEvent, func(t leafdb.DataType, value proto.Message) (proto.Message, error) {
		var ee *CachedEvent
		if value == nil {
			ee = &CachedEvent{
				Id:     eventId.Hex(),
				Relays: make([]string, 0, len(relays)),
				Expiry: time.Now().Add(time.Hour * 24 * 7).Unix(),
			}
		} else {
			ee = value.(*CachedEvent)
		}
		for _, r := range relays {
			r = nostr.NormalizeURL(r)
			if sdk.IsVirtualRelay(r) {
				continue
			}
			if !slices.Contains(ee.Relays, r) {
				ee.Relays = append(ee.Relays, r)
			}
		}
		allRelays = ee.Relays
		return ee, nil
	}); err != nil {
		log.Error().Err(err).Str("id", eventId.Hex()).Strs("relays", relays).Msg("failed to attach relays to event")
	}

	return allRelays
}

func (internal *InternalDB) getRelaysForEvent(eventId nostr.ID) []string {
	for value := range internal.DB.Query(leafdb.ExactQuery("cached-id", eventId[0:8])) {
		evtr := value.(*CachedEvent)
		return evtr.Relays
	}
	return nil
}

func (internal *InternalDB) getEventsInRelay(hostname string) iter.Seq[nostr.ID] {
	return func(yield func(nostr.ID) bool) {
		for value := range internal.DB.View(leafdb.ExactQuery("events-in-relay", []byte(hostname))) {
			if evtid, err := nostr.IDFromHex(value.(*ID).Id); err == nil {
				if !yield(evtid) {
					break
				}
			}
		}
	}
}

func (internal *InternalDB) banEvent(id nostr.ID, reason string) error {
	_, err := internal.DB.AddOrReplace("banned-event", TypeBannedEvent, &BannedEvent{
		Id:     id[:],
		Reason: reason,
	})

	return err
}

func (internal *InternalDB) unbanEvent(id nostr.ID) error {
	_, err := internal.DB.DeleteQuery(leafdb.ExactQuery("banned-event", id[0:8]))
	return err
}

func (internal *InternalDB) isBannedEvent(id nostr.ID) (bool, string) {
	for record := range internal.DB.Query(leafdb.ExactQuery("banned-event", id[0:8])) {
		return true, record.(*BannedEvent).Reason
	}

	return false, ""
}

func (internal *InternalDB) banPubkey(pk nostr.PubKey, reason string) error {
	_, err := internal.DB.AddOrReplace("banned-pubkey", TypeBannedPubkey, &BannedPubkey{
		Pk:     pk[:],
		Reason: reason,
	})

	return err
}

func (internal *InternalDB) unbanPubkey(pk nostr.PubKey) error {
	_, err := internal.DB.DeleteQuery(leafdb.ExactQuery("banned-pubkey", pk[0:8]))
	return err
}

func (internal *InternalDB) isBannedPubkey(pk nostr.PubKey) (bool, string) {
	for record := range internal.DB.Query(leafdb.ExactQuery("banned-pubkey", pk[0:8])) {
		return true, record.(*BannedPubkey).Reason
	}

	return false, ""
}
