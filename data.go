package main

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip10"
	"github.com/nbd-wtf/go-nostr/nip19"
	sdk "github.com/nbd-wtf/nostr-sdk"
)

type EnhancedEvent struct {
	event  *nostr.Event
	relays []string
}

func (ee EnhancedEvent) IsReply() bool {
	return nip10.GetImmediateReply(ee.event.Tags) != nil
}

func (ee EnhancedEvent) Preview() template.HTML {
	lines := strings.Split(html.EscapeString(ee.event.Content), "\n")
	var processedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		processedLine := shortenNostrURLs(line)
		processedLines = append(processedLines, processedLine)
	}

	return template.HTML(strings.Join(processedLines, "<br/>"))
}

func (ee EnhancedEvent) Npub() string {
	npub, _ := nip19.EncodePublicKey(ee.event.PubKey)
	return npub
}

func (ee EnhancedEvent) NpubShort() string {
	npub := ee.Npub()
	return npub[:8] + "…" + npub[len(npub)-4:]
}

func (ee EnhancedEvent) Nevent() string {
	nevent, _ := nip19.EncodeEvent(ee.event.ID, ee.relays, ee.event.PubKey)
	return nevent
}

func (ee EnhancedEvent) CreatedAtStr() string {
	return time.Unix(int64(ee.event.CreatedAt), 0).Format("2006-01-02 15:04:05")
}

func (ee EnhancedEvent) ModifiedAtStr() string {
	return time.Unix(int64(ee.event.CreatedAt), 0).Format("2006-01-02T15:04:05Z07:00")
}

type Data struct {
	templateId          TemplateID
	event               *nostr.Event
	relays              []string
	npub                string
	npubShort           string
	nprofile            string
	nevent              string
	neventNaked         string
	naddr               string
	naddrNaked          string
	createdAt           string
	modifiedAt          string
	parentLink          template.HTML
	metadata            *sdk.ProfileMetadata
	authorRelays        []string
	authorLong          string
	authorShort         string
	renderableLastNotes []EnhancedEvent
	kindDescription     string
	kindNIP             string
	video               string
	videoType           string
	image               string
	content             string
	alt                 string
	kind1063Metadata    *Kind1063Metadata
	kind30311Metadata   *Kind30311Metadata
}

type Kind1063Metadata struct {
	Magnet    string
	Dim       string
	Size      string
	Summary   string
	Image     string
	URL       string
	AES256GCM string
	M         string
	X         string
	I         string
	Blurhash  string
	Thumb     string
}

type Kind30311Metadata struct {
	Title    string
	Summary  string
	Image    string
	Status   string
	Host     sdk.ProfileMetadata
	HostNpub string
	Tags     []string
}

func (fm Kind1063Metadata) IsVideo() bool { return strings.Split(fm.M, "/")[0] == "video" }
func (fm Kind1063Metadata) IsImage() bool { return strings.Split(fm.M, "/")[0] == "image" }
func (fm Kind1063Metadata) DisplayImage() string {
	if fm.Image != "" {
		return fm.Image
	} else if fm.IsImage() {
		return fm.URL
	} else {
		return ""
	}
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
		if shouldUseRelayForNip19(relayUrl) {
			relaysForNip19 = append(relaysForNip19, relayUrl)
			if c == 2 {
				break
			}
		}
	}

	data := &Data{
		event:  event,
		relays: relays,
	}

	data.npub, _ = nip19.EncodePublicKey(event.PubKey)
	data.npubShort = data.npub[:8] + "…" + data.npub[len(data.npub)-4:]
	data.authorLong = data.npub       // hopefully will be replaced later
	data.authorShort = data.npubShort // hopefully will be replaced later
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

	if tag := event.Tags.GetFirst([]string{"alt", ""}); tag != nil {
		data.alt = (*tag)[1]
	}

	switch event.Kind {
	case 0:
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
		if parentNevent := getParentNevent(event); parentNevent != "" {
			data.parentLink = template.HTML(replaceNostrURLsWithTags(nostrNoteNeventMatcher, "nostr:"+parentNevent))
		}
	case 6:
		data.templateId = Note
		if reposted := event.Tags.GetFirst([]string{"e", ""}); reposted != nil {
			originalNevent, _ := nip19.EncodeEvent((*reposted)[1], []string{}, "")
			data.content = "Repost of nostr:" + originalNevent
		}
	case 1063:
		data.templateId = FileMetadata
		data.kind1063Metadata = &Kind1063Metadata{}

		if tag := event.Tags.GetFirst([]string{"url", ""}); tag != nil {
			data.kind1063Metadata.URL = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"m", ""}); tag != nil {
			data.kind1063Metadata.M = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"aes-256-gcm", ""}); tag != nil {
			data.kind1063Metadata.AES256GCM = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"x", ""}); tag != nil {
			data.kind1063Metadata.X = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"size", ""}); tag != nil {
			data.kind1063Metadata.Size = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"dim", ""}); tag != nil {
			data.kind1063Metadata.Dim = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"magnet", ""}); tag != nil {
			data.kind1063Metadata.Magnet = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"i", ""}); tag != nil {
			data.kind1063Metadata.I = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"blurhash", ""}); tag != nil {
			data.kind1063Metadata.Blurhash = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"thumb", ""}); tag != nil {
			data.kind1063Metadata.Thumb = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"image", ""}); tag != nil {
			data.kind1063Metadata.Image = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"summary", ""}); tag != nil {
			data.kind1063Metadata.Summary = (*tag)[1]
		}
	case 30311:
		data.templateId = LiveEvent
		data.kind30311Metadata = &Kind30311Metadata{}

		if tag := event.Tags.GetFirst([]string{"title", ""}); tag != nil {
			data.kind30311Metadata.Title = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"summary", ""}); tag != nil {
			data.kind30311Metadata.Summary = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"image", ""}); tag != nil {
			data.kind30311Metadata.Image = (*tag)[1]
		}
		if tag := event.Tags.GetFirst([]string{"status", ""}); tag != nil {
			data.kind30311Metadata.Status = (*tag)[1]
		}
		pTags := event.Tags.GetAll([]string{"p", ""})
		for _, p := range pTags {
			if p[3] == "host" {
				data.kind30311Metadata.Host = sdk.FetchProfileMetadata(ctx, pool, p[1], data.relays...)
				data.kind30311Metadata.HostNpub = data.kind30311Metadata.Host.Npub()
			}
		}
		tTags := event.Tags.GetAll([]string{"t", ""})
		for _, t := range tTags {
			data.kind30311Metadata.Tags = append(data.kind30311Metadata.Tags, t[1])
		}
	default:
		data.templateId = Other
	}

	if event.Kind == 0 {
		data.nprofile, _ = nip19.EncodeProfile(event.PubKey, limitAt(relays, 2))
		data.metadata, _ = sdk.ParseMetadata(event)
	} else {
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()
		author, relays, _ := getEvent(ctx, data.npub, relaysForNip19)
		if author != nil {
			data.metadata, _ = sdk.ParseMetadata(author)
			if data.metadata != nil {
				data.authorLong = fmt.Sprintf("%s (%s)", data.metadata.Name, data.npub)
				data.authorShort = fmt.Sprintf("%s (%s)", data.metadata.Name, data.npubShort)
			}
		}
		if len(relays) > 0 {
			data.nprofile, _ = nip19.EncodeProfile(event.PubKey, limitAt(relays, 2))
		}
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
