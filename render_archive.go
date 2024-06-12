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

const (
	NPUBS_ARCHIVE  = iota
	RELAYS_ARCHIVE = iota
)

func renderArchive(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, "@.", r.Header.Get("user-agent"))

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
	pathPrefix := ""
	var area int
	if strings.HasPrefix(r.URL.Path[1:], "npubs-archive") {
		area = NPUBS_ARCHIVE
		prefix = "pa:"
		pathPrefix = ""
	} else if strings.HasPrefix(r.URL.Path[1:], "relays-archive") {
		area = RELAYS_ARCHIVE
		prefix = "ra:"
		pathPrefix = "r/"
	}

	keys := cache.GetPaginatedKeys(prefix, page, 5000)
	data := []string{}
	for i := 0; i < len(keys); i++ {
		switch area {
		case NPUBS_ARCHIVE:
			npub, _ := nip19.EncodePublicKey(keys[i][3:])
			data = append(data, npub)
		case RELAYS_ARCHIVE:
			data = append(data, trimProtocol(keys[i][3:]))
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
