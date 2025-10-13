package main

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"fiatjaf.com/nostr/nip31"
	"fiatjaf.com/nostr/nip52"
	"fiatjaf.com/nostr/nip53"
	"fiatjaf.com/nostr/nip92"
	"fiatjaf.com/nostr/nip94"
	"fiatjaf.com/nostr/sdk"
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
	Kind9802Metadata         Kind9802Metadata
}

func grabData(ctx context.Context, code string) (Data, error) {
	// code can be a nevent or naddr, in which case we try to fetch the associated event
	event, err := getEvent(ctx, code)
	if err != nil {
		return Data{}, fmt.Errorf("error fetching event: %w", err)
	}
	if event == nil {
		return Data{}, nil
	}

	ee := NewEnhancedEvent(ctx, *event)
	ee.relays = sys.GetEventRelays(event.ID)

	relaysForNip19 := make([]string, 0, 3)
	c := 0
	for _, relayUrl := range ee.relays {
		if sdk.IsVirtualRelay(relayUrl) {
			continue
		}
		relaysForNip19 = append(relaysForNip19, relayUrl)
		if c == 2 {
			break
		}
	}

	data := Data{
		event: ee,
	}

	data.nevent = nip19.EncodeNevent(event.ID, relaysForNip19, event.PubKey)
	data.neventNaked = nip19.EncodeNevent(event.ID, nil, event.PubKey)
	data.naddr = ""
	data.naddrNaked = ""
	data.createdAt = time.Unix(int64(event.CreatedAt), 0).Format("2006-01-02 15:04:05 MST")

	if event.Kind >= 30000 && event.Kind < 40000 {
		if dTag := event.Tags.Find("d"); dTag != nil {
			data.naddr = nip19.EncodeNaddr(event.PubKey, event.Kind, dTag[1], relaysForNip19)
			data.naddrNaked = nip19.EncodeNaddr(event.PubKey, event.Kind, dTag[1], nil)
		}
	}

	data.alt = nip31.GetAlt(*event)

	switch event.Kind {
	case 1, 7, 11, 1111:
		data.templateId = Note
		data.content = event.Content
	case 30023, 30024:
		data.templateId = LongForm
		data.content = event.Content
	case 20:
		data.templateId = Note
		data.content = event.Content
	case 6:
		data.templateId = Note
		if reposted := event.Tags.Find("e"); reposted != nil {
			id, _ := nostr.IDFromHex(reposted[1])
			originalNevent := nip19.EncodeNevent(id, []string{}, nostr.ZeroPK)
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
		data.Kind30818Metadata.Handle = event.Tags.GetD()
		data.Kind30818Metadata.Title = data.Kind30818Metadata.Handle
		if titleTag := event.Tags.Find("title"); titleTag != nil {
			data.Kind30818Metadata.Title = titleTag[1]
		}
		data.Kind30818Metadata.Summary = func() string {
			if tag := event.Tags.Find("summary"); tag != nil {
				value := tag[1]
				return value
			}
			return ""
		}()
		data.content = event.Content
	case 9802:
		data.templateId = Highlight
		data.content = event.Content
		if sourceEvent := event.Tags.Find("e"); sourceEvent != nil {
			data.Kind9802Metadata.SourceEvent = sourceEvent[1]
			data.Kind9802Metadata.SourceName = "#" + shortenString(sourceEvent[1], 8, 4)
		} else if sourceEvent := event.Tags.Find("a"); sourceEvent != nil {
			spl := strings.Split(sourceEvent[1], ":")
			kind, _ := strconv.Atoi(spl[0])
			var relayHints []string
			if len(sourceEvent) > 2 {
				relayHints = []string{sourceEvent[2]}
			}
			if pk, err := nostr.PubKeyFromHex(spl[1]); err == nil {
				naddr := nip19.EncodeNaddr(pk, nostr.Kind(kind), spl[2], relayHints)
				data.Kind9802Metadata.SourceEvent = naddr
			}
		} else if sourceUrl := event.Tags.Find("r"); sourceUrl != nil {
			data.Kind9802Metadata.SourceURL = sourceUrl[1]
			data.Kind9802Metadata.SourceName = sourceUrl[1]
		}

		if author := event.Tags.Find("p"); author != nil {
			ctx, cancel := context.WithTimeout(ctx, time.Second*3)
			defer cancel()
			if pk, err := nostr.PubKeyFromHex(author[1]); err == nil {
				data.Kind9802Metadata.Author = sys.FetchProfileMetadata(ctx, pk)
			}
		}

		if data.Kind9802Metadata.SourceEvent != "" {
			sourceEvent, _ := getEvent(ctx, data.Kind9802Metadata.SourceEvent)
			if sourceEvent == nil {
				data.Kind9802Metadata.SourceName = data.Kind9802Metadata.SourceEvent
			} else {
				if title := sourceEvent.Tags.Find("title"); title != nil {
					data.Kind9802Metadata.SourceName = title[1]
				} else {
					data.Kind9802Metadata.SourceName = "Note dated " + sourceEvent.CreatedAt.Time().Format("January 1, 2006 15:04")
				}

				// retrieve the author using the event, ignore the `p` tag in the highlight event
				ctx, cancel := context.WithTimeout(ctx, time.Second*3)
				defer cancel()
				data.Kind9802Metadata.Author = sys.FetchProfileMetadata(ctx, sourceEvent.PubKey)
			}
		}

		if context := event.Tags.Find("context"); context != nil {
			data.Kind9802Metadata.Context = context[1]

			escapedContext := html.EscapeString(context[1])
			escapedCitation := html.EscapeString(data.content)

			// Some clients mistakenly put the highlight in the context
			if escapedContext != escapedCitation {
				// Replace the citation with the marked version
				data.Kind9802Metadata.MarkedContext = strings.Replace(
					escapedContext,
					escapedCitation,
					fmt.Sprintf("<span class=\"bg-amber-100 dark:bg-amber-700\">%s</span>", escapedCitation),
					-1, // Replace all occurrences
				)
			}
		}

		if comment := event.Tags.Find("comment"); comment != nil {
			data.Kind9802Metadata.Comment = basicFormatting(comment[1], false, false, false)
		}

	default:
		data.templateId = Other
	}

	data.kindDescription = kindNames[event.Kind]
	if data.kindDescription == "" {
		data.kindDescription = fmt.Sprintf("Kind %d", event.Kind)
	}
	data.kindNIP = kindNIPs[event.Kind]

	image := event.Tags.Find("image")
	if event.Kind == 30023 && image != nil {
		data.cover = image[1]
	} else if event.Kind == 1063 {
		if data.kind1063Metadata.IsImage() {
			data.image = data.kind1063Metadata.URL
		} else if data.kind1063Metadata.IsVideo() {
			data.video = data.kind1063Metadata.URL
			data.videoType = strings.Split(data.kind1063Metadata.M, "/")[1]
		}
	} else if event.Kind == 20 {
		imeta := nip92.ParseTags(event.Tags)
		if len(imeta) > 0 {
			data.image = imeta[0].URL

			content := strings.Builder{}
			content.Grow(110*len(imeta) + len(data.content))
			for _, entry := range imeta {
				content.WriteString(entry.URL)
				content.WriteString(" ")
			}
			content.WriteString(data.content)
			data.content = content.String()
		}
		if tag := data.event.Tags.Find("title"); tag != nil {
			data.event.subject = tag[1]
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

	// check malicious
	hasURL := urlRegex.MatchString(data.event.Content)
	if isMaliciousBridged(data.event.author) ||
		(hasURL && hasProhibitedWordOrTag(data.event.Event)) ||
		(hasURL && hasExplicitMedia(ctx, data.event.Event)) {
		return data, fmt.Errorf("prohibited content: %w", err)
	}

	return data, nil
}
