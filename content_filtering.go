package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/sdk"
)

func isMaliciousBridged(pm sdk.ProfileMetadata) bool {
	return strings.Contains(pm.NIP05, "rape.pet") || strings.Contains(pm.NIP05, "rape-pet")
}

func hasProhibitedWordOrTag(event *nostr.Event) bool {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "t" && slices.Contains(pornTags, strings.ToLower(tag[1])) {
			return true
		}
	}

	return pornWordsRe.MatchString(event.Content)
}

// hasExplicitMedia checks if the event contains explicit media content
// by examining image/video URLs in the content and checking them against the media alert API
func hasExplicitMedia(ctx context.Context, event *nostr.Event) bool {
	// extract image and video URLs from content
	var mediaURLs []string

	// find image URLs
	imgMatches := imageExtensionMatcher.FindAllStringSubmatch(event.Content, -1)
	for _, match := range imgMatches {
		if len(match) > 0 {
			mediaURLs = append(mediaURLs, match[0])
		}
	}

	// find video URLs
	vidMatches := videoExtensionMatcher.FindAllStringSubmatch(event.Content, -1)
	for _, match := range vidMatches {
		if len(match) > 0 {
			mediaURLs = append(mediaURLs, match[0])
		}
	}

	// check each URL for explicit content
	for _, mediaURL := range mediaURLs {
		isExplicit, err := isExplicitContent(ctx, mediaURL)
		if err != nil {
			log.Warn().Err(err).Str("url", mediaURL).Msg("failed to check media content")
			continue
		}

		if isExplicit {
			return true
		}
	}

	return false
}

// hasProhibitedContent checks event via both media-alert and aedos APIs in parallel.
// If a check errors, its verdict is ignored.
// If either check succeeds and says unsafe, event is blocked.
func hasProhibitedContent(ctx context.Context, event *nostr.Event) bool {
	mediaCh := make(chan bool, 1)
	aedosCh := make(chan bool, 1)

	go func() {
		mediaCh <- hasExplicitMedia(ctx, event)
	}()

	go func() {
		safe, err := checkAedos(ctx, event.ID.Hex())
		if err != nil {
			aedosCh <- false
			return
		}
		aedosCh <- !safe
	}()

	return <-mediaCh || <-aedosCh
}

// list copied from https://jsr.io/@gleasonator/policy/0.9.8/policies/AntiPornPolicy.ts
var pornTags = []string{
	"adult",
	"ass",
	"assworship",
	"boobs",
	"boobies",
	"butt",
	"cock",
	"dick",
	"dickpic",
	"explosionloli",
	"femboi",
	"femboy",
	"fetish",
	"fuck",
	"freeporn",
	"girls",
	"loli",
	"milf",
	"nude",
	"nudity",
	"nsfw",
	"pantsu",
	"pussy",
	"porn",
	"porno",
	"porntube",
	"pornvideo",
	"sex",
	"sexpervertsyndicate",
	"sexporn",
	"sexy",
	"slut",
	"teen",
	"tits",
	"teenporn",
	"teens",
	"transnsfw",
	"xxx",
	"うちの子を置くとみんながうちの子に対する印象をリアクションしてくれるタグ",
}

var pornWordsRe = func() *regexp.Regexp {
	// list copied from https://jsr.io/@gleasonator/policy/0.2.0/data/pornwords.json
	pornWords := []string{
		"loli",
		"nsfw",
		"teen porn",
	}
	concat := strings.Join(pornWords, "|")
	regex := fmt.Sprintf(`\b(%s)\b`, concat)
	return regexp.MustCompile(regex)
}()

type aedosRequest struct {
	Events []aedosRequestEvent `json:"events"`
}

type aedosRequestEvent struct {
	EventID string `json:"event_id"`
}

type aedosResponse struct {
	Type       string   `json:"type"`
	EventID    string   `json:"event_id"`
	Status     string   `json:"status"`
	Cache      bool     `json:"cache"`
	Labels     []string `json:"labels"`
	Confidence float64  `json:"confidence"`
}

// checkAedos checks event via aedos.nostr.com API.
// Returns true if safe, false if warn/prohibited, error on failure.
func checkAedos(ctx context.Context, eventID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	body := aedosRequest{
		Events: []aedosRequestEvent{{EventID: eventID}},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://aedos.nostr.com/v1/check_batch", bytes.NewReader(bodyBytes))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("got unexpected response %d: %s", resp.StatusCode, string(msg))
	}

	var results []aedosResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return false, fmt.Errorf("empty response")
	}

	return results[0].Status == "safe", nil
}
