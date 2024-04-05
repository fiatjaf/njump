package main

import (
	"net/http"
)

func renderAbout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")
	err := aboutTemplate(AboutParams{
		HeadParams: HeadParams{IsAbout: true, IsProfile: false},
	}).Render(r.Context(), w)
	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
}
