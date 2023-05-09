package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/image/", generate)
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
