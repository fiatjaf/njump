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
	"github.com/dgraph-io/ristretto"
)

var contentFilterCache, _ = ristretto.NewCache(&ristretto.Config[string, bool]{
	NumCounters: 1e6,
	MaxCost:     1 << 24,
	BufferItems: 64,
})

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

// getMediaURLs extracts image/video URLs from event content
func getMediaURLs(event *nostr.Event) []string {
	var urls []string
	for _, match := range imageExtensionMatcher.FindAllStringSubmatch(event.Content, -1) {
		if len(match) > 0 {
			urls = append(urls, match[0])
		}
	}
	for _, match := range videoExtensionMatcher.FindAllStringSubmatch(event.Content, -1) {
		if len(match) > 0 {
			urls = append(urls, match[0])
		}
	}
	return urls
}

// hasExplicitMedia checks if the event contains explicit media content
// by examining image/video URLs in the content and checking them against the media alert API
func hasExplicitMedia(ctx context.Context, event *nostr.Event) bool {
	for _, mediaURL := range getMediaURLs(event) {
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

// hasProhibitedContent checks event via media-alert and aedos APIs.
// Cache keyed by event ID. aedos only called when event has media URLs.
// If a check errors, its verdict is ignored.
// If either check says unsafe, event is blocked.
func hasProhibitedContent(ctx context.Context, event *nostr.Event) bool {
	if val, found := contentFilterCache.Get(event.ID.Hex()); found {
		return val
	}

	mediaURLs := getMediaURLs(event)
	if len(mediaURLs) == 0 {
		contentFilterCache.SetWithTTL(event.ID.Hex(), false, 1, 24*time.Hour)
		return false
	}

	mediaCh := make(chan bool, 1)
	aedosCh := make(chan bool, 1)

	go func() {
		mediaCh <- hasExplicitMedia(ctx, event)
	}()

	go func() {
		safe, err := checkAedosSafe(ctx, event.ID.Hex())
		if err != nil {
			log.Warn().Err(err).Str("event", event.ID.Hex()).Msg("aedos call failed")
		}
		aedosCh <- !safe
	}()

	result := <-mediaCh || <-aedosCh
	contentFilterCache.SetWithTTL(event.ID.Hex(), result, 1, 24*time.Hour)
	return result
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
	Type       string  `json:"type"`
	EventID    string  `json:"event_id"`
	Status     string  `json:"status"`
	Confidence float64 `json:"confidence"`
}

// checkAedosSafe checks event via aedos.nostr.com API.
// Returns true if safe, false if warn/prohibited, error on failure.
func checkAedosSafe(ctx context.Context, eventID string) (bool, error) {
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
		return true, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return true, fmt.Errorf("got unexpected response %d: %s", resp.StatusCode, string(msg))
	}

	var results []aedosResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return true, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(results) == 0 {
		return true, fmt.Errorf("empty response")
	}

	return results[0].Status == "safe" || results[0].Status == "unknown", nil
}
