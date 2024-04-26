package main

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/nbd-wtf/go-nostr/nip31"
	"github.com/nbd-wtf/go-nostr/nip52"
	"github.com/nbd-wtf/go-nostr/nip53"
	"github.com/nbd-wtf/go-nostr/nip94"
	sdk "github.com/nbd-wtf/nostr-sdk"
)

type Data struct {
	templateId               TemplateID
	event                    EnhancedEvent
	nprofile                 string
	nevent                   string
	neventNaked              string
	naddr                    string
	naddrNaked               string
	createdAt                string
	modifiedAt               string
	parentLink               template.HTML
	metadata                 Metadata
	authorRelays             []string
	authorLong               string
	renderableLastNotes      []EnhancedEvent
	kindDescription          string
	kindNIP                  string
	video                    string
	videoType                string
	image                    string
	content                  string
	alt                      string
	kind1063Metadata         *Kind1063Metadata
	kind30311Metadata        *Kind30311Metadata
	kind31922Or31923Metadata *Kind31922Or31923Metadata
}

func grabData(ctx context.Context, code string, isProfileSitemap bool) (*Data, error) {
	// code can be a nevent, nprofile, npub or nip05 identifier, in which case we try to fetch the associated event
	event, relays, err := getEvent(ctx, code, nil)
	if err != nil {
		log.Warn().Err(err).Str("code", code).Msg("failed to fetch event for code")
		return nil, fmt.Errorf("error fetching event: %w", err)
	}

	relaysForNip19 := make([]string, 0, 3)
	c := 0
	for _, relayUrl := range relays {
		if isntRealRelay(relayUrl) {
			continue
		}

		relaysForNip19 = append(relaysForNip19, relayUrl)
		if c == 2 {
			break
		}
	}

	data := &Data{
		event: EnhancedEvent{
			Event:  event,
			relays: relays,
		},
	}

	npub, _ := nip19.EncodePublicKey(event.PubKey)
	npubShort := npub[:8] + "…" + npub[len(npub)-4:]
	data.authorLong = npub // hopefully will be replaced later
	data.nevent, _ = nip19.EncodeEvent(event.ID, relaysForNip19, event.PubKey)
	data.neventNaked, _ = nip19.EncodeEvent(event.ID, nil, event.PubKey)
	data.naddr = ""
	data.naddrNaked = ""
	data.createdAt = time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02 15:04:05")
	data.modifiedAt = time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")
	data.authorRelays = []string{}

	if event.Kind >= 30000 && event.Kind < 40000 {
		if d := event.Tags.GetFirst([]string{"d", ""}); d != nil {
			data.naddr, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), relaysForNip19)
			data.naddrNaked, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), nil)
		}
	}

	data.alt = nip31.GetAlt(*event)

	switch event.Kind {
	case 0:
		data.templateId = Profile
		{
			rawAuthorRelays := []string{}
			ctx, cancel := context.WithTimeout(ctx, time.Second*4)
			rawAuthorRelays = relaysForPubkey(ctx, event.PubKey)
			cancel()
			for _, relay := range rawAuthorRelays {
				for _, excluded := range relayConfig.ExcludedRelays {
					if strings.Contains(relay, excluded) {
						continue
					}
				}
				if strings.Contains(relay, "/npub1") {
					continue // skip relays with personalyzed query like filter.nostr.wine
				}
				data.authorRelays = append(data.authorRelays, trimProtocol(relay))
			}
		}

		lastNotes := authorLastNotes(ctx, event.PubKey, data.authorRelays, isProfileSitemap)
		data.renderableLastNotes = make([]EnhancedEvent, len(lastNotes))
		for i, levt := range lastNotes {
			data.renderableLastNotes[i] = EnhancedEvent{levt, []string{}}
		}
		if err != nil {
			return nil, err
		}
	case 1, 7, 30023, 30024:
		data.templateId = Note
		data.content = event.Content
	case 6:
		data.templateId = Note
		if reposted := event.Tags.GetFirst([]string{"e", ""}); reposted != nil {
			originalNevent, _ := nip19.EncodeEvent((*reposted)[1], []string{}, "")
			data.content = "Repost of nostr:" + originalNevent
		}
	case 1063:
		data.templateId = FileMetadata
		data.kind1063Metadata = &Kind1063Metadata{nip94.ParseFileMetadata(*event)}
	case 30311:
		data.templateId = LiveEvent
		data.kind30311Metadata = &Kind30311Metadata{LiveEvent: nip53.ParseLiveEvent(*event)}
		host := data.kind30311Metadata.GetHost()
		if host != nil {
			hostProfile := sys.FetchProfileMetadata(ctx, host.PubKey)
			data.kind30311Metadata.Host = &hostProfile
		}
	case 1311:
		data.templateId = LiveEventMessage
		data.content = event.Content
	case 31922, 31923:
		data.templateId = CalendarEvent
		data.kind31922Or31923Metadata = &Kind31922Or31923Metadata{CalendarEvent: nip52.ParseCalendarEvent(*event)}
		data.content = event.Content
	default:
		data.templateId = Other
	}

	if event.Kind == 0 {
		data.nprofile, _ = nip19.EncodeProfile(event.PubKey, limitAt(relays, 2))
		spm, _ := sdk.ParseMetadata(event)
		data.metadata = Metadata{spm}
	} else {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()
		author, relays, _ := getEvent(ctx, npub, relaysForNip19)
		if author == nil {
			data.metadata = Metadata{sdk.ProfileMetadata{PubKey: event.PubKey}}
		} else {
			spm, _ := sdk.ParseMetadata(author)
			data.metadata = Metadata{spm}
			if data.metadata.Name != "" {
				data.authorLong = fmt.Sprintf("%s (%s)", data.metadata.Name, npubShort)
			}
		}
		data.nprofile, _ = nip19.EncodeProfile(event.PubKey, limitAt(relays, 2))
	}

	data.kindDescription = kindNames[event.Kind]
	if data.kindDescription == "" {
		data.kindDescription = fmt.Sprintf("Kind %d", event.Kind)
	}
	data.kindNIP = kindNIPs[event.Kind]

	if event.Kind == 1063 {
		if data.kind1063Metadata.IsImage() {
			data.image = data.kind1063Metadata.URL
		} else if data.kind1063Metadata.IsVideo() {
			data.video = data.kind1063Metadata.URL
			data.videoType = strings.Split(data.kind1063Metadata.M, "/")[1]
		}
	} else {
		urls := urlMatcher.FindAllString(event.Content, -1)
		for _, url := range urls {
			switch {
			case imageExtensionMatcher.MatchString(url):
				if data.image == "" {
					data.image = url
				}
			case videoExtensionMatcher.MatchString(url):
				if data.video == "" {
					data.video = url
					if strings.HasSuffix(data.video, "mp4") {
						data.videoType = "mp4"
					} else if strings.HasSuffix(data.video, "mov") {
						data.videoType = "mov"
					} else {
						data.videoType = "webm"
					}
				}
			}
		}
	}

	return data, nil
}
