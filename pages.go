//go:generate tmpl bind ./...

package main

import (
	_ "embed"
	"html/template"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/tylermmorton/tmpl"
)

type TemplateID int

const (
	Note TemplateID = iota
	LongForm
	TelegramInstantView
	FileMetadata
	Other
)

var (
	//go:embed templates/head_common.html
	tmplHeadCommon     string
	HeadCommonTemplate = tmpl.MustCompile(&HeadCommonPartial{})
)

//tmpl:bind head_common.html
type HeadCommonPartial struct {
	IsProfile          bool
	TailwindDebugStuff template.HTML
	NaddrNaked         string
	NeventNaked        string
}

func (*HeadCommonPartial) TemplateText() string { return tmplHeadCommon }

var (
	//go:embed templates/top.html
	tmplTop     string
	TopTemplate = tmpl.MustCompile(&TopPartial{})
)

//tmpl:bind top.html
type TopPartial struct{}

func (*TopPartial) TemplateText() string { return tmplTop }

var (
	//go:embed templates/details.html
	tmplDetails     string
	DetailsTemplate = tmpl.MustCompile(&DetailsPartial{})
)

//tmpl:bind details.html
type DetailsPartial struct {
	HideDetails     bool
	CreatedAt       string
	EventJSON       template.HTML
	Nevent          string
	Nprofile        string
	Npub            string
	SeenOn          []string
	Kind            int
	KindNIP         string
	KindDescription string
	Magnet          string
	Dim             string
	Size            string
	Summary         string
}

func (*DetailsPartial) TemplateText() string { return tmplDetails }

var (
	//go:embed templates/clients.html
	tmplClients     string
	ClientsTemplate = tmpl.MustCompile(&ClientsPartial{})
)

//tmpl:bind clients.html
type ClientsPartial struct {
	Clients []ClientReference
}

func (*ClientsPartial) TemplateText() string { return tmplClients }

var (
	//go:embed templates/footer.html
	tmplFooter     string
	FooterTemplate = tmpl.MustCompile(&FooterPartial{})
)

//tmpl:bind footer.html
type FooterPartial struct{}

func (*FooterPartial) TemplateText() string { return tmplFooter }

var (
	//go:embed templates/telegram_instant_view.html
	tmplTelegramInstantView     string
	TelegramInstantViewTemplate = tmpl.MustCompile(&TelegramInstantViewPage{})
)

type TelegramInstantViewPage struct {
	Video       string
	VideoType   string
	Image       string
	Summary     template.HTML
	Content     template.HTML
	Description string
	Subject     string
	Metadata    nostr.ProfileMetadata
	AuthorLong  string
	CreatedAt   string
}

func (*TelegramInstantViewPage) TemplateText() string { return tmplTelegramInstantView }

var (
	//go:embed templates/homepage.html
	tmplHomePage     string
	HomePageTemplate = tmpl.MustCompile(&HomePage{})
)

type HomePage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	FooterPartial     `tmpl:"footer"`

	Host      string
	Npubs     []string
	LastNotes []string
}

func (*HomePage) TemplateText() string { return tmplHomePage }

var (
	//go:embed templates/archive.html
	tmplArchive     string
	ArchiveTemplate = tmpl.MustCompile(&ArchivePage{})
)

type ArchivePage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	FooterPartial     `tmpl:"footer"`

	Title         string
	PathPrefix    string
	Data          []string
	ModifiedAt    string
	PaginationUrl string
	NextPage      int
	PrevPage      int
}

func (*ArchivePage) TemplateText() string { return tmplArchive }

var (
	//go:embed templates/other.html
	tmplOther     string
	OtherTemplate = tmpl.MustCompile(&OtherPage{})
)

type OtherPage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	DetailsPartial    `tmpl:"details"`
	FooterPartial     `tmpl:"footer"`

	Naddr           string
	Kind            int
	KindDescription string
}

func (*OtherPage) TemplateText() string { return tmplOther }

