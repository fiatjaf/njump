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

	eventstore_badger "github.com/fiatjaf/eventstore/badger"
	"github.com/fiatjaf/khatru"
	"github.com/kelseyhightower/envconfig"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port              string   `envconfig:"PORT" default:"2999"`
	Domain            string   `envconfig:"DOMAIN" default:"njump.me"`
	DiskCachePath     string   `envconfig:"DISK_CACHE_PATH" default:"/tmp/njump-internal"`
	EventStorePath    string   `envconfig:"EVENT_STORE_PATH" default:"/tmp/njump-db"`
	TailwindDebug     bool     `envconfig:"TAILWIND_DEBUG"`
	SkipLanguageModel bool     `envconfig:"SKIP_LANGUAGE_MODEL"`
	RelayConfigPath   string   `envconfig:"RELAY_CONFIG_PATH"`
	TrustedPubKeys    []string `envconfig:"TRUSTED_PUBKEYS"`
}

//go:embed static/*
var static embed.FS

var (
	s                  Settings
	log                = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	tailwindDebugStuff template.HTML
)

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
		if !relayConfig.Valid() {
			log.Fatal().Err(err).Msgf("invalid relay config file %q", s.RelayConfigPath)
			return
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

		styleb, err := os.ReadFile("tailwind.css")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load tailwind.css")
			return
		}
		style := string(styleb)

		tailwindDebugStuff = template.HTML(fmt.Sprintf("<script src=\"https://cdn.tailwindcss.com?plugins=typography\"></script><script>\n%s</script><style type=\"text/tailwindcss\">%s</style>", config, style))
	}

	// image rendering stuff
	initializeImageDrawingStuff()

	// eventstore and internal db
	deinitCache := initCache()
	defer deinitCache()

	// initialize routines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go updateArchives(ctx)
	go deleteOldCachedEvents(ctx)

	// expose our internal cache as a relay (mostly for debugging purposes)
	relay := khatru.NewRelay()
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)
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
	mux.HandleFunc("/services/oembed", renderOEmbed)
	mux.HandleFunc("/relays-archive/", renderArchive)
	mux.HandleFunc("/npubs-archive/", renderArchive)
	mux.HandleFunc("/njump/image/", renderImage)
	mux.HandleFunc("/njump/proxy/", proxy)
	mux.HandleFunc("/robots.txt", renderRobots)
	mux.HandleFunc("/r/", renderRelayPage)
	mux.HandleFunc("/random", redirectToRandom)
	mux.HandleFunc("/e/", redirectFromESlash)
	mux.HandleFunc("/p/", redirectFromPSlash)
	mux.HandleFunc("/favicon.ico", redirectToFavicon)
	mux.HandleFunc("/embed/", renderEmbedjs)
	mux.HandleFunc("/", renderEvent)

	log.Print("listening at http://0.0.0.0:" + s.Port)
	server := &http.Server{Addr: "0.0.0.0:" + s.Port, Handler: cors.Default().Handler(relay)}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("")
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	<-sc
	server.Close()
}

func initCache() func() {
	// initialize disk cache
	deinit := cache.initialize()

	// initialize eventstore database
	if badgerBackend, ok := db.(*eventstore_badger.BadgerBackend); ok {
		// it may be NullStore, in which case we do nothing
		badgerBackend.Path = s.EventStorePath
	}
	db.Init()

	return func() {
		deinit()
		db.Close()
	}
}
