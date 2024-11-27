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

	nosta        = ClientReference{ID: "nosta", Name: "Nosta", Base: "https://nosta.me/{code}", Platform: "web"}
	snort        = ClientReference{ID: "snort", Name: "Snort", Base: "https://snort.social/{code}", Platform: "web"}
	primalWeb    = ClientReference{ID: "primal", Name: "Primal", Base: "https://primal.net/e/{code}", Platform: "web"}
	nostrudel    = ClientReference{ID: "nostrudel", Name: "Nostrudel", Base: "https://nostrudel.ninja/#/n/{code}", Platform: "web"}
	nostter      = ClientReference{ID: "nostter", Name: "Nostter", Base: "https://nostter.app/{code}", Platform: "web"}
	nostterRelay = ClientReference{ID: "nostter", Name: "Nostter", Base: "https://nostter.app/relays/{code}", Platform: "web"}
	coracle      = ClientReference{ID: "coracle", Name: "Coracle", Base: "https://coracle.social/{code}", Platform: "web"}
	coracleRelay = ClientReference{ID: "coracle", Name: "Coracle", Base: "https://coracle.social/relays/{code}", Platform: "web"}

	zapStream      = ClientReference{ID: "zap.stream", Name: "zap.stream", Base: "https://zap.stream/{code}", Platform: "web"}
	nostrrrRelay   = ClientReference{ID: "nostrrr", Name: "Nostrrr", Base: "https://nostrrr.com/relay/{code}", Platform: "web"}
	nostrrrProfile = ClientReference{ID: "nostrrr", Name: "Nostrrr", Base: "https://nostrrr.com/p/{code}", Platform: "web"}

	yakihonne   = ClientReference{ID: "yakihonne", Name: "YakiHonne", Base: "https://yakihonne.com/{code}", Platform: "web"}
	habla       = ClientReference{ID: "habla", Name: "Habla", Base: "https://habla.news/a/{code}", Platform: "web"}
	highlighter = ClientReference{ID: "highlighter", Name: "Highlighter", Base: "https://highlighter.com/a/{code}", Platform: "web"}
	notestack   = ClientReference{ID: "notestack", Name: "Notestack", Base: "https://notestack.com/{code}", Platform: "web"}

	voyage           = ClientReference{ID: "voyage", Name: "Voyage", Base: "intent:{code}#Intent;scheme=nostr;package=com.dluvian.voyage;end`;", Platform: "android"}
	primalAndroid    = ClientReference{ID: "primal", Name: "Primal", Base: "intent:{code}#Intent;scheme=nostr;package=net.primal.android;end`;", Platform: "android"}
	yakihonneAndroid = ClientReference{ID: "yakihonne", Name: "Yakihonne", Base: "intent:{code}#Intent;scheme=nostr;package=com.yakihonne.yakihonne;end`;", Platform: "android"}
	freeFromAndroid  = ClientReference{ID: "freefrom", Name: "FreeFrom", Base: "intent:{code}#Intent;scheme=nostr;package=com.freefrom;end`;", Platform: "android"}
	yanaAndroid      = ClientReference{ID: "yana", Name: "Yana", Base: "intent:{code}#Intent;scheme=nostr;package=yana.nostr;end`;", Platform: "android"}
	amethyst         = ClientReference{ID: "amethyst", Name: "Amethyst", Base: "intent:{code}#Intent;scheme=nostr;package=com.vitorpamplona.amethyst;end`;", Platform: "android"}

	nos          = ClientReference{ID: "nos", Name: "Nos", Base: "nos:{code}", Platform: "ios"}
	damus        = ClientReference{ID: "damus", Name: "Damus", Base: "damus:{code}", Platform: "ios"}
	nostur       = ClientReference{ID: "nostur", Name: "Nostur", Base: "nostur:{code}", Platform: "ios"}
	primalIOS    = ClientReference{ID: "primal", Name: "Primal", Base: "primal:{code}", Platform: "ios"}
	freeFromIOS  = ClientReference{ID: "freefrom", Name: "FreeFrom", Base: "freefrom:{code}", Platform: "ios"}
	yakihonneIOS = ClientReference{ID: "yakihonne", Name: "Yakihonne", Base: "yakihhone:{code}", Platform: "ios"}

	wikistr     = ClientReference{ID: "wikistr", Name: "Wikistr", Base: "https://Wikistr.com/{handle}*{authorPubkey}", Platform: "web"}
	wikifreedia = ClientReference{ID: "wikifreedia", Name: "Wikifreedia", Base: "https://wikifreedia.xyz/{handle}/{npub}", Platform: "web"}
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
			nostterRelay, coracleRelay, nostrrrRelay,
		}
	case 1, 6:
		clients = []ClientReference{
			native,
			damus, nostur, freeFromIOS, yakihonneIOS, nos, primalIOS,
			voyage, yakihonneAndroid, primalAndroid, freeFromAndroid, yanaAndroid,
			coracle, snort, nostter, nostrudel, primalWeb,
		}
	case 0:
		clients = []ClientReference{
			native,
			nos, damus, nostur, primalIOS, freeFromIOS, yakihonneIOS,
			voyage, yakihonneAndroid, yanaAndroid, freeFromAndroid, primalAndroid,
			nostrrrProfile, nosta, coracle, snort, nostter, nostrudel, primalWeb,
		}
	case 30023, 30024:
		clients = []ClientReference{
			native,
			damus, nos, nostur, yakihonneIOS,
			yakihonneAndroid, amethyst,
			highlighter, yakihonne, habla, notestack,
		}
	case 1063:
		clients = []ClientReference{
			native,
			amethyst,
			snort, coracle, nostrudel,
		}
	case 30311:
		clients = []ClientReference{
			native,
			amethyst,
			nostur,
			zapStream, coracle, nostrudel,
		}
	case 30818:
		clients = []ClientReference{
			native,
			wikistr, wikifreedia,
		}
	case 31922, 31923:
		clients = []ClientReference{
			native,
			coracle,
		}
	default:
		clients = []ClientReference{
			native,
			yakihonneIOS, nos, damus, nostur, primalIOS, freeFromIOS,
			voyage, amethyst, yakihonneAndroid, yanaAndroid, freeFromAndroid, voyage,
			yakihonne, coracle, snort, nostter, nostrudel, primalWeb,
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
