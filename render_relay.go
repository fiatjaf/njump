package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr/nip11"
)

func renderRelayPage(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, "@.", r.Header.Get("user-agent"))

	hostname := r.URL.Path[3:]

	if strings.HasPrefix(hostname, "wss:/") || strings.HasPrefix(hostname, "ws:/") {
		hostname = trimProtocol(hostname)
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

	// relay metadata
	info, err := nip11.Fetch(r.Context(), hostname)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		errorPage := &ErrorPage{
			Message: "The relay you are looking for does not exist or is offline; check the name in the url or try later",
			Errors:  err.Error(),
		}
		errorPage.TemplateText()
		w.WriteHeader(http.StatusNotFound)
		ErrorTemplate.Render(w, errorPage)
		return
	}
	if info == nil {
		info = &nip11.RelayInformationDocument{
			Name: hostname,
		}
	}

	// last notes
	lastNotes := relayLastNotes(r.Context(), hostname, isSitemap)
	renderableLastNotes := make([]EnhancedEvent, len(lastNotes))
	lastEventAt := time.Now()
	if len(lastNotes) > 0 {
		lastEventAt = time.Unix(int64(lastNotes[0].CreatedAt), 0)
	}
	for i, levt := range lastNotes {
		renderableLastNotes[i] = EnhancedEvent{levt, []string{"wss://" + hostname}}
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
		RelayTemplate.Render(w, &RelayPage{
			HeadCommonPartial: HeadCommonPartial{IsProfile: false, TailwindDebugStuff: tailwindDebugStuff},
			ClientsPartial: ClientsPartial{
				Clients: generateRelayBrowserClientList(hostname),
			},
			Info:       info,
			Hostname:   hostname,
			Proxy:      "https://" + hostname + "/njump/proxy?src=",
			LastNotes:  renderableLastNotes,
			ModifiedAt: lastEventAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}
