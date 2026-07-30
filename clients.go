package main

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

type ClientReference struct {
	ID       string
	Name     string
	Base     string
	URL      templ.SafeURL
	Platform string
}

type ClientsConfig struct {
	Clients      map[string]clientData `json:"clients"`
	KindMappings map[string][]string   `json:"kindMappings"`
}

type clientData struct {
	Name     string `json:"name"`
	Base     string `json:"base"`
	Platform string `json:"platform"`
}

var (
	clientConfig ClientsConfig
)

func generateClientList(
	kind int,
	code string,
	withModifiers ...func(ClientReference, string) string,
) []ClientReference {
	kindKey := strconv.Itoa(kind)
	clientIDs, ok := clientConfig.KindMappings[kindKey]
	if !ok {
		clientIDs = clientConfig.KindMappings["default"]
	}

	pubkey, eventID, dTag, relayHint := parseCodeFields(code)

	clients := make([]ClientReference, 0, len(clientIDs))
	for _, cid := range clientIDs {
		clientInfo, ok := clientConfig.Clients[cid]
		if !ok {
			continue
		}

		c := ClientReference{
			ID:       cid,
			Name:     clientInfo.Name,
			Base:     clientInfo.Base,
			Platform: clientInfo.Platform,
		}

		url := c.Base
		url = strings.ReplaceAll(url, "{code}", code)
		url = strings.ReplaceAll(url, "{relay_hint}", relayHint)
		url = strings.ReplaceAll(url, "{pubkey}", pubkey)
		url = strings.ReplaceAll(url, "{d_tag}", dTag)
		url = strings.ReplaceAll(url, "{id}", eventID)

		for _, modifier := range withModifiers {
			url = modifier(c, url)
		}
		c.URL = templ.SafeURL(url)

		clients = append(clients, c)
	}

	return clients
}

func parseCodeFields(code string) (pubkey, eventID, dTag, relayHint string) {
	prefix, decoded, err := nip19.Decode(code)
	if err != nil {
		// not a nip19 code, treat as raw value (e.g. relay hostname for kind -1)
		relayHint = stripRelayScheme(code)
		return
	}

	switch prefix {
	case "naddr":
		if ep, ok := decoded.(nostr.EntityPointer); ok {
			pubkey = ep.PublicKey.Hex()
			dTag = ep.Identifier
			relayHint = firstRelayHint(ep.Relays)
		}
	case "nevent":
		if ep, ok := decoded.(nostr.EventPointer); ok {
			pubkey = ep.Author.Hex()
			eventID = ep.ID.Hex()
			relayHint = firstRelayHint(ep.Relays)
		}
	case "nprofile":
		if pp, ok := decoded.(nostr.ProfilePointer); ok {
			pubkey = pp.PublicKey.Hex()
			relayHint = firstRelayHint(pp.Relays)
		}
	case "note":
		if ep, ok := decoded.(nostr.EventPointer); ok {
			eventID = ep.ID.Hex()
		}
	}
	return
}

func firstRelayHint(relays []string) string {
	if len(relays) == 0 {
		return ""
	}
	return stripRelayScheme(relays[0])
}

func stripRelayScheme(relay string) string {
	relay = strings.TrimPrefix(relay, "wss://")
	relay = strings.TrimPrefix(relay, "ws://")
	return relay
}
