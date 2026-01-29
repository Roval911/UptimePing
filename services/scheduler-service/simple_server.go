package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	fmt.Println("Starting minimal test server...")
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	})
	
	server := &http.Server{
		Addr: ":50052",
	}
	
	fmt.Println("Server listening on :50052")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
