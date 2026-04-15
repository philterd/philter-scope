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
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/philterd/philterscope/pkg/model"
)

//go:embed index.html
var staticAssets embed.FS

// Storage defines the interface for data retrieval.
type Storage interface {
	GetHistory(ctx context.Context) ([]model.HistoryEntry, error)
	GetAuditResult(ctx context.Context, id string) (*model.AuditResult, error)
	DeleteAuditResult(ctx context.Context, id string) error
	ResolveRecommendation(ctx context.Context, auditID string, entity string) error
	DismissRecommendation(ctx context.Context, auditID string, entity string) error
	SaveAuditNotes(ctx context.Context, id string, notes string) error
	SaveRecommendations(ctx context.Context, id string, recs []model.Recommendation) error
}

// StartServer launches the local Evaluation UI.
func StartServer(port int, store Storage) error {
	mux := http.NewServeMux()
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return err
	}

	// API to get history
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		history, err := store.GetHistory(r.Context())
		if err != nil {
			fmt.Printf("Error getting history: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(history); err != nil {
			fmt.Printf("Error encoding history: %v\n", err)
		}
	})

	// API to get or delete specific audit
	mux.HandleFunc("/api/audit", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			res, err := store.GetAuditResult(r.Context(), id)
			if err != nil {
				fmt.Printf("Error getting audit result (id=%s): %v\n", id, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(res); err != nil {
				fmt.Printf("Error encoding audit result: %v\n", err)
			}
		case http.MethodDelete:
			err := store.DeleteAuditResult(r.Context(), id)
			if err != nil {
				fmt.Printf("Error deleting audit result (id=%s): %v\n", id, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// API to resolve a recommendation
	mux.HandleFunc("/api/audit/recommendation/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auditID := r.URL.Query().Get("id")
		entity := r.URL.Query().Get("entity")
		if auditID == "" || entity == "" {
			http.Error(w, "missing id or entity parameter", http.StatusBadRequest)
			return
		}

		err := store.ResolveRecommendation(r.Context(), auditID, entity)
		if err != nil {
			fmt.Printf("Error resolving recommendation (id=%s, entity=%s): %v\n", auditID, entity, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// API to dismiss a recommendation
	mux.HandleFunc("/api/audit/recommendation/dismiss", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auditID := r.URL.Query().Get("id")
		entity := r.URL.Query().Get("entity")
		if auditID == "" || entity == "" {
			http.Error(w, "missing id or entity parameter", http.StatusBadRequest)
			return
		}

		err := store.DismissRecommendation(r.Context(), auditID, entity)
		if err != nil {
			fmt.Printf("Error dismissing recommendation (id=%s, entity=%s): %v\n", auditID, entity, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// API to save notes
	mux.HandleFunc("/api/audit/notes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id parameter", http.StatusBadRequest)
			return
		}

		var payload struct {
			Notes string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := store.SaveAuditNotes(r.Context(), id, payload.Notes)
		if err != nil {
			fmt.Printf("Error saving notes (id=%s): %v\n", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Main UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// For MongoDB mode, we don't pre-populate the report data
		// The UI will fetch it via API
		data := struct {
			ReportJSON template.JS
		}{
			ReportJSON: template.JS("null"),
		}
		if err := tmpl.Execute(w, data); err != nil {
			fmt.Printf("Error executing template: %v\n", err)
		}
	})

	fmt.Printf("Evaluation UI available at http://localhost:%d\n\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

// StartStandaloneServer launches the local Evaluation UI with a single result.
func StartStandaloneServer(port int, result model.AuditResult) error {
	mux := http.NewServeMux()
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return err
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		reportJSON, _ := json.Marshal(result)
		data := struct {
			ReportJSON template.JS
		}{
			ReportJSON: template.JS(reportJSON),
		}
		tmpl.Execute(w, data)
	})

	fmt.Printf("Evaluation UI available at http://localhost:%d\n\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

// GenerateStandaloneReport creates a self-contained HTML file.
func GenerateStandaloneReport(result model.AuditResult) (string, error) {
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return "", err
	}

	reportJSON, _ := json.Marshal(result)
	data := struct {
		ReportJSON template.JS
	}{
		ReportJSON: template.JS(reportJSON),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
