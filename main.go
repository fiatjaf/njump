package main

import (
	"embed"
	"html"
	"net/http"
	"os"
	"text/template"

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

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal().Err(err).Msg("couldn't process envconfig.")
	}

	// initialize disk cache
	defer cache.initialize()()

	// initialize templates
	// use a mapping to expressly link the templates and share them between more kinds/types
	templateMapping["profile"] = "profile.html"
	templateMapping["note"] = "note.html"
	templateMapping["address"] = "other.html"
	templateMapping["relay"] = "relay.html"

	funcMap := template.FuncMap{
		"basicFormatting": basicFormatting,
		"mdToHTML":        mdToHTML,
		"escapeString":    html.EscapeString,
		"sanitizeXSS":     sanitizeXSS,
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
	http.HandleFunc("/", render)

	log.Print("listening at http://0.0.0.0:" + s.Port)
	if err := http.ListenAndServe("0.0.0.0:"+s.Port, nil); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
