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
	clients := []ClientReference{
		{ID: "native", Name: "Your native client", URL: template.URL("nostr:" + code)},
	}

	switch style {
	case StyleIOS:
		clients = append(clients, []ClientReference{
			{ID: "nos", Name: "Nos", URL: template.URL("nos:" + code)},
			{ID: "damus", Name: "Damus", URL: template.URL("damus:" + code)},
			{ID: "nostur", Name: "Nostur", URL: template.URL("nostur:" + code)},
			{ID: "primal", Name: "Primal", URL: template.URL("primal:" + code)},
		}...)
	case StyleAndroid:
		clients = append(clients, []ClientReference{
			{ID: "nostrmo", Name: "Nostrmo", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.github.haorendashu.nostrmo;end`;")},
			{ID: "amethyst", Name: "Amethyst", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.vitorpamplona.amethyst;end`;")},
			{ID: "yana", Name: "Yana", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=yana.nostr;end`;")},
			{ID: "spring", Name: "Spring", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.nostr.universe;end`;")},
			{ID: "snort-app", Name: "Snort", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=social.snort.app;end`;")},
			{ID: "freefrom", Name: "FreeFrom", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.freefrom;end`;")},
			{ID: "current", Name: "Current", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=io.getcurrent.current;end`;")},
			{ID: "plebstr", Name: "Plebstr", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.plebstr.client;end`;")},
			{ID: "nozzle", Name: "Nozzle", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.dluvian.nozzle;end`;")},
			{ID: "plasma", Name: "Plasma", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=social.plasma;end`;")},
			// {ID: "", Name: "", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=;end`;")},
		}...)
	default:
		if event.Kind == 1 || event.Kind == 6 {
			clients = append(clients, []ClientReference{
				{ID: "snort", Name: "Snort", URL: template.URL("https://Snort.social/e/" + code)},
				{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/n/" + code)},
				{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/thread/" + event.ID)},
				{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
				{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/thread/" + event.ID)},
				{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code)},
				{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code)},
				{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code)},
			}...)
		} else if event.Kind == 0 {
			clients = append(clients, []ClientReference{
				{ID: "nosta", Name: "Nosta", URL: template.URL("https://nosta.me/" + code)},
				{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code)},
				{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/@" + code)},
				{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
				{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/profile/" + event.PubKey)},
				{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/u/" + code)},
				{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code)},
				{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code)},
			}...)
		} else if event.Kind == 30023 || event.Kind == 30024 {
			clients = append(clients, []ClientReference{
				{ID: "yakihonne", Name: "YakiHonne", URL: template.URL("https://yakihonne.com/article/" + code)},
				{ID: "habla", Name: "Habla", URL: template.URL("https://habla.news/a/" + code)},
				{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code)},
				{ID: "blogstack", Name: "Blogstack", URL: template.URL("https://blogstack.io/" + code)},
			}...)
		} else if event.Kind == 1063 {
			clients = append(clients, []ClientReference{
				{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code)},
				{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code)},
				{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code)},
			}...)
		} else {
			return nil
		}
	}

	return clients
}

func generateRelayBrowserClientList(host string) []ClientReference {
	return []ClientReference{
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/relays/" + host)},
	}
}
