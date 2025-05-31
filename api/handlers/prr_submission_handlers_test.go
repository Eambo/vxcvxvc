package handlers

import (
	"reflect"
	"testing"
	"time"

	"github.com/user/prr/models"
)

func TestGenerateComparisonReport(t *testing.T) {
	allQuestions := map[string]models.Question{
		"q1": {ID: "q1", SectionID: "s1", Text: "Question 1 Text"},
		"q2": {ID: "q2", SectionID: "s1", Text: "Question 2 Text"},
		"q3": {ID: "q3", SectionID: "s2", Text: "Question 3 Text"},
		"q4": {ID: "q4", SectionID: "s2", Text: "Question 4 Text"},
	}
	now := time.Now().UTC() // Use UTC for consistency
	yesterday := now.Add(-24 * time.Hour)

	testCases := []struct {
		name           string
		oldPRR         models.PRRSubmission
		newPRR         models.PRRSubmission
		allQuestions   map[string]models.Question
		serviceID      string
		expectedReport models.PRRComparisonReport
	}{
		// --- Test Case 1: No changes ---
		{
			name: "No changes",
			oldPRR: models.PRRSubmission{
				ID:        "prrOld1",
				ServiceID: "service1",
				Timestamp: yesterday,
				Answers:   []models.Answer{{QuestionID: "q1", Response: "Yes"}},
				SectionScores: map[string]models.SectionScore{"s1": {SectionID: "s1", YesCount: 1}},
			},
			newPRR: models.PRRSubmission{
				ID:        "prrNew1",
				ServiceID: "service1",
				Timestamp: now,
				Answers:   []models.Answer{{QuestionID: "q1", Response: "Yes"}},
				SectionScores: map[string]models.SectionScore{"s1": {SectionID: "s1", YesCount: 1}},
			},
			allQuestions: allQuestions,
			serviceID:    "service1",
			expectedReport: models.PRRComparisonReport{
				ServiceID:          "service1",
				PRRSubmissionIDOld: "prrOld1",
				PRRSubmissionIDNew: "prrNew1",
				SectionComparison: map[string]models.SectionScoreComparison{
					"s1": {OldScores: models.SectionScore{SectionID: "s1", YesCount: 1}, NewScores: models.SectionScore{SectionID: "s1", YesCount: 1}},
				},
				AnswerChanges:             []models.AnswerChangeDetail{},
				NewlyAnsweredQuestions:    []models.AnswerChangeDetail{},
				NoLongerAnsweredQuestions: []models.AnswerChangeDetail{},
			},
		},
		// --- Test Case 2: Answer change ---
		{
			name: "One answer change",
			oldPRR: models.PRRSubmission{
				ID:        "prrOld2",
				ServiceID: "service1",
				Timestamp: yesterday,
				Answers:   []models.Answer{{QuestionID: "q1", Response: "Yes"}},
				SectionScores: map[string]models.SectionScore{"s1": {SectionID: "s1", YesCount: 1}},
			},
			newPRR: models.PRRSubmission{
				ID:        "prrNew2",
				ServiceID: "service1",
				Timestamp: now,
				Answers:   []models.Answer{{QuestionID: "q1", Response: "No"}},
				SectionScores: map[string]models.SectionScore{"s1": {SectionID: "s1", NoCount: 1}},
			},
			allQuestions: allQuestions,
			serviceID:    "service1",
			expectedReport: models.PRRComparisonReport{
				ServiceID:          "service1",
				PRRSubmissionIDOld: "prrOld2",
				PRRSubmissionIDNew: "prrNew2",
				SectionComparison: map[string]models.SectionScoreComparison{
					"s1": {OldScores: models.SectionScore{SectionID: "s1", YesCount: 1}, NewScores: models.SectionScore{SectionID: "s1", NoCount: 1}},
				},
				AnswerChanges: []models.AnswerChangeDetail{
					{QuestionID: "q1", QuestionText: "Question 1 Text", OldAnswer: "Yes", NewAnswer: "No"},
				},
				NewlyAnsweredQuestions:    []models.AnswerChangeDetail{},
				NoLongerAnsweredQuestions: []models.AnswerChangeDetail{},
			},
		},
		// --- Test Case 3: Newly answered and No longer answered ---
		{
			name: "Newly and No Longer Answered",
			oldPRR: models.PRRSubmission{
				ID:        "prrOld3",
				ServiceID: "service1",
				Timestamp: yesterday,
				Answers:   []models.Answer{{QuestionID: "q1", Response: "Yes"}}, // q1 answered
				SectionScores: map[string]models.SectionScore{"s1": {SectionID: "s1", YesCount: 1}},
			},
			newPRR: models.PRRSubmission{
				ID:        "prrNew3",
				ServiceID: "service1",
				Timestamp: now,
				Answers:   []models.Answer{{QuestionID: "q2", Response: "No"}}, // q2 answered, q1 not
				SectionScores: map[string]models.SectionScore{"s1": {SectionID: "s1", NoCount: 1}},
			},
			allQuestions: allQuestions,
			serviceID:    "service1",
			expectedReport: models.PRRComparisonReport{
				ServiceID:          "service1",
				PRRSubmissionIDOld: "prrOld3",
				PRRSubmissionIDNew: "prrNew3",
				SectionComparison: map[string]models.SectionScoreComparison{
					// s1 old has 1 yes, s1 new has 1 no.
					"s1": {OldScores: models.SectionScore{SectionID: "s1", YesCount: 1}, NewScores: models.SectionScore{SectionID: "s1", NoCount: 1}},
				},
				AnswerChanges: []models.AnswerChangeDetail{},
				NewlyAnsweredQuestions: []models.AnswerChangeDetail{
					{QuestionID: "q2", QuestionText: "Question 2 Text", OldAnswer: "", NewAnswer: "No"},
				},
				NoLongerAnsweredQuestions: []models.AnswerChangeDetail{
					{QuestionID: "q1", QuestionText: "Question 1 Text", OldAnswer: "Yes", NewAnswer: ""},
				},
			},
		},
		// --- Test Case 4: Section score changes due to multiple answers, including different sections ---
		{
			name: "Section score changes complex",
			oldPRR: models.PRRSubmission{
				ID:        "prrOld4",
				ServiceID: "service1",
				Timestamp: yesterday,
				Answers: []models.Answer{
					{QuestionID: "q1", Response: "Yes"}, // s1
					{QuestionID: "q3", Response: "N/A"}, // s2
				},
				SectionScores: map[string]models.SectionScore{
					"s1": {SectionID: "s1", YesCount: 1},
					"s2": {SectionID: "s2", NaCount: 1},
				},
			},
			newPRR: models.PRRSubmission{
				ID:        "prrNew4",
				ServiceID: "service1",
				Timestamp: now,
				Answers: []models.Answer{
					{QuestionID: "q1", Response: "No"},  // s1, changed
					{QuestionID: "q2", Response: "Yes"}, // s1, new
					{QuestionID: "q4", Response: "Yes"}, // s2, new (q3 removed)
				},
				SectionScores: map[string]models.SectionScore{
					"s1": {SectionID: "s1", NoCount: 1, YesCount: 1}, // q1 no, q2 yes
					"s2": {SectionID: "s2", YesCount: 1},             // q4 yes
				},
			},
			allQuestions: allQuestions,
			serviceID:    "service1",
			expectedReport: models.PRRComparisonReport{
				ServiceID:          "service1",
				PRRSubmissionIDOld: "prrOld4",
				PRRSubmissionIDNew: "prrNew4",
				SectionComparison: map[string]models.SectionScoreComparison{
					"s1": {
						OldScores: models.SectionScore{SectionID: "s1", YesCount: 1},
						NewScores: models.SectionScore{SectionID: "s1", NoCount: 1, YesCount: 1},
					},
					"s2": {
						OldScores: models.SectionScore{SectionID: "s2", NaCount: 1},
						NewScores: models.SectionScore{SectionID: "s2", YesCount: 1},
					},
				},
				AnswerChanges: []models.AnswerChangeDetail{
					{QuestionID: "q1", QuestionText: "Question 1 Text", OldAnswer: "Yes", NewAnswer: "No"},
				},
				NewlyAnsweredQuestions: []models.AnswerChangeDetail{
					{QuestionID: "q2", QuestionText: "Question 2 Text", OldAnswer: "", NewAnswer: "Yes"},
					{QuestionID: "q4", QuestionText: "Question 4 Text", OldAnswer: "", NewAnswer: "Yes"},
				},
				NoLongerAnsweredQuestions: []models.AnswerChangeDetail{
					{QuestionID: "q3", QuestionText: "Question 3 Text", OldAnswer: "N/A", NewAnswer: ""},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure slices in expected report are not nil if empty, for DeepEqual
			if tc.expectedReport.AnswerChanges == nil { tc.expectedReport.AnswerChanges = []models.AnswerChangeDetail{} }
			if tc.expectedReport.NewlyAnsweredQuestions == nil { tc.expectedReport.NewlyAnsweredQuestions = []models.AnswerChangeDetail{} }
			if tc.expectedReport.NoLongerAnsweredQuestions == nil { tc.expectedReport.NoLongerAnsweredQuestions = []models.AnswerChangeDetail{} }

			report := generateComparisonReport(tc.oldPRR, tc.newPRR, tc.allQuestions, tc.serviceID)

			if report.ServiceID != tc.expectedReport.ServiceID {
				t.Errorf("ServiceID mismatch: got %s, want %s", report.ServiceID, tc.expectedReport.ServiceID)
			}
			if report.PRRSubmissionIDOld != tc.expectedReport.PRRSubmissionIDOld {
				t.Errorf("PRRSubmissionIDOld mismatch: got %s, want %s", report.PRRSubmissionIDOld, tc.expectedReport.PRRSubmissionIDOld)
			}
			if report.PRRSubmissionIDNew != tc.expectedReport.PRRSubmissionIDNew {
				t.Errorf("PRRSubmissionIDNew mismatch: got %s, want %s", report.PRRSubmissionIDNew, tc.expectedReport.PRRSubmissionIDNew)
			}

			if !reflect.DeepEqual(report.SectionComparison, tc.expectedReport.SectionComparison) {
				t.Errorf("SectionComparison mismatch: got %v, want %v", report.SectionComparison, tc.expectedReport.SectionComparison)
			}
			if !reflect.DeepEqual(report.AnswerChanges, tc.expectedReport.AnswerChanges) {
				t.Errorf("AnswerChanges mismatch: got %v, want %v", report.AnswerChanges, tc.expectedReport.AnswerChanges)
			}
			if !reflect.DeepEqual(report.NewlyAnsweredQuestions, tc.expectedReport.NewlyAnsweredQuestions) {
				t.Errorf("NewlyAnsweredQuestions mismatch: got %v, want %v", report.NewlyAnsweredQuestions, tc.expectedReport.NewlyAnsweredQuestions)
			}
			if !reflect.DeepEqual(report.NoLongerAnsweredQuestions, tc.expectedReport.NoLongerAnsweredQuestions) {
				t.Errorf("NoLongerAnsweredQuestions mismatch: got %v, want %v", report.NoLongerAnsweredQuestions, tc.expectedReport.NoLongerAnsweredQuestions)
			}
		})
	}
}
