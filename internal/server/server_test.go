// Copyright 2026 Philterd, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

type mockStorage struct {
	deleteCalled     bool
	deleteID         string
	resolveCalled    bool
	resolveAudit     string
	resolveEntity    string
	saveNotesCalled  bool
	saveNotesAudit   string
	saveNotesContent string
	dismissCalled    bool
	dismissAudit     string
	dismissEntity    string
	saveRecsCalled   bool

	GetAuditResultFunc func(ctx context.Context, id string) (*model.AuditResult, error)
}

func (m *mockStorage) GetHistory(ctx context.Context) ([]model.HistoryEntry, error) {
	return []model.HistoryEntry{
		{ID: "507f1f77bcf86cd799439011", F1Score: 0.85, Recall: 0.80, Threshold: 0.75},
		{ID: "507f1f77bcf86cd799439012", F1Score: 0.50, Recall: 0.40, Threshold: 0.75},
	}, nil
}

func (m *mockStorage) GetAuditResult(ctx context.Context, id string) (*model.AuditResult, error) {
	if m.GetAuditResultFunc != nil {
		return m.GetAuditResultFunc(ctx, id)
	}
	if id == "507f1f77bcf86cd799439011" {
		return &model.AuditResult{F1Score: 0.85}, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockStorage) DeleteAuditResult(ctx context.Context, id string) error {
	m.deleteCalled = true
	m.deleteID = id
	return nil
}

func (m *mockStorage) ResolveRecommendation(ctx context.Context, auditID string, entity string) error {
	m.resolveCalled = true
	m.resolveAudit = auditID
	m.resolveEntity = entity
	return nil
}

func (m *mockStorage) SaveAuditNotes(ctx context.Context, id string, notes string) error {
	m.saveNotesCalled = true
	m.saveNotesAudit = id
	m.saveNotesContent = notes
	return nil
}

func (m *mockStorage) DismissRecommendation(ctx context.Context, auditID string, entity string) error {
	m.dismissCalled = true
	m.dismissAudit = auditID
	m.dismissEntity = entity
	return nil
}

func (m *mockStorage) SaveRecommendations(ctx context.Context, id string, recs []model.Recommendation) error {
	m.saveRecsCalled = true
	return nil
}

func TestAPIs(t *testing.T) {
	store := &mockStorage{}
	mux := NewServerMux(store, false)

	t.Run("History API", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/history", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var history model.AuditHistory
		json.NewDecoder(w.Body).Decode(&history)
		if len(history.Entries) != 2 || history.Entries[0].F1Score != 0.85 {
			t.Errorf("Unexpected history data: %+v", history)
		}
	})

	t.Run("Audit API", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit?id=507f1f77bcf86cd799439011", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var res model.AuditResult
		json.NewDecoder(w.Body).Decode(&res)
		if res.F1Score != 0.85 {
			t.Errorf("Unexpected audit data: %+v", res)
		}
	})

	t.Run("Delete Audit API", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/audit?id=507f1f77bcf86cd799439011", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", w.Code)
		}

		if !store.deleteCalled {
			t.Error("DeleteAuditResult was not called")
		}

		if store.deleteID != "507f1f77bcf86cd799439011" {
			t.Errorf("Expected delete ID 507f1f77bcf86cd799439011, got %s", store.deleteID)
		}
	})

	t.Run("Resolve Recommendation API", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/audit/recommendation/resolve?id=507f1f77bcf86cd799439011&entity=DATE", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if !store.resolveCalled {
			t.Error("ResolveRecommendation was not called")
		}

		if store.resolveAudit != "507f1f77bcf86cd799439011" {
			t.Errorf("Expected audit ID 507f1f77bcf86cd799439011, got %s", store.resolveAudit)
		}

		if store.resolveEntity != "DATE" {
			t.Errorf("Expected entity DATE, got %s", store.resolveEntity)
		}
	})

	t.Run("Resolve Recommendation API By ID", func(t *testing.T) {
		// A recommendation is addressed by its ID. The entity is no longer
		// unique, since one entity can raise both a recall gap and a precision
		// warning, so "rec" is what the dashboard sends.
		store.resolveEntity = ""
		req := httptest.NewRequest("POST", "/api/audit/recommendation/resolve?id=507f1f77bcf86cd799439011&rec=precision_collapsed%3APERSON", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if store.resolveEntity != "precision_collapsed:PERSON" {
			t.Errorf("Expected the recommendation ID to be passed through, got %s", store.resolveEntity)
		}
	})

	t.Run("Recommendation API Requires A Reference", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/audit/recommendation/resolve?id=507f1f77bcf86cd799439011", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 with no recommendation named, got %d", w.Code)
		}
	})

	t.Run("Dismiss Recommendation API", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/audit/recommendation/dismiss?id=507f1f77bcf86cd799439011&entity=PHONE", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if !store.dismissCalled {
			t.Error("DismissRecommendation was not called")
		}

		if store.dismissAudit != "507f1f77bcf86cd799439011" {
			t.Errorf("Expected audit ID 507f1f77bcf86cd799439011, got %s", store.dismissAudit)
		}

		if store.dismissEntity != "PHONE" {
			t.Errorf("Expected entity PHONE, got %s", store.dismissEntity)
		}
	})

	t.Run("Save Notes API", func(t *testing.T) {
		payload := `{"notes": "This is a test note."}`
		req := httptest.NewRequest("POST", "/api/audit/notes?id=507f1f77bcf86cd799439011", strings.NewReader(payload))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if !store.saveNotesCalled {
			t.Error("SaveAuditNotes was not called")
		}

		if store.saveNotesAudit != "507f1f77bcf86cd799439011" {
			t.Errorf("Expected audit ID 507f1f77bcf86cd799439011, got %s", store.saveNotesAudit)
		}

		if store.saveNotesContent != "This is a test note." {
			t.Errorf("Expected note 'This is a test note.', got %s", store.saveNotesContent)
		}
	})

	t.Run("Index Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "<title>PhilterScope Evaluation</title>") {
			t.Error("Index page missing title")
		}
	})

	t.Run("404 Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/not-found", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("API Error Handling", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit?id=not-found", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})
}

func TestGenerateStandaloneReport(t *testing.T) {
	result := model.AuditResult{
		TotalDocuments: 1,
		Recall:         0.9,
		Precision:      0.8,
		F1Score:        0.85,
		Details: []model.Result{
			{Filename: "test.txt", Expected: "Hello World", Actual: "Hello REDACTED", TP: 1},
		},
	}

	report, err := GenerateStandaloneReport(result, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(report, "<title>PhilterScope Evaluation</title>") {
		t.Error("Report missing expected title")
	}

	if !strings.Contains(report, "test.txt") {
		t.Error("Report missing filename")
	}

	// Check if JSON data is embedded
	if !strings.Contains(report, `"total_documents":1`) {
		t.Errorf("Report missing JSON data (checked for '\"total_documents\":1')\nGot: %s", report)
	}
}

func TestObfuscateAuditResult(t *testing.T) {
	res := model.AuditResult{
		Details: []model.Result{
			{
				Filename: "test.txt",
				Expected: "My name is John Doe.",
				Actual:   "My name is John Doe.",
				Spans: []model.Span{
					{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19, Label: "NAME"},
				},
				Overlaps: []model.Overlap{
					{
						Golden: model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19},
						Actual: model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19},
						Type:   "EXACT",
					},
				},
			},
		},
	}

	obfuscated := ObfuscateAuditResult(res)

	if obfuscated.Details[0].Expected != "My name is 4c2a9." {
		t.Errorf("Expected obfuscated 'Expected' text to be 'My name is 4c2a9.', got %q", obfuscated.Details[0].Expected)
	}

	if obfuscated.Details[0].Actual != "My name is 4c2a9." {
		t.Errorf("Expected obfuscated 'Actual' text to be 'My name is 4c2a9.', got %q", obfuscated.Details[0].Actual)
	}

	if obfuscated.Details[0].Spans[0].Text != "4c2a9" {
		t.Errorf("Expected obfuscated span text to be '4c2a9', got %q", obfuscated.Details[0].Spans[0].Text)
	}

	if obfuscated.Details[0].Spans[0].CharacterEnd != 16 {
		t.Errorf("Expected obfuscated span characterEnd to be 16, got %d", obfuscated.Details[0].Spans[0].CharacterEnd)
	}

	if obfuscated.Details[0].Overlaps[0].Golden.Text != "4c2a9" {
		t.Errorf("Expected obfuscated golden span text to be '4c2a9', got %q", obfuscated.Details[0].Overlaps[0].Golden.Text)
	}

	if obfuscated.Details[0].Overlaps[0].Golden.CharacterEnd != 16 {
		t.Errorf("Expected obfuscated golden span characterEnd to be 16, got %d", obfuscated.Details[0].Overlaps[0].Golden.CharacterEnd)
	}
}

