package main

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
)

func renderOEmbed(w http.ResponseWriter, r *http.Request) {
	// target := r.URL.Query().Get("url")
	data := 1

	format := r.URL.Query().Get("format")
	if format == "xml" {
		w.Header().Add("Content-Type", "text/xml")
		xml.NewEncoder(w).Encode(data)
	} else {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}
