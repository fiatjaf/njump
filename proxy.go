package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func proxy(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, ":~", r.Header.Get("user-agent"))

	src := r.URL.Query().Get("src")
	urlParsed, err := url.Parse(src)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	if urlParsed.Scheme != "http" && urlParsed.Scheme != "https" {
		http.Error(w, "The URL scheme is neither HTTP nor HTTPS", http.StatusBadRequest)
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
