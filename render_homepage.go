package main

import (
	"net/http"

	"github.com/fiatjaf/njump/i18n"
)

func renderHomepage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")
	err := homepageTemplate(HomePageParams{
		HeadParams: HeadParams{
			IsHome:    true,
			IsProfile: false,
			Lang:      i18n.LanguageFromContext(r.Context()),
		},
	}).Render(r.Context(), w)
	if err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
}
