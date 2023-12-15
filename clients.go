package main

import (
	"html/template"

	"github.com/nbd-wtf/go-nostr"
)

type ClientReference struct {
	ID       string
	Name     string
	URL      template.URL
	Platform string
}

func generateClientList(style Style, code string, event *nostr.Event) []ClientReference {
	clients := []ClientReference{
		{ID: "native", Name: "Your default app", URL: template.URL("nostr:" + code), Platform: "native"},
	}

	webClients_1_6 := []ClientReference{
		{ID: "snort", Name: "Snort", URL: template.URL("https://Snort.social/e/" + code), Platform: "web"},
		{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/n/" + code), Platform: "web"},
		{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/thread/" + event.ID), Platform: "web"},
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code), Platform: "web"},
		{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/thread/" + event.ID), Platform: "web"},
		{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code), Platform: "web"},
		{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code), Platform: "web"},
		{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code), Platform: "web"},
	}

	webClients_0 := []ClientReference{
		{ID: "nosta", Name: "Nosta", URL: template.URL("https://nosta.me/" + code), Platform: "web"},
		{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code), Platform: "web"},
		{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/@" + code), Platform: "web"},
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code), Platform: "web"},
		{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/profile/" + event.PubKey), Platform: "web"},
		{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/u/" + code), Platform: "web"},
		{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code), Platform: "web"},
		{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code), Platform: "web"},
	}

	webClients_30024 := []ClientReference{
		{ID: "yakihonne", Name: "YakiHonne", URL: template.URL("https://yakihonne.com/article/" + code), Platform: "web"},
		{ID: "habla", Name: "Habla", URL: template.URL("https://habla.news/a/" + code), Platform: "web"},
		{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code), Platform: "web"},
		{ID: "blogstack", Name: "Blogstack", URL: template.URL("https://blogstack.io/" + code), Platform: "web"},
	}

	webClients_1063 := []ClientReference{
		{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code), Platform: "web"},
		{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code), Platform: "web"},
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code), Platform: "web"},
	}

	androidClients := []ClientReference{
		{ID: "nostrmo", Name: "Nostrmo", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.github.haorendashu.nostrmo;end`;"), Platform: "android"},
		{ID: "amethyst", Name: "Amethyst", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.vitorpamplona.amethyst;end`;"), Platform: "android"},
		{ID: "yana", Name: "Yana", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=yana.nostr;end`;"), Platform: "android"},
		{ID: "spring", Name: "Spring", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.nostr.universe;end`;"), Platform: "android"},
		{ID: "snort-android", Name: "Snort", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=social.snort.android;end`;"), Platform: "android"},
		{ID: "freefrom", Name: "FreeFrom", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.freefrom;end`;"), Platform: "android"},
		{ID: "current", Name: "Current", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=io.getcurrent.current;end`;"), Platform: "android"},
		{ID: "plebstr", Name: "Plebstr", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.plebstr.client;end`;"), Platform: "android"},
		{ID: "nozzle", Name: "Nozzle", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.dluvian.nozzle;end`;"), Platform: "android"},
		{ID: "plasma", Name: "Plasma", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=social.plasma;end`;"), Platform: "android"},
		// {ID: "", Name: "", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=;end`;"), Platform: "app"},
	}

	iosClients := []ClientReference{
		{ID: "nos", Name: "Nos", URL: template.URL("nos:" + code), Platform: "ios"},
		{ID: "damus", Name: "Damus", URL: template.URL("damus:" + code), Platform: "ios"},
		{ID: "nostur", Name: "Nostur", URL: template.URL("nostur:" + code), Platform: "ios"},
		{ID: "primal", Name: "Primal", URL: template.URL("primal:" + code), Platform: "ios"},
	}

	clients = append(clients, androidClients...)
	clients = append(clients, iosClients...)

	if event.Kind == 1 || event.Kind == 6 {
		clients = append(clients, webClients_1_6...)
	} else if event.Kind == 0 {
		clients = append(clients, webClients_0...)
	} else if event.Kind == 30023 || event.Kind == 30024 {
		clients = append(clients, webClients_30024...)
	} else if event.Kind == 1063 {
		clients = append(clients, webClients_1063...)
	}

	return clients
}

func generateRelayBrowserClientList(host string) []ClientReference {
	return []ClientReference{
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/relays/" + host), Platform: "web"},
	}
}
