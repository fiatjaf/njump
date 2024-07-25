package main

import (
	"time"

	"github.com/nbd-wtf/go-nostr/nip52"
	"github.com/nbd-wtf/go-nostr/nip53"
	"github.com/nbd-wtf/go-nostr/nip94"
	sdk "github.com/nbd-wtf/nostr-sdk"
)

type Kind1063Metadata struct {
	nip94.FileMetadata
}

type Kind30311Metadata struct {
	nip53.LiveEvent
	Host *sdk.ProfileMetadata
}

func (le Kind30311Metadata) title() string {
	if le.Host != nil {
		return le.Title + " by " + le.Host.Name
	}
	return le.Title
}

type Kind31922Or31923Metadata struct {
	nip52.CalendarEvent
}

type Kind30818Metadata struct {
	Handle      string
	Title       string
	Summary     string
	PublishedAt time.Time
}
