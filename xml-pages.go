package main

import (
	_ "embed"

	"github.com/nbd-wtf/go-nostr/nip11"
	"github.com/nbd-wtf/go-nostr/sdk"
	"github.com/tylermmorton/tmpl"
)

var (
	//go:embed xml/sitemap.xml
	tmplSitemap     string
	SitemapTemplate = tmpl.MustCompile(&SitemapPage{})
)

type SitemapPage struct {
	Host       string
	ModifiedAt string

	// for the profile sitemap
	Metadata sdk.ProfileMetadata

	// for the relay sitemap
	RelayHostname string
	Info          nip11.RelayInformationDocument

	// for the profile and relay sitemaps
	LastNotes []EnhancedEvent

	// for the archive sitemap
	PathPrefix string
	Data       []string
}

func (*SitemapPage) TemplateText() string { return tmplSitemap }

var (
	//go:embed xml/sitemap-index.xml
	tmplSitemapIndex     string
	SitemapIndexTemplate = tmpl.MustCompile(&SitemapIndexPage{})
)

type SitemapIndexPage struct {
	Host  string
	Npubs []string
}

func (*SitemapIndexPage) TemplateText() string { return tmplSitemapIndex }

var (
	//go:embed xml/rss.xml
	tmplRSS     string
	RSSTemplate = tmpl.MustCompile(&RSSPage{})
)

type RSSPage struct {
	Host       string
	ModifiedAt string
	Title      string

	// for the profile RSS
	Metadata sdk.ProfileMetadata

	// for the relay RSS
	RelayHostname string
	Info          nip11.RelayInformationDocument

	// for the profile and relay RSSs
	LastNotes []EnhancedEvent

	// for the archive RSS
	PathPrefix string
	Data       []string
}

func (*RSSPage) TemplateText() string { return tmplRSS }
