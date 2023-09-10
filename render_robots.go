package main

import (
	"net/http"
)

func renderRobots(w http.ResponseWriter, r *http.Request) {
	typ := "robots"
	w.Header().Set("Cache-Control", "max-age=3600")

	params := map[string]any{
		"CanonicalHost": s.CanonicalHost,
	}

	if err := tmpl.ExecuteTemplate(w, templateMapping[typ], params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}
}
