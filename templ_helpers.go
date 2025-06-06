package main

import (
	"context"

	"github.com/fiatjaf/njump/i18n"
)

func t(ctx context.Context, id string) string {
	return i18n.Translate(ctx, id, nil)
}
