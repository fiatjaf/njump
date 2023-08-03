package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderArchive(w http.ResponseWriter, r *http.Request) {
	resultsPerPage := 50
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

	prefix := ""
	path_prefix := ""
	title := ""
	area := strings.Split(r.URL.Path[1:], "/")[0]
	if area == "npubs-archive" {
		prefix = "pa"
		path_prefix = ""
		title = "Nostr npubs archive"
	} else {
		prefix = "ra"
		path_prefix = "r/"
		title = "Nostr relays archive"
	}

	keys := cache.GetPaginatedkeys(prefix, page, resultsPerPage)
	data := []string{}
	for i := 0; i < len(keys); i++ {
		if area == "npubs-archive" {
			npub, _ := nip19.EncodePublicKey(keys[i])
			data = append(data, npub)
		} else {
			data = append(data, keys[i])
		}
	}

	prevPage := page - 1
	nextPage := page + 1
	if len(keys) == 0 {
		prevPage = 0
		nextPage = 0
	}

	params := map[string]any{
		"title":         title,
		"pathPrefix":    path_prefix,
		"data":          data,
		"paginationUrl": area,
		"nextPage":      fmt.Sprint(nextPage),
		"prevPage":      fmt.Sprint(prevPage),
	}

	if len(data) != 0 {
		w.Header().Set("Cache-Control", "max-age=86400")
	}

	if err := tmpl.ExecuteTemplate(w, "archive.html", params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}
}
