package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type mediaAlertResponse struct {
	Message string  `json:"message"`
	Score   float64 `json:"score"`
}

// isExplicitContent checks if the provided URL contains explicit content
func isExplicitContent(ctx context.Context, mediaURL string) (bool, error) {
	return checkMediaAlert(ctx, mediaURL)
}

func checkMediaAlert(ctx context.Context, mediaURL string) (bool, error) {
	if s.MediaAlertAPIKey == "" {
		return false, nil // skip check if no API key is configured
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*8)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://nostr-media-alert.com/score?"+url.Values{
		"key": {s.MediaAlertAPIKey},
		"url": {mediaURL},
	}.Encode(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

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
