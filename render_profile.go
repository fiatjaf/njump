package main

import (
	"context"
	"html"
	"html/template"
	"net/http"
	"strings"
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

	profile, err := sys.FetchProfileFromInput(ctx, code)
	if err != nil {
		log.Warn().Err(err).Str("code", code).Msg("error fetching profile on render_profile")
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusNotFound)

		errorTemplate(ErrorPageParams{Errors: err.Error()}).Render(ctx, w)
		return
	} else if profile.Event != nil {
		internal.scheduleEventExpiration(profile.Event.ID)
	}

	var createdAt string
	if profile.Event != nil {
		createdAt = profile.Event.CreatedAt.Time().Format("2006-01-02T15:04:05Z07:00")
	}

	var lastNotes []EnhancedEvent
	var cacheControl string = "max-age=86400"
	if !isEmbed {
		var justFetched bool
		lastNotes, justFetched = authorLastNotes(ctx, profile.PubKey)
		if justFetched && profile.Event != nil {
			cacheControl = "only-if-cached"
		}
	}

	w.Header().Set("Cache-Control", cacheControl)

	if isSitemap {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		err = SitemapTemplate.Render(w, &SitemapPage{
			Host:       s.Domain,
			ModifiedAt: createdAt,
			LastNotes:  lastNotes,
		})
	} else if isRSS {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		err = RSSTemplate.Render(w, &RSSPage{
			Host:       s.Domain,
			ModifiedAt: createdAt,
			Metadata:   profile,
			LastNotes:  lastNotes,
		})
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
					if c == nostrudel {
						s = strings.Replace(s, "/n/", "/u/", 1)
					}
					if c == primalWeb {
						s = strings.Replace(
							strings.Replace(s, "/e/", "/p/", 1),
							nprofile, profile.Npub(), 1)
					}
					return s
				},
			),
		}

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
