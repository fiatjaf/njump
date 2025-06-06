//go:generate npm install tailwindcss
//go:generate npx tailwind -i node_modules/tailwindcss/tailwind.css -o tailwind-bundle.min.css --minify
//go:generate go run -mod=mod github.com/a-h/templ/cmd/templ@latest generate

package main

import (
	_ "embed"
	"html/template"

	"github.com/a-h/templ"
	"github.com/nbd-wtf/go-nostr/sdk"
)

type TemplateID int

const (
	Note TemplateID = iota
	Profile
	LongForm
	TelegramInstantView
	FileMetadata
	LiveEvent
	LiveEventMessage
	CalendarEvent
	WikiEvent
	Highlight
	Other
)

type OpenGraphParams struct {
	SingleTitle string
	// x (we will always render just the singletitle if we have that)
	Superscript string
	Subscript   string

	BigImage string
	// x (we will always render just the bigimage if we have that)
	Video        string
	VideoType    string
	Image        string
	ProxiedImage string

	// this is the main text we should always have
	Text string
}

type DetailsParams struct {
	HideDetails     bool
	CreatedAt       string
	EventJSON       template.HTML
	Metadata        sdk.ProfileMetadata
	Nevent          string
	Nprofile        string
	SeenOn          []string
	Kind            int
	KindNIP         string
	KindDescription string
	Extra           templ.Component
}

type HeadParams struct {
	IsHome      bool
	IsAbout     bool
	IsProfile   bool
	Lang        string
	NaddrNaked  string
	NeventNaked string
	Oembed      string
}

type BaseEventPageParams struct {
	Event EnhancedEvent
	Style Style
	Alt   string
}
