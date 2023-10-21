package main

import (
	"net/http"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func redirectFromFormSubmit(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "failed to parse form data", http.StatusBadRequest)
		return
	}
	nip19entity := r.FormValue("nip19entity")
	http.Redirect(w, r, "/"+nip19entity, http.StatusFound)
}

func redirectFromPSlash(w http.ResponseWriter, r *http.Request) {
	code, _ := nip19.EncodePublicKey(r.URL.Path[3:])
	http.Redirect(w, r, "/"+code, http.StatusFound)
	return
}

func redirectFromESlash(w http.ResponseWriter, r *http.Request) {
	code, _ := nip19.EncodeEvent(r.URL.Path[3:], []string{}, "")
	http.Redirect(w, r, "/"+code, http.StatusFound)
	return
}
