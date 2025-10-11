package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Definiujemy, że dla ścieżki "/api/health" ma się wykonać funkcja healthCheckHandler
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "API is healthy!")
	})

	fmt.Println("Starting server on port 8080...")
	// Uruchamiamy serwer na porcie 8080 i nasłuchujemy na żądania
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
