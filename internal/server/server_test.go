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
}

func (m *mockStorage) GetHistory(ctx context.Context) ([]model.HistoryEntry, error) {
	return []model.HistoryEntry{
		{ID: "507f1f77bcf86cd799439011", F1Score: 0.85},
	}, nil
}

func (m *mockStorage) GetAuditResult(ctx context.Context, id string) (*model.AuditResult, error) {
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
	mux := NewServerMux(store)

	t.Run("History API", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/history", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var history model.AuditHistory
		json.NewDecoder(w.Body).Decode(&history)
		if len(history.Entries) != 1 || history.Entries[0].F1Score != 0.85 {
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

	report, err := GenerateStandaloneReport(result)
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
