package i18n

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml"
	"golang.org/x/text/language"
)

// languageKey is used to store the preferred language in context
// via context.WithValue.
type languageKey struct{}

var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	// load translation files from locales directory
	filepath.WalkDir("locales", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext == ".toml" || ext == ".json" {
			bundle.LoadMessageFile(path)
		}
		return nil
	})
}

// WithLanguage returns a copy of ctx that stores lang.
func WithLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, languageKey{}, lang)
}

// LanguageFromContext extracts the stored language from ctx.
// If no language was set it returns an empty string.
func LanguageFromContext(ctx context.Context) string {
	lang, _ := ctx.Value(languageKey{}).(string)
	return lang
}

// Translate returns the localized string for id using the language stored in ctx.
// If translation is missing, it falls back to English and finally to the id itself.
func Translate(ctx context.Context, id string, data map[string]any) string {
	lang, _ := ctx.Value(languageKey{}).(string)
	// always include English so that it is used as a fallback
	localizer := i18n.NewLocalizer(bundle, lang, "en")
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id, TemplateData: data})
	if err != nil {
		var nfe *i18n.MessageNotFoundErr
		if errors.As(err, &nfe) {
			// fall back to the message retrieved from the default language
			return msg
		}
		return id
	}
	return msg
}
