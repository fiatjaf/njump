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
	"github.com/nbd-wtf/go-nostr/sdk"
)

type Data struct {
	templateId               TemplateID
	event                    EnhancedEvent
	nevent                   string
	neventNaked              string
	naddr                    string
	naddrNaked               string
	createdAt                string
	parentLink               template.HTML
	kindDescription          string
	kindNIP                  string
	video                    string
	videoType                string
	image                    string
	cover                    string
	content                  string
	alt                      string
	kind1063Metadata         *Kind1063Metadata
	kind30311Metadata        *Kind30311Metadata
	kind31922Or31923Metadata *Kind31922Or31923Metadata
	Kind30818Metadata        Kind30818Metadata
}

func grabData(ctx context.Context, code string) (Data, error) {
	// code can be a nevent or naddr, in which case we try to fetch the associated event
	event, relays, err := getEvent(ctx, code)
	if err != nil {
		return Data{}, fmt.Errorf("error fetching event: %w", err)
	}

	relaysForNip19 := make([]string, 0, 3)
	c := 0
	for _, relayUrl := range relays {
		if sdk.IsVirtualRelay(relayUrl) {
			continue
		}
		relaysForNip19 = append(relaysForNip19, relayUrl)
		if c == 2 {
			break
		}
	}

	ee := NewEnhancedEvent(ctx, event)
	ee.relays = relays

	data := Data{
		event: ee,
	}

	data.nevent, _ = nip19.EncodeEvent(event.ID, relaysForNip19, event.PubKey)
	data.neventNaked, _ = nip19.EncodeEvent(event.ID, nil, event.PubKey)
	data.naddr = ""
	data.naddrNaked = ""
	data.createdAt = time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02 15:04:05")

	if event.Kind >= 30000 && event.Kind < 40000 {
		if d := event.Tags.GetFirst([]string{"d", ""}); d != nil {
			data.naddr, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), relaysForNip19)
			data.naddrNaked, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), nil)
		}
	}

	data.alt = nip31.GetAlt(*event)

	switch event.Kind {
	case 1, 7:
		data.templateId = Note
		data.content = event.Content
	case 30023, 30024:
		data.templateId = LongForm
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
	case 30818:
		data.templateId = WikiEvent
		data.Kind30818Metadata.Handle = event.Tags.GetFirst([]string{"d"}).Value()
		data.Kind30818Metadata.Title = event.Tags.GetFirst([]string{"title"}).Value()
		data.Kind30818Metadata.Summary = func() string {
			if tag := event.Tags.GetFirst([]string{"summary"}); tag != nil {
				value := tag.Value()
				return value
			}
			return ""
		}()
		data.content = event.Content
	default:
		data.templateId = Other
	}

	data.kindDescription = kindNames[event.Kind]
	if data.kindDescription == "" {
		data.kindDescription = fmt.Sprintf("Kind %d", event.Kind)
	}
	data.kindNIP = kindNIPs[event.Kind]

	image := event.Tags.GetFirst([]string{"image", ""})
	if event.Kind == 30023 && image != nil {
		data.cover = (*image)[1]
	}

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