func TestObfuscatePhilterOutput(t *testing.T) {
	// Span offsets index the original text, and Philter's output differs from
	// it wherever a replacement is a different length from what it replaced.
	person := model.Span{Text: "Bob Jones", CharacterStart: 11, CharacterEnd: 20, FilterType: "PERSON", Replacement: "{{{REDACTED-person}}}"}
	firstName := model.Span{Text: "John", CharacterStart: 11, CharacterEnd: 15, FilterType: "PERSON", Replacement: "{{{REDACTED-person}}}"}
	noReplacement := model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19, FilterType: "PERSON"}

	tests := []struct {
		name         string
		result       model.Result
		wantExpected string
		wantActual   string
		hidden       []string
	}{
		{
			name: "span Philter missed",
			result: model.Result{
				Expected: "My name is Bob Jones and I live in New York.",
				Actual:   "My name is {{{REDACTED-person}}} and I live in New York.",
				Spans:    []model.Span{person},
				Overlaps: []model.Overlap{
					{Golden: model.Span{Text: "Bob Jones", CharacterStart: 11, CharacterEnd: 20, Label: "NAME"}, Actual: person, Type: model.OverlapExact},
					{Golden: model.Span{Text: "New York", CharacterStart: 35, CharacterEnd: 43, Label: "LOCATION"}, Type: model.OverlapNone},
				},
			},
			wantExpected: "My name is " + hashText("Bob Jones") + " and I live in " + hashText("New York") + ".",
			wantActual:   "My name is {{{REDACTED-person}}} and I live in " + hashText("New York") + ".",
			hidden:       []string{"Bob", "Jones", "New York"},
		},
		{
			name: "span Philter redacted in part",
			result: model.Result{
				Expected: "My name is John Doe.",
				Actual:   "My name is {{{REDACTED-person}}} Doe.",
				Spans:    []model.Span{firstName},
				Overlaps: []model.Overlap{
					{Golden: model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19, Label: "NAME"}, Actual: firstName, Type: model.OverlapPartial},
				},
			},
			wantExpected: "My name is " + hashText("John Doe") + ".",
			wantActual:   "My name is {{{REDACTED-person}}}" + hashText(" Doe") + ".",
			hidden:       []string{"John", "Doe"},
		},
		{
			name: "label without its text",
			result: model.Result{
				Expected: "Call 555-0199 now.",
				Actual:   "Call 555-0199 now.",
				Overlaps: []model.Overlap{
					{Golden: model.Span{CharacterStart: 5, CharacterEnd: 13, Label: "PHONE"}, Type: model.OverlapNone},
				},
			},
			wantExpected: "Call " + hashText("555-0199") + " now.",
			wantActual:   "Call " + hashText("555-0199") + " now.",
			hidden:       []string{"555-0199"},
		},
		{
			name: "detected span without a replacement",
			result: model.Result{
				Expected: "My name is John Doe.",
				Actual:   "My name is *****.",
				Spans:    []model.Span{noReplacement},
				Overlaps: []model.Overlap{
					{Golden: model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19, Label: "NAME"}, Actual: noReplacement, Type: model.OverlapExact},
				},
			},
			wantExpected: "My name is " + hashText("John Doe") + ".",
			wantActual:   "My name is " + hashText("John Doe") + ".",
			hidden:       []string{"John", "Doe"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// What each span should point at once obfuscated: a detected span
			// at its replacement in the Philter output, a golden span at its
			// tag in the original text.
			var wantSpans, wantGolden []string
			for _, s := range tt.result.Spans {
				want := s.Replacement
				if want == "" {
					want = hashText(tt.result.Expected[s.CharacterStart:s.CharacterEnd])
				}
				wantSpans = append(wantSpans, want)
			}
			for _, o := range tt.result.Overlaps {
				wantGolden = append(wantGolden, hashText(tt.result.Expected[o.Golden.CharacterStart:o.Golden.CharacterEnd]))
			}

			got := ObfuscateAuditResult(model.AuditResult{Details: []model.Result{tt.result}}).Details[0]

			if got.Expected != tt.wantExpected {
				t.Errorf("Expected = %q, want %q", got.Expected, tt.wantExpected)
			}
			if got.Actual != tt.wantActual {
				t.Errorf("Actual = %q, want %q", got.Actual, tt.wantActual)
			}
			for _, text := range tt.hidden {
				if strings.Contains(got.Expected, text) || strings.Contains(got.Actual, text) {
					t.Errorf("%q is still readable: Expected %q, Actual %q", text, got.Expected, got.Actual)
				}
			}

			for i, s := range got.Spans {
				if at := slice(got.Actual, s); at != wantSpans[i] {
					t.Errorf("detected span %d points at %q in the Philter output, want %q", i, at, wantSpans[i])
				}
			}
			for i, o := range got.Overlaps {
				if at := slice(got.Expected, o.Golden); at != wantGolden[i] {
					t.Errorf("golden span %d points at %q in the original, want %q", i, at, wantGolden[i])
				}
			}
		})
	}
}

