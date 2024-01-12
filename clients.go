package main

import (
	"strings"

	"github.com/a-h/templ"
)

type ClientReference struct {
	ID       string
	Name     string
	Base     string
	URL      templ.SafeURL
	Platform string
}

var (
	native = ClientReference{ID: "native", Name: "Your default app", Base: "nostr:{code}", Platform: "native"}

	nosta     = ClientReference{ID: "nosta", Name: "Nosta", Base: "https://nosta.me/{code}", Platform: "web"}
	snort     = ClientReference{ID: "snort", Name: "Snort", Base: "https://snort.social/{code}", Platform: "web"}
	satellite = ClientReference{ID: "satellite", Name: "Satellite", Base: "https://satellite.earth/@{code}", Platform: "web"}
	primalWeb = ClientReference{ID: "primal", Name: "Primal", Base: "https://primal.net/p/{code}", Platform: "web"}
	nostrudel = ClientReference{ID: "nostrudel", Name: "Nostrudel", Base: "https://nostrudel.ninja/#/n/{code}", Platform: "web"}
	nostter   = ClientReference{ID: "nostter", Name: "Nostter", Base: "https://nostter.app/{code}", Platform: "web"}
	iris      = ClientReference{ID: "iris", Name: "Iris", Base: "https://iris.to/{code}", Platform: "web"}
	coracle   = ClientReference{ID: "coracle", Name: "Coracle", Base: "https://coracle.social/{code}", Platform: "web"}

	zapStream = ClientReference{ID: "zap.stream", Name: "zap.stream", Base: "https://zap.stream/{code}", Platform: "web"}
	nostrrr   = ClientReference{ID: "nostrrr", Name: "Nostrrr", Base: "https://nostrrr.com/relay/{code}", Platform: "web"}
	flockstr  = ClientReference{ID: "flockstr", Name: "Flockstr", Base: "https://www.flockstr.com/event/{code}", Platform: "web"}

	yakihonne   = ClientReference{ID: "yakihonne", Name: "YakiHonne", Base: "https://yakihonne.com/article/{code}", Platform: "web"}
	habla       = ClientReference{ID: "habla", Name: "Habla", Base: "https://habla.news/a/{code}", Platform: "web"}
	highlighter = ClientReference{ID: "highlighter", Name: "Highlighter", Base: "https://highlighter.com/a/{code}", Platform: "web"}
	blogstack   = ClientReference{ID: "blogstack", Name: "Blogstack", Base: "https://blogstack.io/{code}", Platform: "web"}

	yanaAndroid     = ClientReference{ID: "yana", Name: "Yana", Base: "intent:{code}#Intent;scheme=nostr;package=yana.nostr;end`;", Platform: "android"}
	springAndroid   = ClientReference{ID: "spring", Name: "Spring", Base: "intent:{code}#Intent;scheme=nostr;package=com.nostr.universe;end`;", Platform: "android"}
	amethyst        = ClientReference{ID: "amethyst", Name: "Amethyst", Base: "intent:{code}#Intent;scheme=nostr;package=com.vitorpamplona.amethyst;end`;", Platform: "android"}
	freeFromAndroid = ClientReference{ID: "freefrom", Name: "FreeFrom", Base: "intent:{code}#Intent;scheme=nostr;package=com.freefrom;end`;", Platform: "android"}
	currentAndroid  = ClientReference{ID: "current", Name: "Current", Base: "intent:{code}#Intent;scheme=nostr;package=io.getcurrent.current;end`;", Platform: "android"}
	plebstrAndroid  = ClientReference{ID: "plebstr", Name: "Plebstr", Base: "intent:{code}#Intent;scheme=nostr;package=com.plebstr.client;end`;", Platform: "android"}

	nos         = ClientReference{ID: "nos", Name: "Nos", Base: "nos:{code}", Platform: "ios"}
	damus       = ClientReference{ID: "damus", Name: "Damus", Base: "damus:{code}", Platform: "ios"}
	nostur      = ClientReference{ID: "nostur", Name: "Nostur", Base: "nostur:{code}", Platform: "ios"}
	primalIOS   = ClientReference{ID: "primal", Name: "Primal", Base: "primal:{code}", Platform: "ios"}
	freeFromIOS = ClientReference{ID: "freefrom", Name: "FreeFrom", Base: "freefrom:{code}", Platform: "ios"}
	plebstrIOS  = ClientReference{ID: "plebstr", Name: "Plebstr", Base: "plebstr:{code}", Platform: "ios"}
)

func generateClientList(
	kind int,
	code string,
	withModifiers ...func(ClientReference, string) string,
) []ClientReference {
	var clients []ClientReference
	switch kind {
	case -1: // relays
		clients = []ClientReference{
			native,
			coracle, nostrrr,
		}
	case 1, 6:
		clients = []ClientReference{
			native,
			coracle, snort, nostter, nostrudel, primalWeb, satellite, iris,
			nos, damus, nostur, primalIOS, freeFromIOS, plebstrIOS,
			yanaAndroid, springAndroid, amethyst, currentAndroid, plebstrAndroid, freeFromAndroid,
		}
	case 0:
		clients = []ClientReference{
			native,
			nosta, coracle, snort, nostter, nostrudel, primalWeb, satellite, iris,
			nos, damus, nostur, primalIOS, freeFromIOS, plebstrIOS,
			yanaAndroid, springAndroid, amethyst, currentAndroid, plebstrAndroid, freeFromAndroid,
		}
	case 30023, 30024:
		clients = []ClientReference{
			native,
			yakihonne, habla, highlighter, blogstack,
			damus, nos, nostur,
			amethyst, springAndroid,
		}
	case 1063:
		clients = []ClientReference{
			native,
			snort, coracle,
			amethyst,
		}
	case 30311:
		clients = []ClientReference{
			native,
			zapStream, nostrudel,
			amethyst,
		}
	case 31922, 31923:
		clients = []ClientReference{
			native,
			coracle, flockstr,
			amethyst,
		}
	default:
		clients = []ClientReference{
			native,
			coracle, snort, nostter, nostrudel, primalWeb, satellite, iris,
			nos, damus, nostur, primalIOS, freeFromIOS, plebstrIOS,
			yanaAndroid, springAndroid, amethyst, currentAndroid, plebstrAndroid, freeFromAndroid,
		}
	}

	for i, c := range clients {
		clients[i].URL = templ.SafeURL(strings.Replace(c.Base, "{code}", code, -1))
		for _, modifier := range withModifiers {
			clients[i].URL = templ.SafeURL(modifier(c, string(clients[i].URL)))
		}
	}

	return clients
}
