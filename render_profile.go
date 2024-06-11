package main

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strings"
)

func renderProfile(w http.ResponseWriter, r *http.Request, code string) {
	fmt.Println(r.URL.Path, "@.", r.Header.Get("user-agent"))

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

	data, err := grabData(r.Context(), code, isSitemap)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: err.Error()}).Render(r.Context(), w)
		return
	}

	if isSitemap {
		w.Header().Add("content-type", "text/xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write([]byte(XML_HEADER))
		err = SitemapTemplate.Render(w, &SitemapPage{
			Host:       s.Domain,
			ModifiedAt: data.modifiedAt,
			LastNotes:  data.renderableLastNotes,
		})
	} else if isRSS {
		w.Header().Add("content-type", "text/xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write([]byte(XML_HEADER))
		err = RSSTemplate.Render(w, &RSSPage{
			Host:       s.Domain,
			ModifiedAt: data.modifiedAt,
			Metadata:   data.metadata,
			LastNotes:  data.renderableLastNotes,
		})
	} else {
		w.Header().Add("content-type", "text/html")
		w.Header().Set("Cache-Control", "max-age=86400")
		err = profileTemplate(ProfilePageParams{
			HeadParams: HeadParams{IsProfile: true},
			Details: DetailsParams{
				HideDetails:     true,
				CreatedAt:       data.createdAt,
				KindDescription: data.kindDescription,
				KindNIP:         data.kindNIP,
				EventJSON:       data.event.ToJSONHTML(),
				Kind:            data.event.Kind,
				Metadata:        data.metadata,
			},
			Metadata:                   data.metadata,
			NormalizedAuthorWebsiteURL: normalizeWebsiteURL(data.metadata.Website),
			RenderedAuthorAboutText:    template.HTML(basicFormatting(html.EscapeString(data.metadata.About), false, false, false)),
			Nprofile:                   data.nprofile,
			AuthorRelays:               data.authorRelaysPretty,
			LastNotes:                  data.renderableLastNotes,
			Clients: generateClientList(data.event.Kind, data.nprofile,
				func(c ClientReference, s string) string {
					if c == nostrudel {
						s = strings.Replace(s, "/n/", "/u/", 1)
					}
					if c == primalWeb {
						s = strings.Replace(
							strings.Replace(s, "/e/", "/p/", 1),
							data.nprofile, data.metadata.Npub(), 1)
					}
					return s
				},
			),
		}).Render(r.Context(), w)
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
