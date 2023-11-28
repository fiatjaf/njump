package main

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
)

func renderEmbedded(w http.ResponseWriter, r *http.Request, code string) {
	fmt.Println(r.URL.Path, "@.", r.Header.Get("user-agent"))

	data, err := grabData(r.Context(), code, false)
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

	var subject string
	for _, tag := range data.event.Tags {
		if tag[0] == "subject" || tag[0] == "title" {
			subject = tag[1]
		}
	}

	if data.event.Kind == 30023 || data.event.Kind == 30024 {
		data.content = mdToHTML(data.content, data.templateId == TelegramInstantView)
	} else {
		// first we run basicFormatting, which turns URLs into their appropriate HTML tags
		data.content = basicFormatting(html.EscapeString(data.content), true, false)
		// then we render quotes as HTML, which will also apply basicFormatting to all the internal quotes
		data.content = renderQuotesAsHTML(r.Context(), data.content, data.templateId == TelegramInstantView)
		// we must do this because inside <blockquotes> we must treat <img>s differently when telegram_instant_view
	}

	switch data.templateId {
	case Note:
		err = EmbeddedNoteTemplate.Render(w, &EmbeddedNotePage{
			Content:   template.HTML(data.content),
			CreatedAt: data.createdAt,
			Metadata:  data.metadata,
			Npub:      data.npub,
			NpubShort: data.npubShort,
			Subject:   subject,
			Url:       code,
		})

	case Profile:
		err = EmbeddedProfileTemplate.Render(w, &EmbeddedProfilePage{
			Metadata:                   data.metadata,
			NormalizedAuthorWebsiteURL: normalizeWebsiteURL(data.metadata.Website),
			RenderedAuthorAboutText:    template.HTML(basicFormatting(html.EscapeString(data.metadata.About), false, false, true)),
			Npub:                       data.npub,
			Nprofile:                   data.nprofile,
			AuthorRelays:               data.authorRelays,
		})
	default:
		log.Error().Int("templateId", int(data.templateId)).Msg("no way to render")
		http.Error(w, "tried to render an unsupported template at render_event.go", 500)
	}

	if err != nil {
		log.Error().Err(err).Msg("error rendering tmpl")
	}
	return
}
