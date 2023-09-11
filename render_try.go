package main

import (
	"net/http"
)

func renderTry(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}
	nip19entity := r.FormValue("nip19entity")
	http.Redirect(w, r, "/"+nip19entity, http.StatusFound)
}
