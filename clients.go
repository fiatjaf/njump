package main

import (
	"html/template"

	"github.com/nbd-wtf/go-nostr"
)

type ClientReference struct {
	ID   string
	Name string
	URL  template.URL
	Type string
}

func generateClientList(style Style, code string, event *nostr.Event) []ClientReference {
	clients := []ClientReference{
		{ID: "native", Name: "Your default app", URL: template.URL("nostr:" + code), Type: "app"},
	}

	webClients_1_6 := []ClientReference{
		{ID: "snort", Name: "Snort", URL: template.URL("https://Snort.social/e/" + code), Type: "web"},
		{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/n/" + code), Type: "web"},
		{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/thread/" + event.ID), Type: "web"},
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code), Type: "web"},
		{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/thread/" + event.ID), Type: "web"},
		{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code), Type: "web"},
		{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code), Type: "web"},
		{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code), Type: "web"},
	}

	webClients_0 := []ClientReference{
		{ID: "nosta", Name: "Nosta", URL: template.URL("https://nosta.me/" + code), Type: "web"},
		{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code), Type: "web"},
		{ID: "satellite", Name: "Satellite", URL: template.URL("https://satellite.earth/@" + code), Type: "web"},
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code), Type: "web"},
		{ID: "primal", Name: "Primal", URL: template.URL("https://primal.net/profile/" + event.PubKey), Type: "web"},
		{ID: "nostrudel", Name: "Nostrudel", URL: template.URL("https://nostrudel.ninja/#/u/" + code), Type: "web"},
		{ID: "nostter", Name: "Nostter", URL: template.URL("https://nostter.vercel.app/" + code), Type: "web"},
		{ID: "iris", Name: "Iris", URL: template.URL("https://iris.to/" + code), Type: "web"},
	}

	webClients_30024 := []ClientReference{
		{ID: "yakihonne", Name: "YakiHonne", URL: template.URL("https://yakihonne.com/article/" + code), Type: "web"},
		{ID: "habla", Name: "Habla", URL: template.URL("https://habla.news/a/" + code), Type: "web"},
		{ID: "highlighter", Name: "Highlighter", URL: template.URL("https://highlighter.com/a/" + code), Type: "web"},
		{ID: "blogstack", Name: "Blogstack", URL: template.URL("https://blogstack.io/" + code), Type: "web"},
	}

	webClients_1063 := []ClientReference{
		{ID: "native", Name: "your native client", URL: template.URL("nostr:" + code), Type: "web"},
		{ID: "snort", Name: "Snort", URL: template.URL("https://snort.social/p/" + code), Type: "web"},
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/" + code), Type: "web"},
	}

	switch style {
	case StyleIOS:
		clients = append(clients, []ClientReference{
			{ID: "nos", Name: "Nos", URL: template.URL("nos:" + code), Type: "app"},
			{ID: "damus", Name: "Damus", URL: template.URL("damus:" + code), Type: "app"},
			{ID: "nostur", Name: "Nostur", URL: template.URL("nostur:" + code), Type: "app"},
			{ID: "primal", Name: "Primal", URL: template.URL("primal:" + code), Type: "app"},
		}...)
		if event.Kind == 1 || event.Kind == 6 {
			clients = append(clients, webClients_1_6...)
		} else if event.Kind == 0 {
			clients = append(clients, webClients_0...)
		} else if event.Kind == 30023 || event.Kind == 30024 {
			clients = append(clients, webClients_30024...)
		} else if event.Kind == 1063 {
			clients = append(clients, webClients_1063...)
		}
	case StyleAndroid:
		clients = append(clients, []ClientReference{
			{ID: "nostrmo", Name: "Nostrmo", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.github.haorendashu.nostrmo;end`;"), Type: "app"},
			{ID: "amethyst", Name: "Amethyst", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.vitorpamplona.amethyst;end`;"), Type: "app"},
			{ID: "yana", Name: "Yana", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=yana.nostr;end`;"), Type: "app"},
			{ID: "spring", Name: "Spring", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.nostr.universe;end`;"), Type: "app"},
			{ID: "snort-app", Name: "Snort", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=social.snort.app;end`;"), Type: "app"},
			{ID: "freefrom", Name: "FreeFrom", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.freefrom;end`;"), Type: "app"},
			{ID: "current", Name: "Current", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=io.getcurrent.current;end`;"), Type: "app"},
			{ID: "plebstr", Name: "Plebstr", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.plebstr.client;end`;"), Type: "app"},
			{ID: "nozzle", Name: "Nozzle", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=com.dluvian.nozzle;end`;"), Type: "app"},
			{ID: "plasma", Name: "Plasma", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=social.plasma;end`;"), Type: "app"},
			// {ID: "", Name: "", URL: template.URL("intent:" + code + "#Intent;scheme=nostr;package=;end`;"), Type: "app"},
		}...)
		if event.Kind == 1 || event.Kind == 6 {
			clients = append(clients, webClients_1_6...)
		} else if event.Kind == 0 {
			clients = append(clients, webClients_0...)
		} else if event.Kind == 30023 || event.Kind == 30024 {
			clients = append(clients, webClients_30024...)
		} else if event.Kind == 1063 {
			clients = append(clients, webClients_1063...)
		}
	default:
		if event.Kind == 1 || event.Kind == 6 {
			clients = append(clients, webClients_1_6...)
		} else if event.Kind == 0 {
			clients = append(clients, webClients_0...)
		} else if event.Kind == 30023 || event.Kind == 30024 {
			clients = append(clients, webClients_30024...)
		} else if event.Kind == 1063 {
			clients = append(clients, webClients_1063...)
		} else {
			return nil
		}
	}

	return clients
}

func generateRelayBrowserClientList(host string) []ClientReference {
	return []ClientReference{
		{ID: "coracle", Name: "Coracle", URL: template.URL("https://coracle.social/relays/" + host), Type: "web"},
	}
}
