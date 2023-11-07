package main

import (
	"html/template"

	"github.com/nbd-wtf/go-nostr"
)

type ClientReference struct {
	ID   string
	Name string
	URL  template.URL
}

func generateClientList(style Style, code string, event *nostr.Event) []ClientReference {
	if event.Kind == 1 || event.Kind == 6 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "snort", Name: "Snort", URL: template.URL("https://Snort.social/e/" + code)},
			{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/n/" + code)},
			{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/thread/" + event.ID)},
			{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
			{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/thread/" + event.ID)},
			{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code)},
			{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code)},
			{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code)},
		}
	} else if event.Kind == 0 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "nosta", Name: "Nosta", URL: template.URL("https://nosta.me/" + code)},
			{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code)},
			{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/@" + code)},
			{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
			{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/profile/" + event.PubKey)},
			{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/u/" + code)},
			{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code)},
			{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code)},
		}
	} else if event.Kind == 30023 || event.Kind == 30024 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "yakihonne", Name: "YakiHonne", URL: template.URL("https://yakihonne.com/article/" + code)},
			{ID: "habla", Name: "Habla", URL: template.URL("https://habla.news/a/" + code)},
			{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code)},
			{ID: "blogstack", Name: "Blogstack", URL: template.URL("https://blogstack.io/" + code)},
		}
	} else if event.Kind == 1063 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code)},
			{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
		}
	} else if event.Kind == 30311 {
		return []ClientReference{
			{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
			{ID: "zap.stream", Name: "zap.stream", URL: template.URL("https://zap.stream/" + code)},
			{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/streams/" + code)},
		}
	}
	return nil
}

func generateRelayBrowserClientList(host string) []ClientReference {
	return []ClientReference{
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/relays/" + host)},
	}
}
