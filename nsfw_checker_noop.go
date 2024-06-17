//go:build !nsfw

package main

func isImageNSFW(url string) bool {
	return false
}
