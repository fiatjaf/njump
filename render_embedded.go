package main

import (
	"html"
	"html/template"
	"net/http"

	"github.com/a-h/templ"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func renderEmbedjs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	fileContent, _ := static.ReadFile("static/embed.js")
	w.Write(fileContent)
}

func renderEmbedded(w http.ResponseWriter, r *http.Request, code string) {
	ctx := r.Context()

	ctx, span := tracer.Start(ctx, "render-embedded", trace.WithAttributes(attribute.String("code", code)))
	defer span.End()

	data, err := grabData(ctx, code)
	if err != nil {
		w.Header().Set("Cache-Control", "max-age=60")
		w.WriteHeader(http.StatusNotFound)
		errorTemplate(ErrorPageParams{Errors: err.Error()}).Render(ctx, w)
		return
	}

	var subject string
	for _, tag := range data.event.Tags {
		if tag[0] == "subject" || tag[0] == "title" {
			subject = tag[1]
		}
	}

	if data.event.Kind == 30023 || data.event.Kind == 30024 {
		data.content = mdToHTML(data.content, data.templateId == TelegramInstantView, true)
	} else {
		// first we run basicFormatting, which turns URLs into their appropriate HTML tags
		data.content = basicFormatting(html.EscapeString(data.content), true, false, false)
		// then we render quotes as HTML, which will also apply basicFormatting to all the internal quotes
		data.content = renderQuotesAsHTML(ctx, data.content, data.templateId == TelegramInstantView)
		// we must do this because inside <blockquotes> we must treat <img>s differently when telegram_instant_view
	}

	var component templ.Component
	switch data.templateId {
	case Note:
		component = embeddedNoteTemplate(EmbeddedNoteParams{
			Content:   template.HTML(data.content),
			CreatedAt: data.createdAt,
			Metadata:  data.event.author,
			Subject:   subject,
			Url:       code,
		})

	case Profile:
		component = embeddedProfileTemplate(EmbeddedProfileParams{
			Metadata:                   data.event.author,
			NormalizedAuthorWebsiteURL: normalizeWebsiteURL(data.event.author.Website),
			RenderedAuthorAboutText:    template.HTML(basicFormatting(html.EscapeString(data.event.author.About), false, false, true)),
			AuthorRelays:               relaysPretty(ctx, data.event.author.PubKey),
		})
	default:
		log.Error().Int("templateId", int(data.templateId)).Msg("no way to render")
		http.Error(w, "tried to render an unsupported template at render_event.go", 500)
		return
	}

	if err := component.Render(ctx, w); err != nil {
		log.Warn().Err(err).Msg("error rendering tmpl")
	}
	return
}
