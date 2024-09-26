package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/fiatjaf/khatru"
	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port                string   `envconfig:"PORT" default:"2999"`
	Domain              string   `envconfig:"DOMAIN" default:"njump.me"`
	DiskCachePath       string   `envconfig:"DISK_CACHE_PATH" default:"/tmp/njump-internal"`
	EventStorePath      string   `envconfig:"EVENT_STORE_PATH" default:"/tmp/njump-db"`
	HintsMemoryDumpPath string   `envconfig:"HINTS_SAVE_PATH" default:"/tmp/njump-hints.json"`
	TailwindDebug       bool     `envconfig:"TAILWIND_DEBUG"`
	RelayConfigPath     string   `envconfig:"RELAY_CONFIG_PATH"`
	TrustedPubKeys      []string `envconfig:"TRUSTED_PUBKEYS"`
}

//go:embed static/*
var static embed.FS

var (
	s                  Settings
	log                = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	tailwindDebugStuff template.HTML
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fullURL := fmt.Sprintf("%s://%s%s", r.Proto, r.Host, r.URL.Path)
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}
		log.Debug().Str("Full URL:", fullURL).Msg("Request URL => ")
		log.Debug().Str("ip", r.Header.Get("X-Forwarded-For")).
			Str("user-agent", r.Header.Get("User-Agent")).Str("referer", r.Header.Get("Referer")).Msg("Request details => ")

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig")
		return
	} else {
		if canonicalHost := os.Getenv("CANONICAL_HOST"); canonicalHost != "" {
			s.Domain = canonicalHost
		}
	}

	if len(s.TrustedPubKeys) == 0 {
		s.TrustedPubKeys = defaultTrustedPubKeys
	}

	// eventstore and nostr system
	defer initSystem()()

	if s.RelayConfigPath != "" {
		configr, err := os.ReadFile(s.RelayConfigPath)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to load %q", s.RelayConfigPath)
			return
		}
		err = json.Unmarshal(configr, &relayConfig)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to load %q", s.RelayConfigPath)
			return
		}
		if len(relayConfig.Everything) > 0 {
			sys.FallbackRelays = relayConfig.Everything
		}
		if len(relayConfig.Profiles) > 0 {
			sys.MetadataRelays = relayConfig.Profiles
		}
	}

	// if we're in tailwind debug mode, initialize the runtime tailwind stuff
	if s.TailwindDebug {
		configb, err := os.ReadFile("tailwind.config.js")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load tailwind.config.js")
			return
		}
		config := strings.Replace(
			strings.Replace(
				string(configb),
				"plugins: [require('@tailwindcss/typography')]", "", 1,
			),
			"module.exports", "tailwind.config", 1,
		)

		styleb, err := os.ReadFile("base.css")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load base.css")
			return
		}
		style := string(styleb)

		tailwindDebugStuff = template.HTML(fmt.Sprintf("<script src=\"https://cdn.tailwindcss.com?plugins=typography\"></script><script>\n%s</script><style type=\"text/tailwindcss\">%s</style>", config, style))
	}

	// image rendering stuff
	initializeImageDrawingStuff()

	// internal db
	defer cache.initializeCache()()

	// initialize routines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go updateArchives(ctx)
	go deleteOldCachedEvents(ctx)
	go outboxHintsFileLoaderSaver(ctx)

	// expose our internal cache as a relay (mostly for debugging purposes)
	relay := khatru.NewRelay()
	relay.QueryEvents = append(relay.QueryEvents, sys.Store.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, sys.Store.DeleteEvent)
	relay.RejectEvent = append(relay.RejectEvent,
		func(context.Context, *nostr.Event) (bool, string) {
			return true, "this relay is not writable"
		},
	)

	// routes
	mux := relay.Router()
	mux.Handle("/njump/static/", http.StripPrefix("/njump/", http.FileServer(http.FS(static))))

	mux.HandleFunc("/relays-archive.xml", renderArchive)
	mux.HandleFunc("/npubs-archive.xml", renderArchive)
	mux.HandleFunc("/npubs-sitemaps.xml", renderSitemapIndex)
	mux.HandleFunc("/services/oembed", renderOEmbed)
	mux.HandleFunc("/njump/image/", renderImage)
	mux.HandleFunc("/njump/proxy/", proxy)
	mux.HandleFunc("/robots.txt", renderRobots)
	mux.HandleFunc("/r/", renderRelayPage)
	mux.HandleFunc("/random", redirectToRandom)
	mux.HandleFunc("/e/", redirectFromESlash)
	mux.HandleFunc("/p/", redirectFromPSlash)
	mux.HandleFunc("/favicon.ico", redirectToFavicon)
	mux.HandleFunc("/embed/{code}", renderEmbedjs)
	mux.HandleFunc("/about", renderAbout)
	mux.HandleFunc("/{code}", renderEvent)
	mux.HandleFunc("/{$}", renderHomepage)

	var mainHandler http.Handler = relay
	// apply http middlewares
	for _, middleware := range []func(http.Handler) http.Handler{
		cors.Default().Handler,
		agentBlock,
		cloudflareBlock,
		loggingMiddleware,
	} {
		mainHandler = middleware(mainHandler)
	}

	log.Print("listening at http://0.0.0.0:" + s.Port)
	server := &http.Server{Addr: "0.0.0.0:" + s.Port, Handler: mainHandler}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("")
		}
	}()

	// download list of cloudflare ips once a day
	go updateCloudflareRangesRoutine()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	<-sc
	server.Close()
}
