package main

import (
	"net/http"
)

func renderHomepage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")
	err := homepageTemplate(HomePageParams{
		HeadParams: HeadParams{IsHome: true, IsProfile: false},
	}).Render(r.Context(), w)
	if err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
}
