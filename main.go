package main

import (
	"net/http"
	"os"

	"github.com/rs/zerolog"
)

var log = zerolog.New(os.Stderr).Output(zerolog.ConsoleWriter{Out: os.Stdout}).
	With().Timestamp().Logger()

func main() {
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
