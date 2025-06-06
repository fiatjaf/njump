package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/fiatjaf/njump/i18n"
	"github.com/nbd-wtf/go-nostr/nip11"
)

func renderRelayPage(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Path[3:]

	if strings.HasPrefix(hostname, "wss:/") || strings.HasPrefix(hostname, "ws:/") {
		hostname = trimProtocolAndEndingSlash(hostname)
		http.Redirect(w, r, "/r/"+hostname, http.StatusFound)
		return
	}

	isSitemap := false
	if strings.HasSuffix(hostname, ".xml") {
		hostname = hostname[:len(hostname)-4]
		isSitemap = true
	}

	isRSS := false
	if strings.HasSuffix(hostname, ".rss") {
		hostname = hostname[:len(hostname)-4]
		isRSS = true
	}

	if len(hostname) < 3 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// relay metadata
	info, _ := nip11.Fetch(r.Context(), hostname)
	if info.Name == "" {
		info.Name = hostname
	}

	// last notes
	limit := 50
	if isSitemap {
		limit = 500
	}
	renderableLastNotes := make([]EnhancedEvent, 0, limit)
	var lastEventAt *time.Time
	for evt := range relayLastNotes(r.Context(), hostname, limit) {
		ee := NewEnhancedEvent(r.Context(), evt)
		ee.relays = []string{"wss://" + hostname}
		renderableLastNotes = append(renderableLastNotes, ee)
		if lastEventAt == nil {
			last := time.Unix(int64(evt.CreatedAt), 0)
			lastEventAt = &last
		}
	}
	if lastEventAt == nil {
		now := time.Now()
		lastEventAt = &now
	}

	if len(renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "max-age=60")
	}

	if isSitemap {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		SitemapTemplate.Render(w, &SitemapPage{
			Host:          s.Domain,
			ModifiedAt:    lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
			LastNotes:     renderableLastNotes,
			RelayHostname: hostname,
			Info:          info,
		})

	} else if isRSS {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		RSSTemplate.Render(w, &RSSPage{
			Host:          s.Domain,
			ModifiedAt:    lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
			LastNotes:     renderableLastNotes,
			RelayHostname: hostname,
			Info:          info,
		})

	} else {
		relayTemplate(RelayPageParams{
			HeadParams: HeadParams{
				IsProfile: false,
				Lang:      i18n.LanguageFromContext(r.Context()),
			},
			Info:       info,
			Hostname:   hostname,
			Proxy:      "https://" + hostname + "/njump/proxy?src=",
			LastNotes:  renderableLastNotes,
			ModifiedAt: lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
			Clients:    generateClientList(-1, hostname),
		}).Render(r.Context(), w)
	}
}
