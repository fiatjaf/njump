package main

import (
	"fmt"
	"image/png"
	"net/http"
)

func generate(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path, ":~", r.Header.Get("user-agent"))

	code := r.URL.Path[1+len("image/"):]
	if code == "" {
		fmt.Fprintf(w, "call /image/<nip19 code>")
		return
	}

	event, err := getEvent(r.Context(), code)
	if err != nil {
		http.Error(w, "error fetching event: "+err.Error(), 404)
		return
	}

	lines := normalizeText(event.Content)

	img, err := drawImage(lines, getPreviewStyle(r))
	if err != nil {
		log.Printf("error writing image: %s", err)
		http.Error(w, "error writing image!", 500)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=604800")

	if err := png.Encode(w, img); err != nil {
		log.Printf("error encoding image: %s", err)
		http.Error(w, "error encoding image!", 500)
		return
	}
}
