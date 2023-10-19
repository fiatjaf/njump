//go:generate tmpl bind ./...

package main

import (
	_ "embed"
	"html/template"

	"github.com/nbd-wtf/go-nostr"
	"github.com/tylermmorton/tmpl"
)

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
