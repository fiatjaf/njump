package main

import (
	"net/http"
)

func renderAbout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, s-maxage=3600, max-age=3600")
	err := aboutTemplate(AboutParams{
		HeadParams: HeadParams{IsAbout: true, IsProfile: false},
	}).Render(r.Context(), w)
	if err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
		LoggedError(err, "about page template rendering", r, nil)
	}
}
