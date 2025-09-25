package main

import (
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Groups Service is running")
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Groups service starting on port 8003...")
	log.Fatal(http.ListenAndServe(":8003", nil))
}
