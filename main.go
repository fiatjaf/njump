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
	ClientsConfigPath   string `envconfig:"CLIENTS_CONFIG_PATH"`
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

	if s.ClientsConfigPath != "" {
		loadClientsConfig(s.ClientsConfigPath)
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

	sub := http.NewServeMux()
	sub.HandleFunc("/services/oembed", renderOEmbed)
	sub.HandleFunc("/njump/image/", renderImage)
	sub.HandleFunc("/image/", renderImage)
	sub.HandleFunc("/njump/proxy/", proxy)
	sub.HandleFunc("/proxy/", proxy)
	sub.HandleFunc("/robots.txt", renderRobots)
	sub.HandleFunc("/r/", renderRelayPage)
	sub.HandleFunc("/random", redirectToRandom)
	sub.HandleFunc("/e/", redirectFromESlash)
	sub.HandleFunc("/p/", redirectFromPSlash)
	sub.HandleFunc("/favicon.ico", redirectToFavicon)
	sub.HandleFunc("/embed/{code}", renderEmbedjs)
	sub.HandleFunc("/about", renderAbout)
	sub.HandleFunc("/{code}", renderEvent)
	sub.HandleFunc("/{$}", renderHomepage)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		agentBlock(
			ipBlock(
				loggingMiddleware(
					queueMiddleware(
						sub.ServeHTTP,
					),
				),
			),
		)(w, r)
	})

	corsH := cors.Default()
	corsM := func(next http.HandlerFunc) http.HandlerFunc {
		return corsH.Handler(next).ServeHTTP
	}

	log.Print("listening at http://0.0.0.0:" + s.Port)
	server := &http.Server{Addr: "0.0.0.0:" + s.Port, Handler: corsM(relay.ServeHTTP)}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("server error")
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	<-sc
	server.Close()
}