var (
	//go:embed templates/note.html
	tmplNote     string
	NoteTemplate = tmpl.MustCompile(&NotePage{})
)

type NotePage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	DetailsPartial    `tmpl:"details"`
	ClientsPartial    `tmpl:"clients"`
	FooterPartial     `tmpl:"footer"`

	AuthorLong       string
	Content          template.HTML
	CreatedAt        string
	Description      string
	Image            string
	Metadata         nostr.ProfileMetadata
	Npub             string
	NpubShort        string
	Oembed           string
	ParentLink       template.HTML
	Proxy            string
	SeenOn           []string
	Style            string
	Subject          string
	TextImageURL     string
	Title            string
	TitleizedContent string
	TwitterTitle     string
	Video            string
	VideoType        string
}

func (*NotePage) TemplateText() string { return tmplNote }

var (
	//go:embed templates/profile.html
	tmplProfile     string
	ProfileTemplate = tmpl.MustCompile(&ProfilePage{})
)

type ProfilePage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	DetailsPartial    `tmpl:"details"`
	ClientsPartial    `tmpl:"clients"`
	FooterPartial     `tmpl:"footer"`

	AuthorRelays               []string
	Content                    string
	CreatedAt                  string
	Domain                     string
	LastNotes                  []EnhancedEvent
	Metadata                   nostr.ProfileMetadata
	NormalizedAuthorWebsiteURL string
	RenderedAuthorAboutText    template.HTML
	Nevent                     string
	Npub                       string
	Nprofile                   string
	IsReply                    string
	Proxy                      string
	Title                      string
}

func (*ProfilePage) TemplateText() string { return tmplProfile }

var (
	//go:embed templates/file_metadata.html
	tmplFileMetadata     string
	FileMetadataTemplate = tmpl.MustCompile(&FileMetadataPage{})
)

type FileMetadataPage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	DetailsPartial    `tmpl:"details"`
	ClientsPartial    `tmpl:"clients"`
	FooterPartial     `tmpl:"footer"`

	AuthorLong       string
	Content          template.HTML
	CreatedAt        string
	Description      string
	Metadata         nostr.ProfileMetadata
	Npub             string
	NpubShort        string
	Oembed           string
	ParentLink       template.HTML
	Proxy            string
	SeenOn           []string
	Style            string
	Subject          string
	TextImageURL     string
	Title            string
	TitleizedContent string
	TwitterTitle     string
	Video            string
	VideoType        string

	// Specific Metadata
	Url       string
	M         string
	Aes256Gcm string
	X         string
	Size      string
	Dim       string
	Magnet    string
	I         string
	Blurhash  string
	Thumb     string
	Image     string
	Summary   string
	Alt       string

	MType string // The first part of the mime type M
}

func (*FileMetadataPage) TemplateText() string { return tmplFileMetadata }

var (
	//go:embed templates/relay.html
	tmplRelay     string
	RelayTemplate = tmpl.MustCompile(&RelayPage{})
)

type RelayPage struct {
	HeadCommonPartial `tmpl:"head_common"`
	TopPartial        `tmpl:"top"`
	ClientsPartial    `tmpl:"clients"`
	FooterPartial     `tmpl:"footer"`

	Info       *nip11.RelayInformationDocument
	Hostname   string
	Proxy      string
	LastNotes  []EnhancedEvent
	ModifiedAt string
}

func (*RelayPage) TemplateText() string { return tmplRelay }

var (
	//go:embed templates/sitemap.xml
	tmplSitemap     string
	SitemapTemplate = tmpl.MustCompile(&SitemapPage{})
)

type SitemapPage struct {
	Host       string
	ModifiedAt string

	// for the profile sitemap
	Npub string

	// for the relay sitemap
	RelayHostname string

	// for the profile and relay sitemaps
	LastNotes []EnhancedEvent

	// for the archive sitemap
	PathPrefix string
	Data       []string
}

func (*SitemapPage) TemplateText() string { return tmplSitemap }
