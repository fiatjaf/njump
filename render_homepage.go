package main

import (
	"net/http"
)

func renderHomepage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")
	err := HomePageTemplate.Render(w, &HomePage{
		HeadCommonPartial: HeadCommonPartial{IsProfile: false},
		Host:              s.Domain,
	})
	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
}
