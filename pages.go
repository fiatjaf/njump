//go:generate tmpl bind ./...

package main

import (
	_ "embed"
	"html/template"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip11"
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
	Metadata        Metadata
	Nevent          string
	Nprofile        string
	SeenOn          []string
	Kind            int
	KindNIP         string
	KindDescription string

	// kind-specific stuff
	FileMetadata *Kind1063Metadata
	LiveEvent    *Kind30311Metadata
}

type HeadParams struct {
	IsProfile   bool
	NaddrNaked  string
	NeventNaked string
	Oembed      string
}

type TelegramInstantViewParams struct {
	Video       string
	VideoType   string
	Image       string
	Summary     template.HTML
	Content     template.HTML
	Description string
	Subject     string
	Metadata    Metadata
	AuthorLong  string
	CreatedAt   string
	ParentLink  template.HTML
}

type HomePageParams struct {
	HeadParams

	Npubs     []string
	LastNotes []string
}

type ArchivePageParams struct {
	HeadParams

	Title         string
	PathPrefix    string
	Data          []string
	ModifiedAt    string
	PaginationUrl string
	NextPage      int
	PrevPage      int
}

type OtherPageParams struct {
	HeadParams
	DetailsParams

	Kind            int
	KindDescription string
	Alt             string
}

type NotePageParams struct {
	OpenGraphParams
	HeadParams
	DetailsParams

	Content          template.HTML
	CreatedAt        string
	Metadata         Metadata
	ParentLink       template.HTML
	SeenOn           []string
	Subject          string
	TitleizedContent string
	Clients          []ClientReference
}

type EmbeddedNoteParams struct {
	Content   template.HTML
	CreatedAt string
	Metadata  Metadata
	SeenOn    []string
	Subject   string
	Url       string
}

type ProfilePageParams struct {
	HeadParams
	DetailsParams

	AuthorRelays               []string
	Content                    string
	CreatedAt                  string
	Domain                     string
	LastNotes                  []EnhancedEvent
	Metadata                   Metadata
	NormalizedAuthorWebsiteURL string
	RenderedAuthorAboutText    template.HTML
	Nevent                     string
	Nprofile                   string
	IsReply                    string
	Proxy                      string
	Title                      string
	Clients                    []ClientReference
}

type EmbeddedProfileParams struct {
	AuthorRelays               []string
	Content                    string
	CreatedAt                  string
	Domain                     string
	Metadata                   Metadata
	NormalizedAuthorWebsiteURL string
	RenderedAuthorAboutText    template.HTML
	Nevent                     string
	Nprofile                   string
	Proxy                      string
	Title                      string
}

type FileMetadataPageParams struct {
	OpenGraphParams
	HeadParams
	DetailsParams

	Content          template.HTML
	CreatedAt        string
	Metadata         Metadata
	ParentLink       template.HTML
	SeenOn           []string
	Style            Style
	Subject          string
	TitleizedContent string
	Alt              string

	FileMetadata Kind1063Metadata
	IsImage      bool
	IsVideo      bool

	Clients []ClientReference
}

type LiveEventPageParams struct {
	OpenGraphParams
	HeadParams
	DetailsParams

	Content          template.HTML
	CreatedAt        string
	Metadata         Metadata
	ParentLink       template.HTML
	SeenOn           []string
	Style            Style
	Subject          string
	TitleizedContent string
	Alt              string

	LiveEvent Kind30311Metadata

	Clients []ClientReference
}

type LiveEventMessagePageParams struct {
	OpenGraphParams
	HeadParams
	DetailsParams

	Content          template.HTML
	CreatedAt        string
	Metadata         Metadata
	ParentLink       template.HTML
	SeenOn           []string
	Style            Style
	Subject          string
	TitleizedContent string
	Alt              string

	LiveEventMessage Kind1311Metadata

	Clients []ClientReference
}

type RelayPageParams struct {
	HeadParams

	Info       *nip11.RelayInformationDocument
	Hostname   string
	Proxy      string
	LastNotes  []EnhancedEvent
	ModifiedAt string
	Clients    []ClientReference
}

type ErrorPageParams struct {
	HeadParams
	Errors  string
	Message string
}

func (e *ErrorPageParams) MessageHTML() template.HTML {
	if e.Message != "" {
		return template.HTML(e.Message)
	}

	switch {
	case strings.Contains(e.Errors, "invalid checksum"):
		return "It looks like you entered an invalid event code.<br> Check if you copied it fully, a good idea is compare the first and the last characters."
	case strings.Contains(e.Errors, "couldn't find this"):
		return "Can't find the event in the relays. Try getting an `nevent1` code with relay hints."
	case strings.Contains(e.Errors, "invalid bech32 string length"),
		strings.Contains(e.Errors, "invalid separator"),
		strings.Contains(e.Errors, "not part of charset"):
		return "You have typed a wrong event code, we need a URL path that starts with /npub1, /nprofile1, /nevent1, /naddr1, or something like /name@domain.com (or maybe just /domain.com) or an event id as hex (like /aef8b32af...)"
	default:
		return "I can't give any suggestions to solve the problem.<br> Please tag <a href='/dtonon.com'>daniele</a> and <a href='/fiatjaf.com'>fiatjaf</a> and complain!"
	}
}
