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
	w.Header().Set("Content-Type", "text/html")

	isSitemap := false
	if strings.HasSuffix(code, ".xml") {
		isSitemap = true
		code = code[:len(code)-4]
	}

	data, err := grabData(r.Context(), code, isSitemap)

	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
	} else if len(data.renderableLastNotes) != 0 {
		w.Header().Set("Cache-Control", "max-age=3600")
	}

	if err != nil {
		errorPage := &ErrorPage{
			Errors: err.Error(),
		}
		errorPage.TemplateText()
		w.WriteHeader(http.StatusNotFound)
		ErrorTemplate.Render(w, errorPage)
	} else if !isSitemap {
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
				Clients: generateClientList(getPreviewStyle(r), code, data.event),
			},

			Metadata:                   data.metadata,
			NormalizedAuthorWebsiteURL: normalizeWebsiteURL(data.metadata.Website),
			RenderedAuthorAboutText:    template.HTML(basicFormatting(html.EscapeString(data.metadata.About), false, false)),
			Npub:                       data.npub,
			Nprofile:                   data.nprofile,
			AuthorRelays:               data.authorRelays,
			LastNotes:                  data.renderableLastNotes,
		})
	} else {
		w.Header().Add("content-type", "text/xml")
		w.Write([]byte(XML_HEADER))
		SitemapTemplate.Render(w, &SitemapPage{
			Host:       s.Domain,
			ModifiedAt: data.modifiedAt,
			Npub:       data.npub,
			LastNotes:  data.renderableLastNotes,
		})
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
