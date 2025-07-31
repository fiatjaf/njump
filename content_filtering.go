package main

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

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
