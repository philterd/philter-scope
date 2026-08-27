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
	"crypto/md5"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"sort"

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
func StartServer(port int, store Storage, privacyMode bool) error {
	mux := NewServerMux(store, privacyMode)
	fmt.Printf("Evaluation UI: http://localhost:%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

// NewServerMux creates a new http.ServeMux with the necessary handlers.
func NewServerMux(store Storage, privacyMode bool) *http.ServeMux {
	mux := http.NewServeMux()
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		fmt.Printf("Error parsing template: %v\n", err)
	}

	registerHealth(mux)

	// API to get history
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
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

			if privacyMode {
				*res = ObfuscateAuditResult(*res)
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
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return nil, err
	}

	if privacyMode {
		result = ObfuscateAuditResult(result)
	}

	registerHealth(mux)

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

// GenerateStandaloneReport creates a self-contained HTML file.
func GenerateStandaloneReport(result model.AuditResult, privacyMode bool) (string, error) {
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return "", err
	}

	if privacyMode {
		result = ObfuscateAuditResult(result)
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

// ObfuscateAuditResult redacts PII from the audit result.
func ObfuscateAuditResult(res model.AuditResult) model.AuditResult {
	for i := range res.Details {
		res.Details[i] = obfuscateResult(res.Details[i])
	}
	return res
}

func hashText(text string) string {
	if text == "" {
		return ""
	}
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])[:5]
}

func obfuscateResult(r model.Result) model.Result {
	// Collect all spans that might contain PII and need redaction in text
	type redaction struct {
		start, end  int
		replacement string
	}
	var redactions []redaction

	collectRedactions := func(spans []model.Span) {
		for _, s := range spans {
			if s.CharacterStart < s.CharacterEnd {
				redactions = append(redactions, redaction{
					start:       s.CharacterStart,
					end:         s.CharacterEnd,
					replacement: hashText(s.Text),
				})
			}
		}
	}

	collectRedactions(r.Spans)
	for _, o := range r.Overlaps {
		if o.Golden.Text != "" {
			collectRedactions([]model.Span{o.Golden})
		}
		if o.Actual.Text != "" {
			collectRedactions([]model.Span{o.Actual})
		}
	}

	// Sort and merge overlapping redactions
	slices.SortFunc(redactions, func(a, b redaction) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})

	var merged []redaction
	if len(redactions) > 0 {
		curr := redactions[0]
		for i := 1; i < len(redactions); i++ {
			if redactions[i].start < curr.end {
				// Overlap
				if redactions[i].end > curr.end {
					curr.end = redactions[i].end
					// We need a replacement for the merged span.
					// Using hash of the original merged text would be best but we don't have it easily here.
					// Let's just keep the first replacement or re-calculate if we had the text.
				}
			} else {
				merged = append(merged, curr)
				curr = redactions[i]
			}
		}
		merged = append(merged, curr)
	}

	// Function to apply redactions and update spans
	apply := func(text string) (string, func(int) int) {
		var buf bytes.Buffer
		lastIdx := 0

		type shift struct {
			origIdx int
			newIdx  int
		}
		shifts := []shift{{0, 0}}

		for _, red := range merged {
			if red.start < lastIdx {
				continue
			} // Should not happen with merged
			if red.start >= len(text) {
				break
			}

			buf.WriteString(text[lastIdx:red.start])

			// If we didn't have the text for the merged span before, we can get it now to hash it properly
			actualReplacement := red.replacement
			if red.end > len(text) {
				red.end = len(text)
			}
			actualReplacement = hashText(text[red.start:red.end])

			newStart := buf.Len()
			buf.WriteString(actualReplacement)
			lastIdx = red.end

			shifts = append(shifts, shift{red.start, newStart})
			shifts = append(shifts, shift{red.end, buf.Len()})
		}
		buf.WriteString(text[lastIdx:])

		transform := func(orig int) int {
			// Find the last shift that is <= orig
			idx := sort.Search(len(shifts), func(i int) bool {
				return shifts[i].origIdx > orig
			})
			if idx == 0 {
				return orig
			}
			s := shifts[idx-1]

			// If orig was inside a redacted span, it should probably move to the start or end of the replacement
			// But for our purposes, linear interpolation or just using the shift at the start is enough if we only care about span boundaries.

			// Check if orig is exactly one of our tracked points
			if s.origIdx == orig {
				return s.newIdx
			}

			// If it's between points, it's either in a kept segment or a redacted segment.
			// If idx < len(shifts) and s.origIdx was a start and shifts[idx].origIdx was an end:
			// Then orig is inside a redacted span.
			if idx < len(shifts) && (idx-1)%2 == 1 {
				// It's inside a redacted span. map to start of replacement.
				return s.newIdx
			}

			return s.newIdx + (orig - s.origIdx)
		}

		return buf.String(), transform
	}

	expected, transExp := apply(r.Expected)
	actual, transAct := apply(r.Actual)

	r.Expected = expected
	r.Actual = actual

	// Update spans in r.Spans
	for i := range r.Spans {
		r.Spans[i].CharacterStart = transAct(r.Spans[i].CharacterStart)
		r.Spans[i].CharacterEnd = transAct(r.Spans[i].CharacterEnd)
		r.Spans[i].Text = hashText(r.Spans[i].Text)
	}

	// Update spans in Overlaps
	for i := range r.Overlaps {
		if r.Overlaps[i].Golden.Text != "" {
			r.Overlaps[i].Golden.CharacterStart = transExp(r.Overlaps[i].Golden.CharacterStart)
			r.Overlaps[i].Golden.CharacterEnd = transExp(r.Overlaps[i].Golden.CharacterEnd)
			r.Overlaps[i].Golden.Text = hashText(r.Overlaps[i].Golden.Text)
		}
		if r.Overlaps[i].Actual.Text != "" {
			r.Overlaps[i].Actual.CharacterStart = transAct(r.Overlaps[i].Actual.CharacterStart)
			r.Overlaps[i].Actual.CharacterEnd = transAct(r.Overlaps[i].Actual.CharacterEnd)
			r.Overlaps[i].Actual.Text = hashText(r.Overlaps[i].Actual.Text)
		}
	}

	return r
}
