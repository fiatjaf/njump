package main

import (
	"embed"
	"html"
	"net/http"
	"os"
	"text/template"

	"github.com/rs/zerolog"
)

//go:embed static/*
var static embed.FS

//go:embed templates/*
var templates embed.FS

var (
	tmpl            *template.Template
	templateMapping = make(map[string]string)

	log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
)

func main() {
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "2999"
	}

	log.Print("listening at http://0.0.0.0:" + port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
