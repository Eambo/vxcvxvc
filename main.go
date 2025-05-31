// Package main implements the entrypoint for the PRR (Product Readiness Review) backend server.
// This server provides API endpoints for managing PRR questions, sections, services,
// submitting PRR reviews, and comparing PRR versions.
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
	http.HandleFunc("/prr", handlers.PRRRouterHandler)                 // Handles /prr
	http.HandleFunc("/prr/history", handlers.PRRRouterHandler)         // Explicitly handles /prr/history
	http.HandleFunc("/prr/compare", handlers.PRRRouterHandler)         // Explicitly handles /prr/compare

	// Search route
	http.HandleFunc("/search/services", handlers.SearchServicesHandler)

	// main starts the HTTP server and listens for requests.
	// It registers all application handlers and logs server start and potential fatal errors.
	// To run the server, execute `go run main.go`.
	port := ":8080"
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
