package models

import "time"

// Service represents a distinct product or component that undergoes Product Readiness Reviews.
// It is uniquely identified by an ID and has a human-readable Name.
type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Section represents a thematic grouping of questions within a Product Readiness Review.
// Each section has an ID, Name, and a Description outlining its purpose.
type Section struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Question defines an individual query item within a PRR Section.
// It includes the question Text, a Blurb for context, a SupportingLink for more info,
// its essentiality (IsEssential), and an Order for display sequence.
type Question struct {
	ID             string `json:"id"`
	SectionID      string `json:"section_id"` // Links to the Section this question belongs to.
	Text           string `json:"text"`
	Blurb          string `json:"blurb"`
	SupportingLink string `json:"supporting_link"`
	IsEssential    bool   `json:"is_essential"`
	Order          int    `json:"order"`
}

// Answer holds the response to a specific Question within a PRRSubmission.
// The Response can be one of "Yes", "No", or "N/A".
type Answer struct {
	QuestionID string `json:"question_id"` // Links to the Question being answered.
	Response   string `json:"response"`    // "Yes", "No", "N/A"
}

// PRRSubmission captures a complete Product Readiness Review for a specific Service by a User.
// It includes all answers given, calculated scores per section, and a timestamp.
type PRRSubmission struct {
	ID            string                  `json:"id"`
	ServiceID     string                  `json:"service_id"` // Links to the Service being reviewed.
	UserID        string                  `json:"user_id"`
	Timestamp     time.Time               `json:"timestamp"`    // Time of submission.
	Answers       []Answer                `json:"answers"`      // List of all answers.
	SectionScores map[string]SectionScore `json:"section_scores"` // Calculated scores, keyed by SectionID.
}

// SectionScore aggregates the counts of Yes, No, and N/A answers for a specific Section.
type SectionScore struct {
	SectionID string `json:"section_id"` // Links to the Section.
	YesCount  int    `json:"yes_count"`
	NoCount   int    `json:"no_count"`
	NaCount   int    `json:"na_count"`
}

// SectionScoreComparison holds the old and new scores for a section, used in PRRComparisonReport.
type SectionScoreComparison struct {
	OldScores SectionScore `json:"oldScores"`
	NewScores SectionScore `json:"newScores"`
}

// AnswerChangeDetail describes the change in a single answer between two PRR submissions.
// It includes the QuestionID, optional QuestionText, and the Old and New answers.
type AnswerChangeDetail struct {
	QuestionID   string `json:"questionId"`
	QuestionText string `json:"questionText,omitempty"`
	OldAnswer    string `json:"oldAnswer"`
	NewAnswer    string `json:"newAnswer"`
}

// PRRComparisonReport provides a structured view of differences between two PRR submissions,
// detailing changes in section scores and individual answers.
type PRRComparisonReport struct {
	ServiceID                 string                              `json:"serviceId"`
	PRRSubmissionIDOld        string                              `json:"prrSubmissionIdOld"`
	PRRSubmissionIDNew        string                              `json:"prrSubmissionIdNew"`
	SectionComparison         map[string]SectionScoreComparison `json:"sectionComparison"`
	AnswerChanges             []AnswerChangeDetail                `json:"answerChanges"`
	NewlyAnsweredQuestions    []AnswerChangeDetail                `json:"newlyAnsweredQuestions"`
	NoLongerAnsweredQuestions []AnswerChangeDetail                `json:"noLongerAnsweredQuestions"`
}

// ServiceSearchResult represents a service in search results, augmented with its latest PRR information.
// This includes the service's basic details, scores from its most recent PRR, and its timestamp.
type ServiceSearchResult struct {
	ServiceID         string                  `json:"serviceId"`
	ServiceName       string                  `json:"serviceName"`
	LatestPRRScores   map[string]SectionScore `json:"latestPrrScores,omitempty"`
	LastPRRTimestamp  *time.Time              `json:"lastPrrTimestamp,omitempty"`
}
