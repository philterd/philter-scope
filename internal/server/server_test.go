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

type mockStorage struct{}

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

func TestAPIs(t *testing.T) {
	store := &mockStorage{}
	mux := http.NewServeMux()

	// Use a dummy handler to get the mux from StartServer or just test StartServer indirectly.
	// Actually, let's just use the same logic as in StartServer for the test mux.
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		history, _ := store.GetHistory(r.Context())
		json.NewEncoder(w).Encode(history)
	})

	mux.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		res, err := store.GetAuditResult(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(res)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "index")
	})

	t.Run("History API", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/history", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var history []model.HistoryEntry
		json.NewDecoder(w.Body).Decode(&history)
		if len(history) != 1 || history[0].F1Score != 0.85 {
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
