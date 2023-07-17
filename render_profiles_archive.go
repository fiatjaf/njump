package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderProfilesArchive(w http.ResponseWriter, r *http.Request) {
	resultsPerPage := 100
	lastIndex := strings.LastIndex(r.URL.Path, "/")
	page := 1
	if lastIndex != -1 {
		pageString := r.URL.Path[lastIndex+1:]
		pageInt, err := strconv.Atoi(pageString)
		if err != nil {
			page = 1
		} else {
			page = pageInt
		}
	}

	keys := cache.GetPaginatedkeys("pa", page, resultsPerPage)
	npubs := []string{}
	for i := 0; i < len(keys); i++ {
		npub, _ := nip19.EncodePublicKey(keys[i])
		npubs = append(npubs, npub)
	}

	prevPage := page - 1
	nextPage := page + 1
	if len(keys) == 0 {
		prevPage = 0
		nextPage = 0
	}

	params := map[string]any{
		"nextPage": fmt.Sprint(nextPage),
		"prevPage": fmt.Sprint(prevPage),
		"data": npubs,
	}

	w.Header().Set("Cache-Control", "max-age=604800")

	if err := tmpl.ExecuteTemplate(w, "archive.html", params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}
}
