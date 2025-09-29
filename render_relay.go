package main

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"fiatjaf.com/nostr/nip11"
)

func renderRelayPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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

	// last notes
	limit := 50
	if isSitemap {
		limit = 500
	}
	renderableLastNotes := make([]EnhancedEvent, 0, limit)
	var lastEventAt *time.Time
	for evt := range relayLastNotes(ctx, hostname, limit) {
		ee := NewEnhancedEvent(ctx, evt)
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

	// relay metadata
	info, _ := nip11.Fetch(ctx, hostname)
	if info.Name == "" {
		info.Name = hostname
	}

	if len(renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "public, immutable, max-age=86400, s-maxage=86400")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400, s-maxage=1200")
	}

	var err error
	if isSitemap {
		w.Header().Add("content-type", "text/xml")

		var buf bytes.Buffer
		buf.WriteString(XML_HEADER)
		err = SitemapTemplate.Render(&buf, &SitemapPage{
			Host:          s.Domain,
			ModifiedAt:    lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
			LastNotes:     renderableLastNotes,
			RelayHostname: hostname,
			Info:          info,
		})
		if err == nil {
			w.Write(buf.Bytes())
		}

	} else if isRSS {
		w.Header().Add("content-type", "text/xml")

		var buf bytes.Buffer
		buf.WriteString(XML_HEADER)
		err = RSSTemplate.Render(&buf, &RSSPage{
			Host:          s.Domain,
			ModifiedAt:    lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
			LastNotes:     renderableLastNotes,
			RelayHostname: hostname,
			Info:          info,
		})
		if err == nil {
			w.Write(buf.Bytes())
		}

	} else {
		err = relayTemplate(RelayPageParams{
			HeadParams: HeadParams{IsProfile: false},
			Info:       info,
			Hostname:   hostname,
			Proxy:      "https://" + hostname + "/proxy?src=",
			LastNotes:  renderableLastNotes,
			ModifiedAt: lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
			Clients:    generateClientList(-1, hostname),
		}).Render(ctx, w)
	}

	if err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
}
