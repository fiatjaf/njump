//go:generate tmpl bind ./...

package main

import (
	_ "embed"
	"html/template"

	"github.com/nbd-wtf/go-nostr"
	"github.com/tylermmorton/tmpl"
)

type TemplateID int

const (
	Profile TemplateID = iota
	Note
	LongForm
	TelegramInstantView
	Other
)

var (
	//go:embed templates/head_common.html
	tmplHeadCommon     string
	HeadCommonTemplate = tmpl.MustCompile(&HeadCommonPartial{})
)

//tmpl:bind head_common.html
type HeadCommonPartial struct {
	IsProfile bool
}

func (*HeadCommonPartial) TemplateText() string {
	return tmplHeadCommon
}

var (
	//go:embed templates/top.html
	tmplTop     string
	TopTemplate = tmpl.MustCompile(&TopPartial{})
)

//tmpl:bind top.html
type TopPartial struct{}

func (*TopPartial) TemplateText() string {
	return tmplTop
}

var (
	//go:embed templates/details.html
	tmplDetails     string
	DetailsTemplate = tmpl.MustCompile(&DetailsPartial{})
)

//tmpl:bind footer.html
type DetailsPartial struct {
	HideDetails     bool
	CreatedAt       string
	EventJSON       string
	Nevent          string
	Kind            int
	KindNIP         string
	KindDescription string
}

func (*DetailsPartial) TemplateText() string {
	return tmplDetails
}

var (
	//go:embed templates/footer.html
	tmplFooter     string
	FooterTemplate = tmpl.MustCompile(&FooterPartial{})
)

//tmpl:bind footer.html
type FooterPartial struct{}

func (*FooterPartial) TemplateText() string {
	return tmplFooter
}

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

func (*TelegramInstantViewPage) TemplateText() string {
	return tmplTelegramInstantView
}

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
	NextPage      string
	PrevPage      string
}

func (*ArchivePage) TemplateText() string {
	return tmplArchive
}

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

	IsParameterizedReplaceable bool
	Naddr                      string
	Npub                       string
	Kind                       int
	KindDescription            string
}

func (*OtherPage) TemplateText() string {
	return tmplOther
}
