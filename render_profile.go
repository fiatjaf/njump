package main

import (
	"bytes"
	"context"
	"html"
	"html/template"
	"net/http"
	"strings"
	"time"

	"fiatjaf.com/nostr/sdk"
)

func renderProfile(ctx context.Context, r *http.Request, w http.ResponseWriter, code string) {
	isEmbed := r.URL.Query().Get("embed") != ""

	isSitemap := false
	if strings.HasSuffix(code, ".xml") {
		code = code[:len(code)-4]
		isSitemap = true
	}

	isRSS := false
	if strings.HasSuffix(code, ".rss") {
		code = code[:len(code)-4]
		isRSS = true
	}

	pp := sdk.InputToProfile(ctx, code)
	if pp == nil {
		log.Warn().Str("code", code).Msg("invalid profile code")
		w.Header().Set("Cache-Control", "public, immutable, s-maxage=86400, max-age=86400")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: "invalid profile code", Clients: generateClientList(999999, code)}).Render(ctx, w)
		return
	}

	if banned, reason := isPubkeyBanned(pp.PublicKey); banned {
		deleteAllEventsFromPubKey(pp.PublicKey)
		w.Header().Set("Cache-Control", "public, immutable, s-maxage=604800, max-age=604800")
		log.Warn().Str("pubkey", pp.PublicKey.Hex()).Str("reason", reason).Msg("pubkey banned")
		http.Error(w, "pubkey banned", http.StatusNotFound)
		return
	}

	profile := sys.FetchProfileMetadata(ctx, pp.PublicKey)
	if isMaliciousBridged(profile) {
		deleteAllEventsFromPubKey(pp.PublicKey)
		w.Header().Set("Cache-Control", "public, immutable, s-maxage=604800, max-age=604800")
		log.Warn().Str("pubkey", pp.PublicKey.Hex()).Msg("pubkey malicious bridged blocked")
		http.Error(w, "profile is malicious", http.StatusNotFound)
		return
	}
	if is, _ := isExplicitContent(ctx, profile.Picture); is {
		deleteAllEventsFromPubKey(pp.PublicKey)
		w.Header().Set("Cache-Control", "public, immutable, s-maxage=604800, max-age=604800")
		log.Warn().Str("pubkey", pp.PublicKey.Hex()).Msg("pubkey explicit content blocked")
		http.Error(w, "profile is not allowed", http.StatusNotFound)
		return
	}

	var createdAt string
	if profile.Event != nil {
		createdAt = profile.Event.CreatedAt.Time().Format("2006-01-02T15:04:05Z07:00")
		w.Header().Set("ETag", profile.Event.ID.Hex())
	}

	var lastNotes []EnhancedEvent
	if !isEmbed {
		lastNotes, _ = authorLastNotes(ctx, profile.PubKey)
	}

	w.Header().Set("Cache-Control", "public, s-maxage=604800, max-age=604800, stale-while-revalidate=31536000")

	var err error
	if isSitemap {
		w.Header().Add("content-type", "text/xml")

		var buf bytes.Buffer
		buf.WriteString(XML_HEADER)
		err = SitemapTemplate.Render(&buf, &SitemapPage{
			Host:       s.Domain,
			ModifiedAt: createdAt,
			Metadata:   profile,
			LastNotes:  lastNotes,
		})
		if err == nil {
			w.Write(buf.Bytes())
		}
	} else if isRSS {
		w.Header().Add("content-type", "text/xml")

		var buf bytes.Buffer
		buf.WriteString(XML_HEADER)
		err = RSSTemplate.Render(&buf, &RSSPage{
			Host:       s.Domain,
			ModifiedAt: createdAt,
			Metadata:   profile,
			LastNotes:  lastNotes,
		})
		if err == nil {
			w.Write(buf.Bytes())
		}
	} else {
		w.Header().Add("content-type", "text/html")

		nprofile := profile.Nprofile(ctx, sys, 2)
		params := ProfilePageParams{
			HeadParams: HeadParams{IsProfile: true},
			Details: DetailsParams{
				HideDetails:     true,
				CreatedAt:       createdAt,
				KindDescription: kindNames[0],
				KindNIP:         kindNIPs[0],
				EventJSON:       toJSONHTML(profile.Event),
				Kind:            0,
				Metadata:        profile,
			},
			Metadata:                   profile,
			NormalizedAuthorWebsiteURL: normalizeWebsiteURL(profile.Website),
			RenderedAuthorAboutText:    template.HTML(basicFormatting(html.EscapeString(profile.About), false, false, false)),
			Nprofile:                   nprofile,
			AuthorRelays:               relaysPretty(ctx, profile.PubKey),
			LastNotes:                  lastNotes,
			Clients: generateClientList(0, nprofile,
				func(c ClientReference, s string) string {
					if c.ID == "nostrudel" {
						s = strings.Replace(s, "/n/", "/u/", 1)
					}
					if c.ID == "primal-web" {
						s = strings.Replace(
							strings.Replace(s, "/e/", "/p/", 1),
							nprofile, profile.Npub(), 1)
					}
					return s
				},
			),
		}

		// give this global context a timeout because it may used inside the template to validate the nip05 address
		ctx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()

		if isEmbed {
			err = embeddedProfileTemplate(params).Render(ctx, w)
		} else {
			err = profileTemplate(params).Render(ctx, w)
		}
	}

	if err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
	return
}
