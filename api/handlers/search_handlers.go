package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/user/prr/database"
	"github.com/user/prr/models"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	// "github.com/elastic/go-elasticsearch/v8/esutil" // Not strictly needed for string/map queries
)

// SearchServicesHandler handles requests to GET /search/services.
// It searches for services based on a query parameter 'q' and augments the results
// with information about the latest PRR submission for each service.
// Query Parameter:
//   q (string, required): The search term to query against service names and descriptions.
// Response:
//   A JSON array of models.ServiceSearchResult, where each element contains service
//   details and, if available, the section scores and timestamp of its most recent PRR.
func SearchServicesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParam := strings.TrimSpace(r.URL.Query().Get("q"))
	if queryParam == "" {
		http.Error(w, "Query parameter 'q' cannot be empty", http.StatusBadRequest)
		return
	}

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// 1. Search for services
	var servicesQuery map[string]interface{}
	servicesQuery = map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  queryParam,
				"fields": []string{"name", "description"}, // Assuming 'name' and 'description' fields in 'services' index
				"type":   "best_fields",
			},
		},
		"size": 20, // Limit service search results
	}

	var serviceBuf bytes.Buffer
	if err := json.NewEncoder(&serviceBuf).Encode(servicesQuery); err != nil {
		log.Printf("Error encoding service search query: %v", err)
		http.Error(w, "Failed to build service search query", http.StatusInternalServerError)
		return
	}

	serviceSearchReq := esapi.SearchRequest{
		Index: []string{"services"},
		Body:  &serviceBuf,
	}

	serviceRes, err := serviceSearchReq.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error searching services with query '%s': %v", queryParam, err)
		http.Error(w, "Service search failed", http.StatusInternalServerError)
		return
	}
	defer serviceRes.Body.Close()

	if serviceRes.IsError() {
		log.Printf("Elasticsearch service search error (query '%s'): %s", queryParam, serviceRes.String())
		http.Error(w, "Service search database error", http.StatusInternalServerError)
		return
	}

	var serviceRmap map[string]interface{}
	if err := json.NewDecoder(serviceRes.Body).Decode(&serviceRmap); err != nil {
		log.Printf("Error decoding service search results (query '%s'): %v", queryParam, err)
		http.Error(w, "Failed to parse service search results", http.StatusInternalServerError)
		return
	}

	var foundServices []models.Service
	if hits, ok := serviceRmap["hits"].(map[string]interface{}); ok {
		if actualHits, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range actualHits {
				if source, ok := hit.(map[string]interface{})["_source"]; ok {
					var s models.Service
					sourceBytes, _ := json.Marshal(source)
					if err := json.Unmarshal(sourceBytes, &s); err == nil && s.ID != "" {
						foundServices = append(foundServices, s)
					}
				}
			}
		}
	}

	// 2. For each found service, get its latest PRR submission
	var results []models.ServiceSearchResult
	for _, service := range foundServices {
		prrQuery := map[string]interface{}{
			"query": map[string]interface{}{
				"term": map[string]interface{}{
					"service_id.keyword": service.ID,
				},
			},
			"sort": []map[string]interface{}{
				{"timestamp": "desc"},
			},
			"size": 1,
		}
		var prrBuf bytes.Buffer
		if err := json.NewEncoder(&prrBuf).Encode(prrQuery); err != nil {
			log.Printf("Error encoding PRR search query for service %s: %v", service.ID, err)
			// Continue to next service, or append with no PRR info
			results = append(results, models.ServiceSearchResult{
				ServiceID:   service.ID,
				ServiceName: service.Name,
			})
			continue
		}

		prrSearchReq := esapi.SearchRequest{
			Index: []string{"prr_submissions"},
			Body:  &prrBuf,
		}
		prrRes, err := prrSearchReq.Do(context.Background(), esClient)
		if err != nil {
			log.Printf("Error searching latest PRR for service %s: %v", service.ID, err)
			results = append(results, models.ServiceSearchResult{
				ServiceID:   service.ID,
				ServiceName: service.Name,
			})
			continue
		}
		defer prrRes.Body.Close() // Important to close in loop

		searchResultItem := models.ServiceSearchResult{
			ServiceID:   service.ID,
			ServiceName: service.Name,
		}

		if !prrRes.IsError() {
			var prrRmap map[string]interface{}
			if err := json.NewDecoder(prrRes.Body).Decode(&prrRmap); err == nil {
				if hits, ok := prrRmap["hits"].(map[string]interface{}); ok {
					if actualHits, ok := hits["hits"].([]interface{}); ok && len(actualHits) > 0 {
						if source, ok := actualHits[0].(map[string]interface{})["_source"]; ok {
							var latestPRR models.PRRSubmission
							sourceBytes, _ := json.Marshal(source)
							if err := json.Unmarshal(sourceBytes, &latestPRR); err == nil {
								searchResultItem.LatestPRRScores = latestPRR.SectionScores
								searchResultItem.LastPRRTimestamp = &latestPRR.Timestamp
							}
						}
					}
				}
			} else {
                 log.Printf("Error decoding latest PRR for service %s: %v", service.ID, err)
            }
		} else {
            log.Printf("Elasticsearch error searching latest PRR for service %s: %s", service.ID, prrRes.String())
        }
		results = append(results, searchResultItem)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("Error encoding search services response: %v", err)
	}
}
