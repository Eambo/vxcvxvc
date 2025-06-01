package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/user/prr/database"
	"github.com/user/prr/models"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	// "strings" // Removed as it's not used when Query is a raw string.
	"github.com/google/uuid"
)

// SectionsRouterHandler handles all HTTP requests to the /admin/sections endpoint.
// It routes incoming requests based on the HTTP method:
//   POST: Calls createSectionHandler to create a new section.
//         Expects a models.Section JSON object (Name, Description) in the request body. ID is generated.
//   GET: Calls listSectionsHandler to retrieve a list of all sections.
// Future methods like PUT (update) and DELETE can be added here.
func SectionsRouterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		createSectionHandler(w, r)
	case http.MethodGet:
		listSectionsHandler(w, r)
	// Add PUT and DELETE cases here in future tasks
	default:
		http.Error(w, "Method not allowed for sections", http.StatusMethodNotAllowed)
	}
}

// createSectionHandler handles the creation of new sections (POST).
// Renamed from CreateSectionHandler
func createSectionHandler(w http.ResponseWriter, r *http.Request) {
	// Existing POST logic from CreateSectionHandler

	var section models.Section
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&section); err != nil {
		log.Printf("Error decoding request body for section: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate required fields
	if section.Name == "" {
		http.Error(w, "Section name is required", http.StatusBadRequest)
		return
	}
	// Description is optional, so no validation needed for it unless specified

	// Generate a new UUID for the section
	section.ID = uuid.New().String()

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Index the section document
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(section); err != nil {
		log.Printf("Error encoding section for Elasticsearch: %v", err)
		http.Error(w, "Failed to process section data", http.StatusInternalServerError)
		return
	}

	req := esapi.IndexRequest{
		Index:      "sections",
		DocumentID: section.ID,
		Body:       &buf,
		Refresh:    "true", // Refresh after indexing
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error indexing section in Elasticsearch: %v", err)
		http.Error(w, "Failed to save section", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("Elasticsearch indexing error for section: %s", res.String())
		http.Error(w, "Failed to save section due to database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(section); err != nil {
		log.Printf("Error encoding response for section: %v", err)
	}
}

// listSectionsHandler handles listing all sections (GET).
func listSectionsHandler(w http.ResponseWriter, r *http.Request) {
	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	req := esapi.SearchRequest{
		Index: []string{"sections"},
		Query: `{ "query": { "match_all": {} } }`,
		Size:  esapi.IntPtr(1000), // Adjust size as needed
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error searching sections in Elasticsearch: %v", err)
		http.Error(w, "Failed to retrieve sections", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("Elasticsearch search error for sections: %s", res.String())
		http.Error(w, "Failed to retrieve sections due to database error", http.StatusInternalServerError)
		return
	}

	var rmap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&rmap); err != nil {
		log.Printf("Error decoding Elasticsearch response for sections: %v", err)
		http.Error(w, "Failed to parse sections data", http.StatusInternalServerError)
		return
	}

	var sections []models.Section
	if hits, ok := rmap["hits"].(map[string]interface{}); ok {
		if actualHits, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range actualHits {
				if source, ok := hit.(map[string]interface{})["_source"]; ok {
					var s models.Section
					sourceBytes, err := json.Marshal(source)
					if err != nil {
						log.Printf("Error marshalling section hit source: %v", err)
						continue
					}
					if err := json.Unmarshal(sourceBytes, &s); err != nil {
						log.Printf("Error unmarshalling section from hit source: %v", err)
						continue
					}
					sections = append(sections, s)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(sections); err != nil {
		log.Printf("Error encoding response for sections: %v", err)
	}
}
