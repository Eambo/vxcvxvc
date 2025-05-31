package models

import "time"

// Service represents a service that can be reviewed.
type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Section represents a section of questions in a review.
type Section struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Question represents a question in a review.
type Question struct {
	ID             string `json:"id"`
	SectionID      string `json:"section_id"`
	Text           string `json:"text"`
	Blurb          string `json:"blurb"`
	SupportingLink string `json:"supporting_link"`
	IsEssential    bool   `json:"is_essential"`
	Order          int    `json:"order"`
}

// Answer represents an answer to a question.
type Answer struct {
	QuestionID string `json:"question_id"`
	Response   string `json:"response"` // "Yes", "No", "N/A"
}

// PRRSubmission represents a Product Readiness Review submission.
type PRRSubmission struct {
	ID            string                  `json:"id"`
	ServiceID     string                  `json:"service_id"`
	UserID        string                  `json:"user_id"`
	Timestamp     time.Time               `json:"timestamp"`
	Answers       []Answer                `json:"answers"`
	SectionScores map[string]SectionScore `json:"section_scores"`
}

// SectionScore represents the score for a section.
type SectionScore struct {
	SectionID string `json:"section_id"`
	YesCount  int    `json:"yes_count"`
	NoCount   int    `json:"no_count"`
	NaCount   int    `json:"na_count"`
}