// slice returns the text a span points at, or a marker if it points outside.
func slice(text string, s model.Span) string {
	if s.CharacterStart < 0 || s.CharacterStart > s.CharacterEnd || s.CharacterEnd > len(text) {
		return fmt.Sprintf("<out of range [%d, %d)>", s.CharacterStart, s.CharacterEnd)
	}
	return text[s.CharacterStart:s.CharacterEnd]
}

func TestPrivacyModeAPI(t *testing.T) {
	store := &mockStorage{}
	mux := NewServerMux(store, true) // Enable privacy mode

	req := httptest.NewRequest("GET", "/api/audit?id=507f1f77bcf86cd799439011", nil)
	w := httptest.NewRecorder()

	// Update mock to return an audit result with PII
	store.GetAuditResultFunc = func(ctx context.Context, id string) (*model.AuditResult, error) {
		return &model.AuditResult{
			Details: []model.Result{
				{
					Expected: "PII: Secret",
					Spans: []model.Span{
						{Text: "Secret", CharacterStart: 5, CharacterEnd: 11},
					},
				},
			},
		}, nil
	}

	mux.ServeHTTP(w, req)

	var res model.AuditResult
	json.NewDecoder(w.Body).Decode(&res)

	if res.Details[0].Expected != "PII: 1e694" {
		t.Errorf("API response should be obfuscated, got %q", res.Details[0].Expected)
	}
}
