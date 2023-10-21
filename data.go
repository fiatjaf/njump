package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Event struct {
	Npub         string
	NpubShort    string
	Nevent       string
	Content      string
	CreatedAt    string
	ModifiedAt   string
	ParentNevent string
}

type Data struct {
	templateId          TemplateID
	typ                 string
	event               *nostr.Event
	relays              []string
	npub                string
	npubShort           string
	nevent              string
	naddr               string
	createdAt           string
	modifiedAt          string
	parentNevent        string
	metadata            nostr.ProfileMetadata
	authorRelays        []string
	authorLong          string
	authorShort         string
	renderableLastNotes []*Event
	kindDescription     string
	kindNIP             string
	video               string
	videoType           string
	image               string
	content             string
}

func grabData(ctx context.Context, code string, isProfileSitemap bool) (*Data, error) {
	// code can be a nevent, nprofile, npub or nip05 identifier, in which case we try to fetch the associated event
	event, relays, err := getEvent(ctx, code)
	if err != nil {
		log.Warn().Err(err).Str("code", code).Msg("failed to fetch event for code")
		return nil, err
	}

	relaysForNip19 := make([]string, 0, 3)
	for i, relay := range relays {
		relaysForNip19 = append(relaysForNip19, relay)
		if i == 2 {
			break
		}
	}

	npub, _ := nip19.EncodePublicKey(event.PubKey)
	nevent, _ := nip19.EncodeEvent(event.ID, relaysForNip19, event.PubKey)
	naddr := ""
	createdAt := time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02 15:04:05")
	modifiedAt := time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")

	author := event
	var renderableLastNotes []*Event
	parentNevent := ""
	authorRelays := []string{}
	var content string
	var typ string
	var templateId TemplateID

	eventRelays := []string{}
	for _, relay := range relays {
		for _, excluded := range excludedRelays {
			if strings.Contains(relay, excluded) {
				continue
			}
		}
		if strings.Contains(relay, "/npub1") {
			continue // skip relays with personalyzed query like filter.nostr.wine
		}
		eventRelays = append(eventRelays, trimProtocol(relay))
	}

	switch event.Kind {
	case 0:
		key := ""
		eventsToFetch := 100
		if isProfileSitemap {
			typ = "profile_sitemap"
			key = "lns:" + event.PubKey
			eventsToFetch = 50000
		} else {
			typ = "profile"
			key = "ln:" + event.PubKey
		}

		{
			rawAuthorRelays := []string{}
			ctx, cancel := context.WithTimeout(ctx, time.Second*4)
			rawAuthorRelays = relaysForPubkey(ctx, event.PubKey)
			cancel()
			for _, relay := range rawAuthorRelays {
				for _, excluded := range excludedRelays {
					if strings.Contains(relay, excluded) {
						continue
					}
				}
				if strings.Contains(relay, "/npub1") {
					continue // skip relays with personalyzed query like filter.nostr.wine
				}
				authorRelays = append(authorRelays, trimProtocol(relay))
			}
		}

		var lastNotes []*nostr.Event

		if ok := cache.GetJSON(key, &lastNotes); !ok {
			ctx, cancel := context.WithTimeout(ctx, time.Second*4)
			lastNotes = getLastNotes(ctx, code, eventsToFetch)
			cancel()
			if len(lastNotes) > 0 {
				cache.SetJSONWithTTL(key, lastNotes, time.Hour*24)
			}
		}

		renderableLastNotes = make([]*Event, len(lastNotes))
		for i, n := range lastNotes {
			nevent, _ := nip19.EncodeEvent(n.ID, []string{}, n.PubKey)
			renderableLastNotes[i] = &Event{
				Nevent:       nevent,
				Content:      n.Content,
				CreatedAt:    time.Unix(int64(n.CreatedAt), 0).Format("2006-01-02 15:04:05"),
				ModifiedAt:   time.Unix(int64(n.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00"),
				ParentNevent: getParentNevent(n),
			}
		}
		if err != nil {
			return nil, err
		}
	case 1, 7, 30023, 30024:
		templateId = Note
		content = event.Content
		parentNevent = getParentNevent(event)
	case 6:
		templateId = Note
		if reposted := event.Tags.GetFirst([]string{"e", ""}); reposted != nil {
			original_nevent, _ := nip19.EncodeEvent((*reposted)[1], []string{}, "")
			content = "Repost of nostr:" + original_nevent
		}
	default:
		if event.Kind >= 30000 && event.Kind < 40000 {
			templateId = Other
			if d := event.Tags.GetFirst([]string{"d", ""}); d != nil {
				naddr, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), relaysForNip19)
			}
		}
	}

	if event.Kind != 0 {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		author, _, _ = getEvent(ctx, npub)
		cancel()
	}

	kindDescription := kindNames[event.Kind]
	if kindDescription == "" {
		kindDescription = fmt.Sprintf("Kind %d", event.Kind)
	}
	kindNIP := kindNIPs[event.Kind]

	urls := urlMatcher.FindAllString(event.Content, -1)
	var image string
	var video string
	var videoType string
	for _, url := range urls {
		switch {
		case imageExtensionMatcher.MatchString(url):
			if image == "" {
				image = url
			}
		case videoExtensionMatcher.MatchString(url):
			if video == "" {
				video = url
				if strings.HasSuffix(video, "mp4") {
					videoType = "mp4"
				} else if strings.HasSuffix(video, "mov") {
					videoType = "mov"
				} else {
					videoType = "webm"
				}
			}
		}
	}

	npubShort := npub[:8] + "…" + npub[len(npub)-4:]
	authorLong := npub
	authorShort := npubShort

	var metadata nostr.ProfileMetadata
	if author != nil {
		if err := json.Unmarshal([]byte(author.Content), &metadata); err == nil {
			authorLong = fmt.Sprintf("%s (%s)", metadata.Name, npub)
			authorShort = fmt.Sprintf("%s (%s)", metadata.Name, npubShort)
		}
	}

	return &Data{
		templateId:          templateId,
		typ:                 typ,
		event:               event,
		relays:              eventRelays,
		npub:                npub,
		npubShort:           npubShort,
		nevent:              nevent,
		naddr:               naddr,
		authorRelays:        authorRelays,
		createdAt:           createdAt,
		modifiedAt:          modifiedAt,
		parentNevent:        parentNevent,
		metadata:            metadata,
		authorLong:          authorLong,
		authorShort:         authorShort,
		renderableLastNotes: renderableLastNotes,
		kindNIP:             kindNIP,
		kindDescription:     kindDescription,
		video:               video,
		videoType:           videoType,
		image:               image,
		content:             content,
	}, nil
}
