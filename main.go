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

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/khatru"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port                string `envconfig:"PORT" default:"2999"`
	Domain              string `envconfig:"DOMAIN" default:"njump.me"`
	ServiceURL          string `envconfig:"SERVICE_URL"`
	InternalDBPath      string `envconfig:"DISK_CACHE_PATH" default:"/tmp/njump-internal"`
	EventStorePath      string `envconfig:"EVENT_STORE_PATH" default:"/tmp/njump-db"`
	KVStorePath         string `envconfig:"KV_STORE_PATH" default:"/tmp/njump-kv"`
	HintsMemoryDumpPath string `envconfig:"HINTS_SAVE_PATH" default:"/tmp/njump-hints.json"`
	TailwindDebug       bool   `envconfig:"TAILWIND_DEBUG"`
	RelayConfigPath     string `envconfig:"RELAY_CONFIG_PATH"`
	MediaAlertAPIKey    string `envconfig:"MEDIA_ALERT_API_KEY"`
	ErrorLogPath        string `envconfig:"ERROR_LOG_PATH" default:"/tmp/njump-errors.jsonl"`

	TrustedPubKeysHex []string `envconfig:"TRUSTED_PUBKEYS"`
	trustedPubKeys    []nostr.PubKey
}

//go:embed static/*
var static embed.FS

var (
	s   Settings
	log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().Timestamp().Logger()
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

		s.trustedPubKeys = make([]nostr.PubKey, len(s.TrustedPubKeysHex))
		for i, pkhex := range s.TrustedPubKeysHex {
			s.trustedPubKeys[i] = nostr.MustPubKeyFromHex(pkhex)
		}
	}

	if len(s.trustedPubKeys) == 0 {
		s.trustedPubKeys = defaultTrustedPubKeys
	}

	// initialize error tracker
	if err := InitErrorTracker(s.ErrorLogPath); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize error tracker")
		return
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
			sys.FallbackRelays.URLs = relayConfig.Everything
		}
		if len(relayConfig.Profiles) > 0 {
			sys.MetadataRelays.URLs = relayConfig.Profiles
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

	// initialize routines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go updateArchives(ctx)
	go deleteOldCachedEvents(ctx)
	go outboxHintsFileLoaderSaver(ctx)

	// expose our internal cache as a relay (mostly for debugging purposes)
	relay := khatru.NewRelay()
	relay.ServiceURL = "https://" + s.Domain
	relay.UseEventstore(sys.Store, DB_MAX_LIMIT)
	relay.OnEvent = func(ctx context.Context, event nostr.Event) (reject bool, msg string) {
		return true, "this relay is not writable"
	}

	// admin
	setupRelayManagement(relay)

	// routes
	mux := relay.Router()
	mux.Handle("/njump/static/", http.StripPrefix("/njump/", http.FileServer(http.FS(static))))

	mux.HandleFunc("/relays-archive.xml", renderArchive)
	mux.HandleFunc("/npubs-archive.xml", renderArchive)
	mux.HandleFunc("/npubs-sitemaps.xml", renderSitemapIndex)
	mux.HandleFunc("/services/oembed", renderOEmbed)
	mux.HandleFunc("/njump/image/", renderImage)
	mux.HandleFunc("/image/", renderImage)
	mux.HandleFunc("/njump/proxy/", proxy)
	mux.HandleFunc("/proxy/", proxy)
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

	corsH := cors.Default()
	corsM := func(next http.HandlerFunc) http.HandlerFunc {
		return corsH.Handler(next).ServeHTTP
	}

	var mainHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		recoveryMiddleware(
			ipBlock(
				agentBlock(
					loggingMiddleware(
						semaphoreMiddleware(
							queueMiddleware(
								corsM(
									relay.ServeHTTP,
								),
							),
						),
					),
				),
			),
		)(w, r)
	}

	log.Print("listening at http://0.0.0.0:" + s.Port)
	server := &http.Server{Addr: "0.0.0.0:" + s.Port, Handler: mainHandler}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("server error")
			TrackGenericError("HTTP server failed", err)
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	<-sc
	server.Close()
}
