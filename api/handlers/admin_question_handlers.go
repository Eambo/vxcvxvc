package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/user/prr/database"
	"github.com/user/prr/models"

	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
)

// QuestionsRouterHandler handles routing for /admin/questions based on HTTP method.
func QuestionsRouterHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		createQuestionHandler(w, r)
	case http.MethodGet:
		listQuestionsHandler(w, r)
	case http.MethodPut:
		updateQuestionHandler(w, r)
	case http.MethodDelete:
		deleteQuestionHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// createQuestionHandler handles the creation of new questions (POST).
func createQuestionHandler(w http.ResponseWriter, r *http.Request) {
	// Existing POST logic from CreateQuestionHandler

	var question models.Question
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&question); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Generate a new UUID for the question
	question.ID = uuid.New().String()

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Index the question document
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(question); err != nil {
		log.Printf("Error encoding question for Elasticsearch: %v", err)
		http.Error(w, "Failed to process question data", http.StatusInternalServerError)
		return
	}

	req := esapi.IndexRequest{
		Index:      "questions",
		DocumentID: question.ID,
		Body:       &buf,
		Refresh:    "true", // Refresh after indexing to make the document immediately searchable
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error indexing question in Elasticsearch: %v", err)
		http.Error(w, "Failed to save question", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("Elasticsearch indexing error: %s", res.String())
		http.Error(w, "Failed to save question due to database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(question); err != nil {
		log.Printf("Error encoding response: %v", err)
		// If headers are already sent, we can't send another error code
		// but we should log this server-side issue.
	}
}

// listQuestionsHandler handles listing all questions (GET).
func listQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	// Existing GET logic from ListQuestionsHandler

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Perform a search request to retrieve all questions
	req := esapi.SearchRequest{
		Index: []string{"questions"},
		Query: `{ "query": { "match_all": {} } }`,
		Size:  esapi.IntPtr(1000), // Adjust size as needed
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error searching questions in Elasticsearch: %v", err)
		http.Error(w, "Failed to retrieve questions", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("Elasticsearch search error: %s", res.String())
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			log.Printf("Error parsing the Elasticsearch error response: %v", err)
			http.Error(w, "Failed to retrieve questions due to database error (and failed to parse error response)", http.StatusInternalServerError)
		} else {
			log.Printf("Elasticsearch error details: %v", e)
			http.Error(w, "Failed to retrieve questions due to database error", http.StatusInternalServerError)
		}
		return
	}

	var rmap map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&rmap); err != nil {
		log.Printf("Error decoding Elasticsearch response: %v", err)
		http.Error(w, "Failed to parse questions data", http.StatusInternalServerError)
		return
	}

	var questions []models.Question
	if hits, ok := rmap["hits"].(map[string]interface{}); ok {
		if actualHits, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range actualHits {
				if source, ok := hit.(map[string]interface{})["_source"]; ok {
					var q models.Question
					// Re-marshal and unmarshal to convert map[string]interface{} to struct
					// This is a common way but can be optimized if performance is critical
					sourceBytes, err := json.Marshal(source)
					if err != nil {
						log.Printf("Error marshalling hit source: %v", err)
						continue
					}
					if err := json.Unmarshal(sourceBytes, &q); err != nil {
						log.Printf("Error unmarshalling question from hit source: %v", err)
						continue
					}
					questions = append(questions, q)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(questions); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// deleteQuestionHandler handles deleting a question by ID (DELETE).
func deleteQuestionHandler(w http.ResponseWriter, r *http.Request) {
	questionID := r.URL.Query().Get("id")
	if questionID == "" {
		http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	req := esapi.DeleteRequest{
		Index:      "questions",
		DocumentID: questionID,
		Refresh:    "true", // Refresh after delete
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error deleting question in Elasticsearch: %v", err)
		http.Error(w, "Failed to delete question", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == http.StatusNotFound {
			// Consider "not found" as a successful deletion for idempotency,
			// or return http.StatusNotFound if you want to inform the client.
			// For this implementation, we'll treat it as "already deleted" -> success.
			w.WriteHeader(http.StatusNoContent)
		} else {
			log.Printf("Elasticsearch delete error: %s", res.String())
			http.Error(w, "Failed to delete question due to database error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateQuestionHandler handles updating existing questions (PUT).
func updateQuestionHandler(w http.ResponseWriter, r *http.Request) {
	var questionUpdates models.Question
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&questionUpdates); err != nil {
		log.Printf("Error decoding request body for update: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if questionUpdates.ID == "" {
		http.Error(w, "Question ID is required for updates", http.StatusBadRequest)
		return
	}

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	// Prepare the update script for Elasticsearch
	// This creates a map that will be marshalled into the JSON body for the update request
	updateFields := map[string]interface{}{
		"doc": questionUpdates, // Using "doc" will replace all fields provided in questionUpdates
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(updateFields); err != nil {
		log.Printf("Error encoding update for Elasticsearch: %v", err)
		http.Error(w, "Failed to process update data", http.StatusInternalServerError)
		return
	}

	req := esapi.UpdateRequest{
		Index:      "questions",
		DocumentID: questionUpdates.ID,
		Body:       &buf,
		Refresh:    "true", // Refresh after update to make changes immediately searchable
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error updating question in Elasticsearch: %v", err)
		http.Error(w, "Failed to update question", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == http.StatusNotFound {
			http.Error(w, "Question not found", http.StatusNotFound)
		} else {
			log.Printf("Elasticsearch update error: %s", res.String())
			http.Error(w, "Failed to update question due to database error", http.StatusInternalServerError)
		}
		return
	}

	// To return the updated document, we would ideally get it from the response
	// or perform a get request. For simplicity, we return the input `questionUpdates`
	// assuming the update was successful as indicated by Elasticsearch.
	// A more robust solution might fetch the document after update.

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(questionUpdates); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
