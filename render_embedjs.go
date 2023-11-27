package main

import (
	_ "embed"
	"net/http"
)

func renderEmbedjs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	http.ServeFile(w, r, "templates/embed.js")
	return
}
