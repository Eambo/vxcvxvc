package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/user/prr/database"
	"github.com/user/prr/models"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
)

// CreateServiceRequest defines the expected structure for creating/finding a service
// via the POST /services endpoint.
type CreateServiceRequest struct {
	Name string `json:"name"`
}

// ServicesRouterHandler handles all HTTP requests to the /services endpoint.
// It routes incoming requests based on the HTTP method:
//   POST: Calls findOrCreateServiceHandler. Expects a CreateServiceRequest JSON
//         in the body ({"name": "service_name"}). It finds an existing service
//         by name or creates a new one if not found.
//   GET: Calls listServicesHandler to retrieve a list of all services.
func ServicesRouterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost: // Using POST for find-or-create semantics
		findOrCreateServiceHandler(w, r)
	case http.MethodGet:
		listServicesHandler(w, r)
	default:
		http.Error(w, "Method not allowed for services", http.StatusMethodNotAllowed)
	}
}

// findOrCreateServiceHandler handles finding an existing service by name or creating a new one.
func findOrCreateServiceHandler(w http.ResponseWriter, r *http.Request) {
	var reqPayload CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(reqPayload.Name) == "" {
		http.Error(w, "Service name cannot be empty", http.StatusBadRequest)
		return
	}

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	// 1. Try to find the service by name (assuming 'name' is a keyword or properly analyzed field for exact match)
	// For exact match, it's common to use a term query on a .keyword field if using default ES mapping.
	// Or, ensure your 'name' field mapping supports exact matches as needed.
	query := fmt.Sprintf(`{
		"query": {
			"term": {
				"name.keyword": "%s"
			}
		}
	}`, reqPayload.Name) // Use name.keyword for exact match if standard analyzer is used for name

	searchReq := esapi.SearchRequest{
		Index: []string{"services"},
		Body:  strings.NewReader(query),
	}

	res, err := searchReq.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error searching for service '%s': %v", reqPayload.Name, err)
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		// This could be an actual error or just index not found, etc.
		log.Printf("Elasticsearch search error for service '%s': %s", reqPayload.Name, res.String())
		// Distinguish between "index not found" (ok to create) vs other errors
		// For simplicity here, we'll proceed to create if hits are empty,
		// but a robust implementation might check res.StatusCode or error type.
	}

	var searchResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		log.Printf("Error decoding search result for service '%s': %v", reqPayload.Name, err)
		http.Error(w, "Failed to parse database response", http.StatusInternalServerError)
		return
	}

	hits, _ := searchResult["hits"].(map[string]interface{})
	totalHits, _ := hits["total"].(map[string]interface{})
	totalValue, _ := totalHits["value"].(float64) // ES 7.x+ returns total as an object

	if totalValue > 0 {
		// Service found
		actualHits, _ := hits["hits"].([]interface{})
		if len(actualHits) > 0 {
			firstHit, _ := actualHits[0].(map[string]interface{})
			source, _ := firstHit["_source"].(map[string]interface{})

			var service models.Service
			sourceBytes, _ := json.Marshal(source)
			if err := json.Unmarshal(sourceBytes, &service); err != nil {
				log.Printf("Error unmarshalling found service '%s': %v", reqPayload.Name, err)
				http.Error(w, "Failed to parse found service data", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(service)
			return
		}
	}

	// 2. Service not found, so create it
	service := models.Service{
		ID:   uuid.New().String(),
		Name: reqPayload.Name,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(service); err != nil {
		log.Printf("Error encoding new service '%s' for Elasticsearch: %v", service.Name, err)
		http.Error(w, "Failed to process service data", http.StatusInternalServerError)
		return
	}

	indexReq := esapi.IndexRequest{
		Index:      "services",
		DocumentID: service.ID,
		Body:       &buf,
		Refresh:    "true",
	}

	idxRes, err := indexReq.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error indexing new service '%s': %v", service.Name, err)
		http.Error(w, "Failed to save new service", http.StatusInternalServerError)
		return
	}
	defer idxRes.Body.Close()

	if idxRes.IsError() {
		log.Printf("Elasticsearch indexing error for new service '%s': %s", service.Name, idxRes.String())
		http.Error(w, "Failed to save new service due to database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(service)
}

// listServicesHandler handles listing all services.
func listServicesHandler(w http.ResponseWriter, r *http.Request) {
	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	req := esapi.SearchRequest{
		Index: []string{"services"},
		Query: `{ "query": { "match_all": {} } }`,
		Size:  esapi.IntPtr(1000), // Adjust as needed
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error searching services in Elasticsearch: %v", err)
		http.Error(w, "Failed to retrieve services", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("Elasticsearch search error for services: %s", res.String())
		http.Error(w, "Failed to retrieve services due to database error", http.StatusInternalServerError)
		return
	}

	var rmap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&rmap); err != nil {
		log.Printf("Error decoding Elasticsearch response for services: %v", err)
		http.Error(w, "Failed to parse services data", http.StatusInternalServerError)
		return
	}

	var services []models.Service
	if hits, ok := rmap["hits"].(map[string]interface{}); ok {
		if actualHits, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range actualHits {
				if source, ok := hit.(map[string]interface{})["_source"]; ok {
					var s models.Service
					sourceBytes, err := json.Marshal(source)
					if err != nil {
						log.Printf("Error marshalling service hit source: %v", err)
						continue
					}
					if err := json.Unmarshal(sourceBytes, &s); err != nil {
						log.Printf("Error unmarshalling service from hit source: %v", err)
						continue
					}
					services = append(services, s)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(services); err != nil {
		log.Printf("Error encoding response for services: %v", err)
	}
}
