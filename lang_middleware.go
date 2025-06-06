package main

import (
	"net/http"
	"strings"

	"github.com/fiatjaf/njump/i18n"
)

func languageMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			al := r.Header.Get("Accept-Language")
			if al != "" {
				p := strings.Split(al, ",")[0]
				p = strings.SplitN(p, ";", 2)[0]
				lang = strings.TrimSpace(p)
			}
		}
		if lang == "" {
			lang = "en"
		}
		ctx := i18n.WithLanguage(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
