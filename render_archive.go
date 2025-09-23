package main

import (
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fiatjaf.com/leafdb"
	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

func renderArchive(w http.ResponseWriter, r *http.Request) {
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

	var data []string
	pathPrefix := ""
	if strings.HasPrefix(r.URL.Path[1:], "npubs-archive") {
		pathPrefix = ""
		data = make([]string, 0, 5000)
		params := leafdb.AnyQuery("pubkey-archive")
		params.Skip = (page - 1) * 5000
		params.Limit = 5000
		for val := range internal.View(params) {
			if pka, err := nostr.PubKeyFromHex(val.(*PubKeyArchive).Pubkey); err == nil {
				data = append(data, nip19.EncodeNpub(pka))
			}
		}
	} else if strings.HasPrefix(r.URL.Path[1:], "relays-archive") {
		data = []string{
			"pyramid.fiatjaf.com",
			"nostr.wine",
			"140.f7z.io",
		}
	}

	// Generate a random duration between 2 and 6 hours
	minHours := 2
	maxHours := 6
	randomHours := rand.Intn(maxHours-minHours+1) + minHours
	randomDuration := time.Duration(randomHours) * time.Hour
	currentTime := time.Now()
	modifiedAt := currentTime.Add(-randomDuration).Format("2006-01-02T15:04:05Z07:00")

	if len(data) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	w.Header().Add("content-type", "text/xml")
	w.Write([]byte(XML_HEADER))
	SitemapTemplate.Render(w, &SitemapPage{
		Host:       s.Domain,
		ModifiedAt: modifiedAt,
		PathPrefix: pathPrefix,
		Data:       data,
	})
}
