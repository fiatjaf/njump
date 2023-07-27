package main

import (
	"context"
	"embed"
	"html"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
)

type Settings struct {
	Port string `envconfig:"PORT" default:"2999"`
}

//go:embed static/*
var static embed.FS

//go:embed templates/*
var templates embed.FS

var (
	s Settings

	tmpl            *template.Template
	templateMapping = make(map[string]string)

	log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
)

func updateArchives(ctx context.Context) {
	for {
		loadNpubsArchive(ctx)
		loadRelaysArchive(ctx)
		// Wait for 24 hours before executing the function again
		time.Sleep(24 * time.Hour)
	}
}

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}

	// initialize disk cache
	defer cache.initialize()()

	// initialize the function to update the npubs/relays archive
	ctx := context.Background()
	go updateArchives(ctx)

	// initialize templates
	// use a mapping to expressly link the templates and share them between more kinds/types
	templateMapping["profile"] = "profile.html"
	templateMapping["profile_sitemap"] = "sitemap.xml"
	templateMapping["note"] = "note.html"
	templateMapping["address"] = "other.html"
	templateMapping["relay"] = "relay.html"
	templateMapping["relay_sitemap"] = "sitemap.xml"

	funcMap := template.FuncMap{
		"basicFormatting": basicFormatting,
		"mdToHTML":        mdToHTML,
		"escapeString":    html.EscapeString,
		"sanitizeXSS":     sanitizeXSS,
		"trimProtocol":    trimProtocol,
	}

	tmpl = template.Must(
		template.New("tmpl").
			Funcs(funcMap).
			ParseFS(templates, "templates/*"),
	)

	// routes
	http.HandleFunc("/njump/image/", generate)
	http.HandleFunc("/njump/proxy/", proxy)
	http.Handle("/njump/static/", http.StripPrefix("/njump/", http.FileServer(http.FS(static))))
	http.HandleFunc("/npubs-archive/", renderArchive)
	http.HandleFunc("/relays-archive/", renderArchive)
	http.HandleFunc("/", render)

	log.Print("listening at http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe("0.0.0.0:"+s.Port, nil); err != nil {
		log.Fatal().Err(err).Msg("")
	}

	select {}

}
