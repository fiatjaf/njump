package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/pelletier/go-toml"
)

var kindNames = map[int]string{
	0:     "profile metadata",
	1:     "text note",
	2:     "relay recommendation",
	3:     "contact list",
	4:     "encrypted direct message",
	5:     "event deletion",
	6:     "repost",
	7:     "reaction",
	8:     "badge award",
	40:    "channel creation",
	41:    "channel metadata",
	42:    "channel message",
	43:    "channel hide message",
	44:    "channel mute user",
	1984:  "report",
	9735:  "zap",
	9734:  "zap request",
	10002: "relay list",
	30008: "profile badges",
	30009: "badge definition",
	30078: "app-specific data",
	30023: "article",
}

func generateClientList(code string, event *nostr.Event) []map[string]string {
	if strings.HasPrefix(code, "nevent") || strings.HasPrefix(code, "note") {
		return []map[string]string{
			{"name": "native client", "url": "nostr:" + code},
			{"name": "Snort", "url": "https://Snort.social/e/" + code},
			{"name": "Coracle", "url": "https://coracle.social/" + code},
			{"name": "Satellite", "url": "https://satellite.earth/thread/" + event.ID},
			{"name": "Iris", "url": "https://iris.to/" + code},
			{"name": "Yosup", "url": "https://yosup.app/thread/" + event.ID},
			{"name": "Nostr.band", "url": "https://nostr.band/" + code},
			{"name": "Primal", "url": "https://primal.net/thread/" + event.ID},
			{"name": "Nostribe", "url": "https://www.nostribe.com/post/" + event.ID},
			{"name": "Nostrid", "url": "https://web.nostrid.app/note/" + event.ID},
		}
	} else if strings.HasPrefix(code, "npub") || strings.HasPrefix(code, "nprofile") {
		return []map[string]string{
			{"name": "native client", "url": "nostr:" + code},
			{"name": "Snort", "url": "https://snort.social/p/" + code},
			{"name": "Coracle", "url": "https://coracle.social/" + code},
			{"name": "Satellite", "url": "https://satellite.earth/@" + code},
			{"name": "Iris", "url": "https://iris.to/" + code},
			{"name": "Yosup", "url": "https://yosup.app/profile/" + event.PubKey},
			{"name": "Nostr.band", "url": "https://nostr.band/" + code},
			{"name": "Primal", "url": "https://primal.net/profile/" + event.PubKey},
			{"name": "Nostribe", "url": "https://www.nostribe.com/profile/" + event.PubKey},
			{"name": "Nostrid", "url": "https://web.nostrid.app/account/" + event.PubKey},
		}
	} else if strings.HasPrefix(code, "naddr") {
		return []map[string]string{
			{"name": "native client", "url": "nostr:" + code},
			{"name": "habla", "url": "https://habla.news/a/" + code},
			{"name": "blogstack", "url": "https://blogstack.io/" + code},
		}
	} else {
		return []map[string]string{
			{"name": "native client", "url": "nostr:" + code},
		}
	}
}

func mergeMaps[K comparable, V any](m1 map[K]V, m2 map[K]V) map[K]V {
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

func prettyJsonOrRaw(j string) (map[string]interface{}, string) {
	var parsedContent map[string]interface{}
	if err := json.Unmarshal([]byte(j), &parsedContent); err == nil {
		if t, err := toml.Marshal(parsedContent); err == nil && len(t) > 0 {
			return parsedContent, string(t)
		}
	}
	return nil, j
}

func getPreviewStyle(r *http.Request) string {
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	accept := r.Header.Get("Accept")

	switch {
	case strings.Contains(ua, "telegrambot"):
		return "telegram"
	case strings.Contains(ua, "twitterbot"):
		return "twitter"
	case strings.Contains(ua, "mattermost"):
		return "mattermost"
	case strings.Contains(ua, "slack"):
		return "slack"
	case strings.Contains(ua, "discord"):
		return "discord"
	case strings.Contains(ua, "whatsapp"):
		return "whatsapp"
	case strings.Contains(accept, "text/html"):
		return ""
	default:
		return "unknown"
	}
}
