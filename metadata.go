package main

import (
	"github.com/nbd-wtf/go-nostr/sdk"
)

// check if the user has requested to not be indexed
// https://gitlab.com/soapbox-pub/ditto/-/merge_requests/596
func indexOptOut(p sdk.ProfileMetadata) bool {
	if p.Event == nil {
		return false
	}

	for _, tag := range p.Event.Tags {
		if tag[0] == "l" && tag[1] == "!no-unauthenticated" && tag[2] == "com.atproto.label.defs#selfLabel" {
			return true
		}
	}

	return false
}