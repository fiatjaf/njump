package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

func hasProhibitedWordOrTag(event *nostr.Event) bool {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "t" && slices.Contains(pornTags, strings.ToLower(tag[1])) {
			return true
		}
	}

	return pornWordsRe.MatchString(event.Content)
}

// list copied from https://jsr.io/@gleasonator/policy/0.2.0/data/porntags.json
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
	"lewd",
	"loli",
	"milf",
	"naked",
	"nude",
	"nudes",
	"nudeart",
	"nudity",
	"nsfw",
	"pantsu",
	"pussy",
	"porn",
	"porngif",
	"porno",
	"pornstar",
	"porntube",
	"pornvideo",
	"sex",
	"sexpervertsyndicate",
	"sexporn",
	"sexworker",
	"sexy",
	"slut",
	"teen",
	"tits",
	"teenporn",
	"teens",
	"transnsfw",
	"xxx",
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
