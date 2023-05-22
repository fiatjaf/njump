package main

import (
	"log"
	"net/http"
	"os"
)

func main() {

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/image/", generate)
	http.HandleFunc("/proxy/", proxy)
	http.HandleFunc("/", render)

	port := os.Getenv("PORT")
	if port == "" {
		port = "2999"
	}

	log.Print("listening at http://0.0.0.0:" + port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Fatal(err)
	}
}
