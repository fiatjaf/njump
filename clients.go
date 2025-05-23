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

const (
	platformWeb     = "web"
	platformIOS     = "ios"
	platformAndroid = "android"
)

var (
	native = ClientReference{ID: "native", Name: "Your default app", Base: "nostr:{code}", Platform: "native"}

	nosta         = ClientReference{ID: "nosta", Name: "Nosta", Base: "https://nosta.me/{code}", Platform: platformWeb}
	snort         = ClientReference{ID: "snort", Name: "Snort", Base: "https://snort.social/{code}", Platform: platformWeb}
	olasWeb       = ClientReference{ID: "olas", Name: "Olas", Base: "https://olas.app/e/{code}", Platform: platformWeb}
	primalWeb     = ClientReference{ID: "primal", Name: "Primal", Base: "https://primal.net/e/{code}", Platform: platformWeb}
	lumilumi      = ClientReference{ID: "lumilumi", Name: "Lumilumi", Base: "https://lumilumi.app/{code}", Platform: platformWeb}
	lumilumiRelay = ClientReference{ID: "lumilumi", Name: "Lumilumi", Base: "https://lumilumi.app/relay/wss%3A%2F%2F{code}", Platform: platformWeb}
	nostrudel     = ClientReference{ID: "nostrudel", Name: "Nostrudel", Base: "https://nostrudel.ninja/l/{code}", Platform: platformWeb}
	nostter       = ClientReference{ID: "nostter", Name: "Nostter", Base: "https://nostter.app/{code}", Platform: platformWeb}
	nostterRelay  = ClientReference{ID: "nostter", Name: "Nostter", Base: "https://nostter.app/relays/wss%3A%2F%2F{code}", Platform: platformWeb}
	jumble        = ClientReference{ID: "jumble", Name: "Jumble", Base: "https://jumble.social/notes/{code}", Platform: platformWeb}
	jumbleRelay   = ClientReference{ID: "jumble", Name: "Jumble", Base: "https://jumble.social/?r=wss://{code}", Platform: platformWeb}
	coracle       = ClientReference{ID: "coracle", Name: "Coracle", Base: "https://coracle.social/{code}", Platform: platformWeb}
	coracleRelay  = ClientReference{ID: "coracle", Name: "Coracle", Base: "https://coracle.social/relays/wss%3A%2F%2F{code}", Platform: platformWeb}
	relayTools    = ClientReference{ID: "relay.tools", Name: "relay.tools", Base: "https://relay.tools/posts/?relay=wss://{code}"}
	iris          = ClientReference{ID: "iris", Name: "Iris", Base: "https://iris.to/{code}", Platform: "web"}

	zapStream = ClientReference{ID: "zap.stream", Name: "zap.stream", Base: "https://zap.stream/{code}", Platform: platformWeb}

	yakihonne = ClientReference{ID: "yakihonne", Name: "YakiHonne", Base: "https://yakihonne.com/{code}", Platform: platformWeb}
	habla     = ClientReference{ID: "habla", Name: "Habla", Base: "https://habla.news/a/{code}", Platform: platformWeb}

	voyage           = ClientReference{ID: "voyage", Name: "Voyage", Base: "intent:{code}#Intent;scheme=nostr;package=com.dluvian.voyage;end`;", Platform: platformAndroid}
	olasAndroid      = ClientReference{ID: "olas", Name: "Olas", Base: "intent:{code}#Intent;scheme=nostr;package=com.pablof7z.snapstr;end`;", Platform: platformAndroid}
	primalAndroid    = ClientReference{ID: "primal", Name: "Primal", Base: "intent:{code}#Intent;scheme=nostr;package=net.primal.android;end`;", Platform: platformAndroid}
	yakihonneAndroid = ClientReference{ID: "yakihonne", Name: "Yakihonne", Base: "intent:{code}#Intent;scheme=nostr;package=com.yakihonne.yakihonne;end`;", Platform: platformAndroid}
	freeFromAndroid  = ClientReference{ID: "freefrom", Name: "FreeFrom", Base: "intent:{code}#Intent;scheme=nostr;package=com.freefrom;end`;", Platform: platformAndroid}
	yanaAndroid      = ClientReference{ID: "yana", Name: "Yana", Base: "intent:{code}#Intent;scheme=nostr;package=yana.nostr;end`;", Platform: platformAndroid}
	amethyst         = ClientReference{ID: "amethyst", Name: "Amethyst", Base: "intent:{code}#Intent;scheme=nostr;package=com.vitorpamplona.amethyst;end`;", Platform: platformAndroid}

	nos          = ClientReference{ID: "nos", Name: "Nos", Base: "nos:{code}", Platform: platformIOS}
	damus        = ClientReference{ID: "damus", Name: "Damus", Base: "damus:{code}", Platform: platformIOS}
	nostur       = ClientReference{ID: "nostur", Name: "Nostur", Base: "nostur:{code}", Platform: platformIOS}
	olasIOS      = ClientReference{ID: "olas", Name: "Olas", Base: "olas:{code}", Platform: platformIOS}
	primalIOS    = ClientReference{ID: "primal", Name: "Primal", Base: "primal:{code}", Platform: platformIOS}
	freeFromIOS  = ClientReference{ID: "freefrom", Name: "FreeFrom", Base: "freefrom:{code}", Platform: platformIOS}
	yakihonneIOS = ClientReference{ID: "yakihonne", Name: "Yakihonne", Base: "yakihhone:{code}", Platform: platformIOS}

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
			jumbleRelay, coracleRelay, relayTools, nostterRelay, lumilumiRelay,
		}
	case 1, 6:
		clients = []ClientReference{
			native,
			damus, nostur, freeFromIOS, yakihonneIOS, nos, primalIOS,
			voyage, yakihonneAndroid, primalAndroid, freeFromAndroid, yanaAndroid,
			coracle, snort, jumble, nostter, nostrudel, primalWeb, iris, lumilumi,
		}
	case 20:
		clients = []ClientReference{
			native,
			olasAndroid,
			olasIOS,
			jumble, olasWeb, coracle, lumilumi,
		}
	case 0:
		clients = []ClientReference{
			native,
			nos, damus, nostur, primalIOS, freeFromIOS, yakihonneIOS,
			voyage, yakihonneAndroid, yanaAndroid, freeFromAndroid, primalAndroid,
			nosta, coracle, snort, nostter, nostrudel, primalWeb, iris, lumilumi,
		}
	case 30023, 30024:
		clients = []ClientReference{
			native,
			damus, nos, nostur, yakihonneIOS,
			yakihonneAndroid, amethyst,
			yakihonne, habla, lumilumi,
		}
	case 1063:
		clients = []ClientReference{
			native,
			amethyst,
			snort, coracle, nostrudel, lumilumi,
		}
	case 9802:
		clients = []ClientReference{
			coracle,
			nostrudel, lumilumi,
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
			yakihonne, coracle, snort, nostter, nostrudel, primalWeb, iris, lumilumi,
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
