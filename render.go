package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

//go:embed event.html
var eventHTML string

var tmpl = template.Must(template.New("event").Parse(eventHTML))

func render(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	code := r.URL.Path[1:]
	if strings.HasPrefix(code, "e/") {
		code, _ = nip19.EncodeNote(code[2:])
	} else if strings.HasPrefix(code, "p/") {
		code, _ = nip19.EncodePublicKey(code[2:])
	}
	if code == "" {
		fmt.Fprintf(w, "call /<nip19 code>")
		return
	}

	hostname := r.Header.Get("X-Forwarded-Host")
	style := getPreviewStyle(r)

	event, err := getEvent(r.Context(), code)
	if err != nil {
		http.Error(w, "error fetching event: "+err.Error(), 404)
		return
	}

	typ := "profile"

	npub, _ := nip19.EncodePublicKey(event.PubKey)
	nevent, _ := nip19.EncodeEvent(event.ID, []string{}, event.PubKey)
	naddr := ""

	author := event
	if event.Kind != 0 {
		typ = "event"
		author, _ = getEvent(r.Context(), npub)

		if event.Kind >= 30000 && event.Kind < 40000 {
			typ = "address"
			if d := event.Tags.GetFirst([]string{"d", ""}); d != nil {
				naddr, _ = nip19.EncodeEntity(event.PubKey, event.Kind, d.Value(), []string{})
			}
		}
	}

	imageMatch := regexp.MustCompile(`https:\/\/[^ ]*\.(gif|jpe?g|png|webp)`).FindStringSubmatch(event.Content)
	var image string
	if len(imageMatch) > 0 {
		image = imageMatch[0]
	}
	fmt.Println("IMAGE", image)

	videoMatch := regexp.MustCompile(`https:\/\/[^ ]*\.(mp4|webm)`).FindStringSubmatch(event.Content)
	var video string
	if len(videoMatch) > 0 {
		video = videoMatch[0]
	}

	var videoType string
	if video != "" {
		if strings.HasSuffix(video, "mp4") {
			videoType = "mp4"
		} else {
			videoType = "webm"
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

	var subject string
	for _, tag := range event.Tags {
		if tag[0] == "subject" || tag[0] == "title" {
			subject = tag[1]
			break
		}
	}

	useTextImage := (event.Kind == 1 || event.Kind == 30023) && image == "" && video == "" && len(event.Content) > 120
	if style == "slack" || style == "discord" {
		useTextImage = false
	}

	title := ""
	twitterTitle := title
	if event.Kind == 0 && metadata.Name != "" {
		title = metadata.Name
	} else {
		if event.Kind >= 30000 && event.Kind < 40000 {
			tValue := "~"
			for _, tag := range event.Tags {
				if tag[0] == "t" {
					tValue = tag[1]
					break
				}
			}
			title = fmt.Sprintf("%s: %s", kindNames[event.Kind], tValue)
		} else if kindName, ok := kindNames[event.Kind]; ok {
			title = kindName
		} else {
			title = fmt.Sprintf("kind:%d event", event.Kind)
		}
		if subject != "" {
			title += " (" + subject + ")"
		}
		twitterTitle += " by " + authorShort
		date := event.CreatedAt.Time().UTC().Format("2006-01-02 15:04")
		title += " at " + date
		twitterTitle += " at " + date
	}

	seenOnRelays := ""
	//  event.seenOn && event.seenOn.length > 0
	//    ? `seen on [ ${event.seenOn.join(' ')} ]`
	//    : ''

	textImageURL := ""
	description := ""
	if useTextImage {
		textImageURL = fmt.Sprintf("https://%s/image/%s", hostname, code)
		if subject != "" {
			description = fmt.Sprintf("%s -- %s", subject, seenOnRelays)
		} else {
			description = seenOnRelays
		}
	} else {
		description = prettyJsonOrRaw(event.Content)
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")

	params := map[string]any{
		"clients":      generateClientList(code, event),
		"type":         typ,
		"title":        title,
		"twitterTitle": twitterTitle,
		"npub":         npub,
		"nevent":       nevent,
		"naddr":        naddr,
		"metadata":     metadata,
		"authorLong":   authorLong,
		"description":  description,
		"textImageURL": textImageURL,
		"videoType":    videoType,
		"image":        image,
		"video":        video,
		"eventJSON":    string(eventJSON),
	}
	if err := tmpl.ExecuteTemplate(w, "event", params); err != nil {
		http.Error(w, "error rendering: "+err.Error(), 500)
		return
	}

	return
}
