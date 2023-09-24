package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func renderArchive(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Path[1:]
	hostname := code[2:]
	typ := "archive"
	resultsPerPage := 50

	if strings.HasSuffix(hostname, ".xml") {
		typ = "archive_sitemap"
		resultsPerPage = 5000
	}

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
	area := ""
	if strings.HasPrefix(r.URL.Path[1:], "npubs-archive") {
		area = "npubs-archive"
	}

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

	// Generate a random duration between 2 and 6 hours
	minHours := 2
	maxHours := 6
	randomHours := rand.Intn(maxHours-minHours+1) + minHours
	randomDuration := time.Duration(randomHours) * time.Hour
	currentTime := time.Now()
	modifiedAt := currentTime.Add(-randomDuration).Format("2006-01-02T15:04:05Z07:00")

	params := map[string]any{
		"title":         title,
		"pathPrefix":    path_prefix,
		"data":          data,
		"modifiedAt":    modifiedAt,
		"paginationUrl": area,
		"nextPage":      fmt.Sprint(nextPage),
		"prevPage":      fmt.Sprint(prevPage),
		"s":             s,
	}

	if len(data) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	if err := tmpl.ExecuteTemplate(w, templateMapping[typ], params); err != nil {
		log.Error().Err(err).Msg("error rendering")
		return
	}
}
