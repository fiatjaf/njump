package main

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/fiatjaf/njump/i18n"
)

var langRegex = regexp.MustCompile("^[a-z]{2}$")

var rtlLanguages = map[string]bool{
	"ar": true, // Arabic
	"he": true, // Hebrew  
	"fa": true, // Persian/Farsi
	"ur": true, // Urdu
}

func languageMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			raw    string
			source string
			lang   string
		)
		if v := r.URL.Query().Get("lang"); v != "" {
			raw = v
			source = "query"
			http.SetCookie(w, &http.Cookie{Name: "lang", Value: raw, Path: "/", MaxAge: 365 * 24 * 60 * 60})
		} else if c, err := r.Cookie("lang"); err == nil && c.Value != "" {
			raw = c.Value
			source = "cookie"
		}
		if raw == "" {
			if al := r.Header.Get("Accept-Language"); al != "" {
				raw = strings.SplitN(strings.Split(al, ",")[0], ";", 2)[0]
				source = "header"
			}
		}
		if raw == "" {
			raw = s.DefaultLanguage
			source = "default"
		}
		lang = strings.ToLower(raw)
		if i := strings.Index(lang, "-"); i != -1 {
			lang = lang[:i]
		}
		if !langRegex.MatchString(lang) {
			lang = s.DefaultLanguage
		}
		log.Debug().
			Str("source", source).
			Str("raw", raw).
			Str("lang", lang).
			Str("Accept-Language", r.Header.Get("Accept-Language")).
			Msg("resolved language for request")
		ctx := i18n.WithLanguage(r.Context(), lang)
		ctx = context.WithValue(ctx, "requestPath", r.URL.Path)
		ctx = context.WithValue(ctx, "isRTL", rtlLanguages[lang])
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
