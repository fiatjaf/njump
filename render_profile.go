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
		errorPage := &ErrorPage{
			Errors: err.Error(),
		}
		errorPage.TemplateText()
		w.WriteHeader(http.StatusNotFound)
		ErrorTemplate.Render(w, errorPage)
		return
	}

	if len(data.renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	}

	if isSitemap {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		SitemapTemplate.Render(w, &SitemapPage{
			Host:       s.Domain,
			ModifiedAt: data.modifiedAt,
			Npub:       data.npub,
			LastNotes:  data.renderableLastNotes,
		})
	} else if isRSS {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		RSSTemplate.Render(w, &RSSPage{
			Host:       s.Domain,
			ModifiedAt: data.modifiedAt,
			Npub:       data.npub,
			Metadata:   data.metadata,
			LastNotes:  data.renderableLastNotes,
		})
	} else {
		w.Header().Add("content-type", "text/html")
		err = ProfileTemplate.Render(w, &ProfilePage{
			HeadCommonPartial: HeadCommonPartial{IsProfile: true, TailwindDebugStuff: tailwindDebugStuff},
			DetailsPartial: DetailsPartial{
				HideDetails:     true,
				CreatedAt:       data.createdAt,
				KindDescription: data.kindDescription,
				KindNIP:         data.kindNIP,
				EventJSON:       eventToHTML(data.event),
				Kind:            data.event.Kind,
			},
			ClientsPartial: ClientsPartial{
				Clients: generateClientList(data.nprofile, data.event),
			},

			Metadata:                   data.metadata,
			NormalizedAuthorWebsiteURL: normalizeWebsiteURL(data.metadata.Website),
			RenderedAuthorAboutText:    template.HTML(basicFormatting(html.EscapeString(data.metadata.About), false, false, false)),
			Npub:                       data.npub,
			Nprofile:                   data.nprofile,
			AuthorRelays:               data.authorRelays,
			LastNotes:                  data.renderableLastNotes,
		})
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
