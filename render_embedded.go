package main

import (
	"net/http"
)

func renderEmbedjs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	fileContent, _ := static.ReadFile("static/embed.js")
	w.Write(fileContent)
}
