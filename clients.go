package main

import (
	"strings"

	"github.com/a-h/templ"
	"github.com/nbd-wtf/go-nostr"
)

type ClientReference struct {
	ID       string
	Name     string
	Base     string
	Platform string
}

func (c ClientReference) URL(code string) templ.SafeURL {
	return templ.SafeURL(strings.Replace(c.Base, "{code}", code, -1))
}

var (
	native = ClientReference{ID: "native", Name: "Your default app", Base: "nostr:{code}", Platform: "native"}

	nosta     = ClientReference{ID: "nosta", Name: "Nosta", Base: "https://nosta.me/{code}", Platform: "web"}
	snort     = ClientReference{ID: "snort", Name: "Snort", Base: "https://snort.social/p/{code}", Platform: "web"}
	satellite = ClientReference{ID: "satellite", Name: "Satellite", Base: "https://satellite.earth/@{code}", Platform: "web"}
	primalWeb = ClientReference{ID: "primal", Name: "Primal", Base: "https://primal.net/p/{code}", Platform: "web"}
	nostrudel = ClientReference{ID: "nostrudel", Name: "Nostrudel", Base: "https://nostrudel.ninja/#/u/{code}", Platform: "web"}
	nostter   = ClientReference{ID: "nostter", Name: "Nostter", Base: "https://nostter.app/{code}", Platform: "web"}
	iris      = ClientReference{ID: "iris", Name: "Iris", Base: "https://iris.to/{code}", Platform: "web"}
	coracle   = ClientReference{ID: "coracle", Name: "Coracle", Base: "https://coracle.social/{code}", Platform: "web"}

	zapStream = ClientReference{ID: "zap.stream", Name: "zap.stream", Base: "https://zap.stream/{code}", Platform: "web"}
	nostrrr   = ClientReference{ID: "nostrrr", Name: "Nostrrr", Base: "https://nostrrr.com/relay/{code}", Platform: "web"}

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

func generateClientList(code string, event *nostr.Event) []ClientReference {
	switch event.Kind {
	case 1, 6:
		return []ClientReference{
			native,
			coracle, snort, nostter, nostrudel, primalWeb, satellite, iris,
			nos, damus, nostur, primalIOS, freeFromIOS, plebstrIOS,
			yanaAndroid, springAndroid, amethyst, currentAndroid, plebstrAndroid, freeFromAndroid,
		}
	case 0:
		return []ClientReference{
			native,
			nosta, coracle, snort, nostter, nostrudel, primalWeb, satellite, iris,
			nos, damus, nostur, primalIOS, freeFromIOS, plebstrIOS,
			yanaAndroid, springAndroid, amethyst, currentAndroid, plebstrAndroid, freeFromAndroid,
		}
	case 30023, 30024:
		return []ClientReference{
			native,
			yakihonne, habla, highlighter, blogstack,
			damus, nos, nostur,
			amethyst, springAndroid,
		}
	case 1063:
		return []ClientReference{
			native,
			snort, coracle,
			amethyst,
		}
	case 30311:
		return []ClientReference{
			native,
			zapStream, nostrudel,
			amethyst,
		}
	default:
		return []ClientReference{
			native,
			coracle, snort, nostter, nostrudel, primalWeb, satellite, iris,
			nos, damus, nostur, primalIOS, freeFromIOS, plebstrIOS,
			yanaAndroid, springAndroid, amethyst, currentAndroid, plebstrAndroid, freeFromAndroid,
		}
	}
}

func generateRelayBrowserClientList(host string) []ClientReference {
	return []ClientReference{
		coracle,
		nostrrr,
	}
}
