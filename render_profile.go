package main

import (
	"context"
	"html"
	"html/template"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func renderProfile(ctx context.Context, w http.ResponseWriter, code string) {
	ctx, span := tracer.Start(ctx, "render-profile", trace.WithAttributes(attribute.String("code", code)))
	defer span.End()

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
	if err != nil || profile.Event == nil {
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusNotFound)

		errMsg := "profile metadata not found"
		if err != nil {
			errMsg = err.Error()
		}
		errorTemplate(ErrorPageParams{Errors: errMsg}).Render(ctx, w)
		return
	}

	createdAt := profile.Event.CreatedAt.Time().Format("2006-01-02T15:04:05Z07:00")
	modifiedAt := profile.Event.CreatedAt.Time().Format("2006-01-02T15:04:05Z07:00")

	lastNotes := authorLastNotes(ctx, profile.PubKey, isSitemap)

	if isSitemap {
		w.Header().Add("content-type", "text/xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write([]byte(XML_HEADER))
		err = SitemapTemplate.Render(w, &SitemapPage{
			Host:       s.Domain,
			ModifiedAt: modifiedAt,
			LastNotes:  lastNotes,
		})
	} else if isRSS {
		w.Header().Add("content-type", "text/xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write([]byte(XML_HEADER))
		err = RSSTemplate.Render(w, &RSSPage{
			Host:       s.Domain,
			ModifiedAt: modifiedAt,
			Metadata:   profile,
			LastNotes:  lastNotes,
		})
	} else {
		w.Header().Add("content-type", "text/html")
		w.Header().Set("Cache-Control", "max-age=86400")

		nprofile := profile.Nprofile(ctx, sys, 2)

		err = profileTemplate(ProfilePageParams{
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
		}).Render(ctx, w)
	}

	if err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
	return
}
