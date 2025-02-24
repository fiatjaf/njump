package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/dgraph-io/ristretto"
)

var mediaAlertCache, _ = ristretto.NewCache(&ristretto.Config[string, bool]{
	NumCounters: 1e6,     // number of keys to track frequency of (1M)
	MaxCost:     1 << 24, // maximum cost of cache (64MB)
	BufferItems: 64,      // number of keys per Get buffer
})

type mediaAlertResponse struct {
	Message string  `json:"message"`
	Score   float64 `json:"score"`
}

// isExplicitContent checks if the provided URL contains explicit content
// it returns true if the content is explicit, false otherwise
// the function handles caching and retries for timeout errors
func isExplicitContent(ctx context.Context, mediaURL string) (bool, error) {
	// check cache first
	if val, found := mediaAlertCache.Get(mediaURL); found {
		return val, nil
	}

	// make the API request
	isExplicit, err := checkMediaAlert(ctx, mediaURL, false)
	if err != nil {
		return false, err
	}

	// store result in cache
	mediaAlertCache.SetWithTTL(mediaURL, isExplicit, 1, 24*time.Hour)

	return isExplicit, nil
}

// checkMediaAlert makes the actual API request to the Media Alert service
// if retry is true, this is a retry attempt after a timeout
func checkMediaAlert(ctx context.Context, mediaURL string, retry bool) (bool, error) {
	if s.MediaAlertAPIKey == "" {
		return false, nil // skip check if no API key is configured
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://nostr-media-alert.com/score?"+url.Values{
		"key": {s.MediaAlertAPIKey},
		"url": {mediaURL},
	}.Encode(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("got unexpected response %d: %s", resp.StatusCode, string(msg))
	}

	var result mediaAlertResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	// handle different response types
	switch result.Message {
	case "SUCCESS":
		return result.Score >= 0.90, nil
	case "TIMEOUT":
		if retry {
			// if this is already a retry, don't retry again
			return false, nil
		}

		// handle timeout by retrying after delay
		go func() {
			// create a new context with timeout for the retry
			retryCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// wait before retrying
			time.Sleep(20 * time.Second)

			// retry the request
			isExplicit, err := checkMediaAlert(retryCtx, mediaURL, true)
			if err == nil {
				// update cache with the result (the expensive stuff we store for longer )
				mediaAlertCache.SetWithTTL(mediaURL, isExplicit, 1, time.Hour*72)
			}
		}()

		return false, nil
	case "RATE LIMITED":
		log.Warn().Str("url", mediaURL).Msg("media alert API rate limited")
		return false, nil
	case "INVALID MEDIA":
		log.Debug().Str("url", mediaURL).Msg("invalid media for content check")
		return false, nil
	default:
		return false, fmt.Errorf("unknown response message: %s", result.Message)
	}
}
