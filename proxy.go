package main

import (
	"github.com/fiatjaf/njump/i18n"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func proxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=6048000")

	src := r.URL.Query().Get("src")
	urlParsed, err := url.Parse(src)
	if err != nil {
		http.Error(w, i18n.Translate(r.Context(), "proxy.invalid_url", nil), http.StatusBadRequest)
		return
	}
	if urlParsed.Scheme != "http" && urlParsed.Scheme != "https" {
		http.Error(w, i18n.Translate(r.Context(), "proxy.bad_scheme", nil), http.StatusBadRequest)
		return
	}

	proxy := httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL = urlParsed
			r.Host = urlParsed.Host
		},
	}

	proxy.ServeHTTP(w, r)
}
