package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/user/prr/database"
	"github.com/user/prr/models"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
)

// SubmitPRRRequest defines the expected structure for a PRR submission.
type SubmitPRRRequest struct {
	ServiceID string          `json:"service_id"`
	UserID    string          `json:"user_id"`
	Answers   []models.Answer `json:"answers"`
}

// SubmitPRRHandler handles the submission of a Product Readiness Review.
func SubmitPRRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestPayload SubmitPRRRequest
	if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(requestPayload.ServiceID) == "" {
		http.Error(w, "Service ID cannot be empty", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(requestPayload.UserID) == "" {
		http.Error(w, "User ID cannot be empty", http.StatusBadRequest)
		return
	}
	if len(requestPayload.Answers) == 0 {
		http.Error(w, "Answers cannot be empty", http.StatusBadRequest)
		return
	}

	esClient, err := database.GetESClient()
	if err != nil {
		log.Printf("Error getting Elasticsearch client: %v", err)
		http.Error(w, "Failed to connect to database", http.StatusInternalServerError)
		return
	}

	// 1. Fetch all questions from the "questions" index
	questionsMap := make(map[string]models.Question)
	searchReq := esapi.SearchRequest{
		Index: []string{"questions"},
		Query: `{ "query": { "match_all": {} } }`,
		Size:  esapi.IntPtr(10000), // Assuming up to 10000 questions, adjust as necessary
	}
	res, err := searchReq.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error fetching questions: %v", err)
		http.Error(w, "Failed to retrieve necessary data (questions)", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Printf("Elasticsearch error fetching questions: %s", res.String())
		http.Error(w, "Failed to retrieve necessary data (questions) due to DB error", http.StatusInternalServerError)
		return
	}

	var questionsResult map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&questionsResult); err != nil {
		log.Printf("Error decoding questions result: %v", err)
		http.Error(w, "Failed to parse necessary data (questions)", http.StatusInternalServerError)
		return
	}

	if hits, ok := questionsResult["hits"].(map[string]interface{}); ok {
		if actualHits, ok := hits["hits"].([]interface{}); ok {
			for _, hit := range actualHits {
				if source, ok := hit.(map[string]interface{})["_source"]; ok {
					var q models.Question
					sourceBytes, _ := json.Marshal(source)
					if err := json.Unmarshal(sourceBytes, &q); err == nil && q.ID != "" {
						questionsMap[q.ID] = q
					}
				}
			}
		}
	}
	if len(questionsMap) == 0 {
		// This could be an error or simply no questions defined yet.
		// Depending on requirements, might want to error out if no questions are found.
		log.Println("Warning: No questions found in the database.")
	}

	// 2. Create PRRSubmission object
	submission := models.PRRSubmission{
		ID:            uuid.New().String(),
		ServiceID:     requestPayload.ServiceID,
		UserID:        requestPayload.UserID,
		Timestamp:     time.Now().UTC(),
		Answers:       requestPayload.Answers,
		SectionScores: make(map[string]models.SectionScore),
	}

	// 3. Calculate SectionScores
	for _, answer := range submission.Answers {
		question, ok := questionsMap[answer.QuestionID]
		if !ok {
			log.Printf("Warning: QuestionID '%s' from submission not found in fetched questions. Skipping for scoring.", answer.QuestionID)
			continue // Or handle as an error, e.g., by returning http.StatusBadRequest
		}

		sectionID := question.SectionID
		if sectionID == "" {
			log.Printf("Warning: QuestionID '%s' has an empty SectionID. Skipping for scoring.", answer.QuestionID)
			continue
		}

		score, sectionExists := submission.SectionScores[sectionID]
		if !sectionExists {
			score = models.SectionScore{SectionID: sectionID}
		}

		switch strings.ToLower(answer.Response) {
		case "yes":
			score.YesCount++
		case "no":
			score.NoCount++
		case "n/a": // Assuming "N/A" is the exact string for Not Applicable
			score.NaCount++
		default:
			log.Printf("Warning: Unknown response value '%s' for QuestionID '%s'.", answer.Response, answer.QuestionID)
			// Optionally, count unknown responses or handle as an error
		}
		submission.SectionScores[sectionID] = score
	}

	// 4. Index the PRRSubmission
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(submission); err != nil {
		log.Printf("Error encoding PRR submission: %v", err)
		http.Error(w, "Failed to process submission data", http.StatusInternalServerError)
		return
	}

	indexReq := esapi.IndexRequest{
		Index:      "prr_submissions",
		DocumentID: submission.ID,
		Body:       &buf,
		Refresh:    "true",
	}

	idxRes, err := indexReq.Do(context.Background(), esClient)
	if err != nil {
		log.Printf("Error indexing PRR submission: %v", err)
		http.Error(w, "Failed to save submission", http.StatusInternalServerError)
		return
	}
	defer idxRes.Body.Close()

	if idxRes.IsError() {
		log.Printf("Elasticsearch indexing error for PRR submission: %s", idxRes.String())
		http.Error(w, "Failed to save submission due to database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(submission)
}
