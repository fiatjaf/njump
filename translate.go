package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var translateHTTPClient = &http.Client{Timeout: 30 * time.Second}

const (
	translateMaxChars   = 5000 // hard cap on a single request's text
	translateChunkChars = 480  // MyMemory rejects queries longer than 500 chars
)

// translateConfigured reports whether a translation backend has been set up.
// The translate button is only shown (and the endpoint only works) when it is.
func translateConfigured() bool {
	return s.TranslateAPIURL != ""
}

type myMemoryResponse struct {
	ResponseData struct {
		TranslatedText   string `json:"translatedText"`
		DetectedLanguage string `json:"detectedLanguage"`
	} `json:"responseData"`
	ResponseStatus  json.Number `json:"responseStatus"`
	ResponseDetails string      `json:"responseDetails"`
}

// translateProxy translates the post body through MyMemory. Going through the
// server keeps any API key/email out of the client and avoids CORS, so the
// browser only ever talks to njump's own origin. MyMemory caps a single query
// at 500 chars, so longer text is split into chunks here.
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

	var out strings.Builder
	detected := ""
	for _, chunk := range chunkText(req.Q, translateChunkChars) {
		translated, dl, err := myMemoryTranslate(chunk, req.Target)
		if err != nil {
			log.Warn().Err(err).Msg("translation failed")
			http.Error(w, "translation service unavailable", http.StatusBadGateway)
			return
		}
		if detected == "" {
			detected = dl
		}
		out.WriteString(translated)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=86400")
	json.NewEncoder(w).Encode(map[string]string{
		"translatedText":   out.String(),
		"detectedLanguage": detected,
	})
}

func myMemoryTranslate(text, target string) (translated, detected string, err error) {
	q := url.Values{}
	q.Set("q", text)
	q.Set("langpair", "Autodetect|"+target)
	if s.TranslateAPIKey != "" {
		q.Set("key", s.TranslateAPIKey)
	}
	if s.TranslateAPIEmail != "" {
		q.Set("de", s.TranslateAPIEmail)
	}

	endpoint := strings.TrimRight(s.TranslateAPIURL, "/") + "/get?" + q.Encode()
	resp, err := translateHTTPClient.Get(endpoint)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var mm myMemoryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&mm); err != nil {
		return "", "", err
	}
	if status := mm.ResponseStatus.String(); status != "" && status != "200" {
		return "", "", fmt.Errorf("mymemory %s: %s", status, mm.ResponseDetails)
	}
	return mm.ResponseData.TranslatedText, mm.ResponseData.DetectedLanguage, nil
}

// chunkText splits text into pieces of at most maxRunes characters, preferring
// to break on a newline or space so words stay intact. All characters
// (including the separators) are preserved, so the chunks rejoin into the
// original text.
func chunkText(text string, maxRunes int) []string {
	runes := []rune(text)
	var chunks []string
	for len(runes) > 0 {
		if len(runes) <= maxRunes {
			chunks = append(chunks, string(runes))
			break
		}
		cut := 0
		for i := maxRunes; i > maxRunes/2; i-- {
			if runes[i] == '\n' {
				cut = i + 1
				break
			}
		}
		if cut == 0 {
			for i := maxRunes; i > maxRunes/2; i-- {
				if runes[i] == ' ' {
					cut = i + 1
					break
				}
			}
		}
		if cut == 0 {
			cut = maxRunes // no boundary found (e.g. CJK), hard cut
		}
		chunks = append(chunks, string(runes[:cut]))
		runes = runes[cut:]
	}
	return chunks
}
