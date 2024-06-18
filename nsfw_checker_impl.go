//go:build nsfw

package main

import (
	"fmt"
	"image/png"
	"os"
	"sync"

	"github.com/ccuetoh/nsfw"
	lru "github.com/hashicorp/golang-lru/v2"
)

var nsfwCache, _ = lru.New[string, bool](64)

var nsfwPredictor = func() *nsfw.Predictor {
	p, err := nsfw.NewLatestPredictor()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start keras nsfw detector")
		return p
	}
	log.Info().Msg("keras nsfw detector enabled")
	return p
}()

var tempFileLocks = [3]sync.Mutex{{}, {}, {}}

func isImageNSFW(url string) bool {
	if is, ok := nsfwCache.Get(url); ok {
		return is
	}

	img, err := fetchImageFromURL(url)
	if err != nil {
		return false // if we can't read it that means it's ok
	}

	// grab mutex
	var tempPath string
	for i, mu := range tempFileLocks {
		if ok := mu.TryLock(); ok {
			tempPath = fmt.Sprintf("/tmp/nsfw-detection-%d.png", i)
			defer mu.Unlock()
			break
		}
	}

	if tempPath == "" {
		// apparently we can't allocate a temporary file for this, so let's warn and return false
		log.Warn().Msg("failed to allocate a temp file for nsfw detection")
		return false
	}

	tempFile, err := os.Create(tempPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to open a temp file for nsfw detection")
		return false
	}
	if err := png.Encode(tempFile, img); err != nil {
		log.Warn().Err(err).Msg("failed to encode png for nsfw detection")
		tempFile.Close()
		return false
	}
	tempFile.Close() // close here so the thing can read it below

	res := nsfwPredictor.Predict(nsfwPredictor.NewImage(tempPath, 3))
	log.Debug().Str("url", url).Str("desc", res.Describe()).Msg("image analyzed")

	is := res.Porn > 0.85 || res.Hentai > 0.85
	nsfwCache.Add(url, is)
	return is
}
