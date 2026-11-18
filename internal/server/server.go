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
	"html/template"
	"net/http"

	"github.com/philterd/philterscope/internal/report"
	"github.com/philterd/philterscope/pkg/model"
)

// Storage defines the interface for data retrieval.
type Storage interface {
	GetHistory(ctx context.Context) ([]model.HistoryEntry, error)
	GetAuditResult(ctx context.Context, id string) (*model.AuditResult, error)
	DeleteAuditResult(ctx context.Context, id string) error
	ResolveRecommendation(ctx context.Context, auditID string, ref string) error
	DismissRecommendation(ctx context.Context, auditID string, ref string) error
	SaveAuditNotes(ctx context.Context, id string, notes string) error
	SaveRecommendations(ctx context.Context, id string, recs []model.Recommendation) error
}

// StartServer launches the local Evaluation UI.
func StartServer(port int, store Storage, privacyMode bool) error {
	mux := NewServerMux(store, privacyMode)
	fmt.Printf("Evaluation UI: http://localhost:%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

// NewServerMux creates a new http.ServeMux with the necessary handlers.
func NewServerMux(store Storage, privacyMode bool) *http.ServeMux {
	mux := http.NewServeMux()
	tmpl, err := report.Template()
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
	}

	registerHealth(mux)
	registerReadiness(mux, store)

	m := newMetrics()
	m.register(mux)

	// API to get history
	mux.HandleFunc("/api/history", m.instrument("/api/history", func(w http.ResponseWriter, r *http.Request) {
		history, err := store.GetHistory(r.Context())
		response := model.AuditHistory{
			Entries: history,
		}
		if err != nil {
			fmt.Printf("Error getting history: %v\n", err)
			response.Error = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			fmt.Printf("Error encoding history: %v\n", err)
		}
	}))

	// API to get or delete specific audit
	mux.HandleFunc("/api/audit", m.instrument("/api/audit", func(w http.ResponseWriter, r *http.Request) {
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

			if privacyMode {
				*res = report.ObfuscateAuditResult(*res)
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
	}))

	// API to resolve a recommendation
	mux.HandleFunc("/api/audit/recommendation/resolve", m.instrument("/api/audit/recommendation/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auditID := r.URL.Query().Get("id")
		ref := recommendationRef(r)
		if auditID == "" || ref == "" {
			http.Error(w, "missing id or rec parameter", http.StatusBadRequest)
			return
		}

		err := store.ResolveRecommendation(r.Context(), auditID, ref)
		if err != nil {
			fmt.Printf("Error resolving recommendation (id=%s, rec=%s): %v\n", auditID, ref, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// API to dismiss a recommendation
	mux.HandleFunc("/api/audit/recommendation/dismiss", m.instrument("/api/audit/recommendation/dismiss", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		auditID := r.URL.Query().Get("id")
		ref := recommendationRef(r)
		if auditID == "" || ref == "" {
			http.Error(w, "missing id or rec parameter", http.StatusBadRequest)
			return
		}

		err := store.DismissRecommendation(r.Context(), auditID, ref)
		if err != nil {
			fmt.Printf("Error dismissing recommendation (id=%s, rec=%s): %v\n", auditID, ref, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// API to save notes
	mux.HandleFunc("/api/audit/notes", m.instrument("/api/audit/notes", func(w http.ResponseWriter, r *http.Request) {
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
	}))

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

	return mux
}

// StartStandaloneServer launches the local Evaluation UI with a single result.
func StartStandaloneServer(port int, result model.AuditResult, privacyMode bool) error {
	mux, err := NewStandaloneServerMux(result, privacyMode)
	if err != nil {
		return err
	}

	fmt.Printf("Evaluation UI available at http://localhost:%d\n\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

// NewStandaloneServerMux creates an http.ServeMux that serves a single result
// without any backing storage.
func NewStandaloneServerMux(result model.AuditResult, privacyMode bool) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	tmpl, err := report.Template()
	if err != nil {
		return nil, err
	}

	if privacyMode {
		result = report.ObfuscateAuditResult(result)
	}

	registerHealth(mux)
	// No backing store to check, so this reports ready as soon as it serves.
	registerReadiness(mux, nil)
	newMetrics().register(mux)

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
		if err := tmpl.Execute(w, data); err != nil {
			fmt.Printf("Error executing template: %v\n", err)
		}
	})

	return mux, nil
}

// recommendationRef reads which recommendation a request is addressing. The
// "rec" parameter carries a recommendation ID, which is what identifies one now
// that an entity can have several. An older client, or a report stored before
// recommendations had IDs, sends "entity" instead and the storage layer falls
// back to matching on that.
func recommendationRef(r *http.Request) string {
	if ref := r.URL.Query().Get("rec"); ref != "" {
		return ref
	}
	return r.URL.Query().Get("entity")
}
