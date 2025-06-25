package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fiatjaf/njump/i18n"
)

func t(ctx context.Context, id string) string {
	return i18n.Translate(ctx, id, nil)
}

func tWithData(ctx context.Context, id string, data map[string]any) string {
	return i18n.Translate(ctx, id, data)
}

func isRTL(ctx context.Context) bool {
	if val := ctx.Value("isRTL"); val != nil {
		return val.(bool)
	}
	return false
}

func typewriterScript(ctx context.Context) string {
	// Build array of translated strings for typewriter animation
	keys := []string{
		"home.hero.protocol",
		"home.typewriter.not_crypto",
		"home.typewriter.simple",
		"home.typewriter.not_blockchain",
		"home.typewriter.universal_identity",
		"home.typewriter.not_company",
		"home.typewriter.free_expression",
		"home.typewriter.free_association",
		"home.typewriter.for_friends",
		"home.typewriter.for_everyone",
		"home.typewriter.for_broadcasting",
		"home.typewriter.for_groups",
		"home.typewriter.for_opinions",
		"home.typewriter.like_internet",
		"home.typewriter.flexible",
		"home.typewriter.scalable",
		"home.typewriter.public_square",
		"home.typewriter.client_relay",
		"home.typewriter.not_p2p",
		"home.typewriter.old_web",
		"home.typewriter.decentralized",
		"home.typewriter.communities",
		"home.typewriter.open_alternative",
		"home.typewriter.wikis",
		"home.typewriter.git",
		"home.typewriter.articles",
		"home.typewriter.microblogging",
		"home.typewriter.livestreaming",
		"home.typewriter.forums",
		"home.typewriter.annotating",
		"home.typewriter.commenting",
		"home.typewriter.social_web",
		"home.typewriter.for_you",
		"home.typewriter.dots",
	}
	
	strings := []string{""}
	for _, key := range keys {
		strings = append(strings, t(ctx, key))
	}
	
	stringsJSON, _ := json.Marshal(strings)
	
	return fmt.Sprintf(`<script>
var tw = document.getElementById('tw')
new Typewriter(tw, {
  strings: %s,
  autoStart: true,
  loop: true,
  cursorClassName: 'typewriter-cursor',
  pauseFor: 3000
})
</script>`, stringsJSON)
}
