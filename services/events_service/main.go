package main

import (
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Events Service is running")
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Events service starting on port 8002...")
	log.Fatal(http.ListenAndServe(":8002", nil))
}
