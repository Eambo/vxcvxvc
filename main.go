package main

import (
	"log"
	"net/http"

	"github.com/user/prr/api/handlers"
)

func main() {
	// Register handler functions
	http.HandleFunc("/admin/questions", handlers.QuestionsRouterHandler)
	http.HandleFunc("/admin/sections", handlers.SectionsRouterHandler) // Updated to use SectionsRouterHandler
	http.HandleFunc("/services", handlers.ServicesRouterHandler)       // New route for services

	// Start the server
	port := ":8080"
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
