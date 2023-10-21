package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port          string `envconfig:"PORT" default:"2999"`
	DiskCachePath string `envconfig:"DISK_CACHE_PATH" default:"/tmp/njump-cache"`
	Domain        string `envconfig:"DOMAIN" default:"njump.me"`
}

//go:embed static/*
var static embed.FS

//go:embed templates/*
var templates embed.FS

var (
	s   Settings
	log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
)

func updateArchives(ctx context.Context) {
	// do this so we don't run this every time we restart it locally
	time.Sleep(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			loadNpubsArchive(ctx)
			loadRelaysArchive(ctx)
		}
		time.Sleep(24 * time.Hour)
	}
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

	// initialize disk cache
	defer cache.initialize()()

	// initialize the function to update the npubs/relays archive
	ctx := context.Background()
	go updateArchives(ctx)

	// routes
	mux := http.NewServeMux()
	mux.Handle("/njump/static/", http.StripPrefix("/njump/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/relays-archive.xml", renderArchive)
	mux.HandleFunc("/npubs-archive.xml", renderArchive)
	mux.HandleFunc("/services/oembed", renderOEmbed)
	mux.HandleFunc("/relays-archive/", renderArchive)
	mux.HandleFunc("/npubs-archive/", renderArchive)
	mux.HandleFunc("/njump/image/", renderImage)
	mux.HandleFunc("/njump/proxy/", proxy)
	mux.HandleFunc("/favicon.ico", renderFavicon)
	mux.HandleFunc("/robots.txt", renderRobots)
	mux.HandleFunc("/r/", renderRelayPage)
	mux.HandleFunc("/try", redirectFromFormSubmit)
	mux.HandleFunc("/e/", redirectFromESlash)
	mux.HandleFunc("/p/", redirectFromPSlash)
	mux.HandleFunc("/", renderEvent)

	log.Print("listening at http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe("0.0.0.0:"+s.Port, cors.Default().Handler(mux)); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	select {}
}
