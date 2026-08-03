package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/translated/lara-go/lara"
)

const translateMaxChars = 5000 // hard cap on a single request's text

var (
	laraOnce       sync.Once
	laraTranslator *lara.Translator
)

// laraClient lazily builds the Lara translator from the configured credentials.
func laraClient() *lara.Translator {
	laraOnce.Do(func() {
		creds := lara.NewCredentials(s.LaraAccessKeyID, s.LaraAccessKeySecret)
		laraTranslator = lara.NewTranslator(creds, nil)
	})
	return laraTranslator
}

// translateConfigured reports whether a translation backend has been set up.
// The translate button is only shown (and the endpoint only works) when it is.
func translateConfigured() bool {
	return s.LaraAccessKeyID != "" && s.LaraAccessKeySecret != ""
}

// translateProxy translates the post body through Lara. Going through the server
// keeps the API credentials out of the client and avoids CORS, so the browser
// only ever talks to njump's own origin. Lara auto-detects the source language
// and handles long text, so the whole body is sent in a single request.
func translateProxy(w http.ResponseWriter, r *http.Request) {
	if !translateConfigured() {
		http.Error(w, "translation not configured", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Q      string `json:"q"`
		Target string `json:"target"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Q = strings.TrimSpace(req.Q)
	if req.Q == "" || req.Target == "" {
		http.Error(w, "missing q or target", http.StatusBadRequest)
		return
	}
	if runes := []rune(req.Q); len(runes) > translateMaxChars {
		req.Q = string(runes[:translateMaxChars])
	}

	// empty source => Lara auto-detects the language.
	result, err := laraClient().Translate(req.Q, "", req.Target, lara.TranslateOptions{})
	if err != nil {
		log.Warn().Err(err).Msg("translation failed")
		http.Error(w, "translation service unavailable", http.StatusBadGateway)
		return
	}

	translated := ""
	if result.Translation.String != nil {
		translated = *result.Translation.String
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=86400")
	json.NewEncoder(w).Encode(map[string]string{
		"translatedText":   translated,
		"detectedLanguage": result.SourceLanguage,
	})
}
