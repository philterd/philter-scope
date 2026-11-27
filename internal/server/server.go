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

	"github.com/philterd/philterscope/pkg/model"
)

//go:embed index.html
var staticAssets embed.FS

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
	tmpl, err := template.ParseFS(staticAssets, "index.html")
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
	tmpl, err := template.ParseFS(staticAssets, "index.html")
	if err != nil {
		return nil, err
	}

	if privacyMode {
		result = ObfuscateAuditResult(result)
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

// obfuscateResult replaces the text of every labeled and detected span with
// its tag. Span offsets index the original text, not Philter's output, where
// each replacement shifts the text after it. So the Philter output is rebuilt
// from the original text and the detected spans' replacements, rather than
// tagged at offsets that do not point into it.
func obfuscateResult(r model.Result) model.Result {
	// The ranges of the original text to hide, sorted and merged.
	var hidden []textRange
	hide := func(s model.Span) {
		if s.CharacterStart < s.CharacterEnd {
			hidden = append(hidden, textRange{start: s.CharacterStart, end: s.CharacterEnd})
		}
	}
	for _, s := range r.Spans {
		hide(s)
	}
	for _, o := range r.Overlaps {
		hide(o.Golden)
		hide(o.Actual)
	}
	hidden = mergeRanges(clampRanges(hidden, len(r.Expected)))

	// Where Philter replaced text, by the spans it applied.
	var replaced []textRange
	for _, s := range r.Spans {
		if s.CharacterStart < s.CharacterEnd {
			replaced = append(replaced, textRange{start: s.CharacterStart, end: s.CharacterEnd, replacement: s.Replacement})
		}
	}
	replaced = clampRanges(replaced, len(r.Expected))
	slices.SortFunc(replaced, func(a, b textRange) int { return a.start - b.start })

	expected, inExpected := obfuscateText(r.Expected, nil, hidden)
	actual, inActual := obfuscateText(r.Expected, replaced, hidden)

	r.Expected = expected
	r.Actual = actual

	for i := range r.Spans {
		r.Spans[i] = moveSpan(r.Spans[i], inActual)
	}
	for i := range r.Overlaps {
		r.Overlaps[i].Golden = moveSpan(r.Overlaps[i].Golden, inExpected)
		r.Overlaps[i].Actual = moveSpan(r.Overlaps[i].Actual, inActual)
	}

	return r
}

// textRange is a range of the original text. A replaced range also carries
// Philter's replacement for it.
type textRange struct {
	start, end  int
	replacement string
}

// clampRanges drops the ranges that start past the end of the text and cuts
// the rest at its end.
func clampRanges(ranges []textRange, length int) []textRange {
	var clamped []textRange
	for _, t := range ranges {
		t.start = max(t.start, 0)
		t.end = min(t.end, length)
		if t.start < t.end {
			clamped = append(clamped, t)
		}
	}
	return clamped
}

// mergeRanges sorts ranges and merges the ones that overlap.
func mergeRanges(ranges []textRange) []textRange {
	slices.SortFunc(ranges, func(a, b textRange) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})
	var merged []textRange
	for _, t := range ranges {
		if n := len(merged); n > 0 && t.start < merged[n-1].end {
			merged[n-1].end = max(merged[n-1].end, t.end)
			continue
		}
		merged = append(merged, t)
	}
	return merged
}

// piece is a range of the original text and what it became in the output.
type piece struct {
	start, end       int
	outStart, outEnd int
	kept             bool // copied as it is, so offsets inside it move by a constant
}

// obfuscateText writes text with each replaced range swapped for its
// replacement and every other hidden character swapped for the tag of the run
// it is in. A replaced range without a replacement, as in a report written
// before Philter returned them, gets the tag of its text. It returns the output
// and a function that moves an offset in text to the output: a start offset
// inside a replaced or tagged run moves to the run's start, and an end offset
// to its end.
func obfuscateText(text string, replaced, hidden []textRange) (string, func(offset int, isEnd bool) int) {
	var buf bytes.Buffer
	var pieces []piece
	write := func(start, end int, out string, kept bool) {
		outStart := buf.Len()
		buf.WriteString(out)
		pieces = append(pieces, piece{start: start, end: end, outStart: outStart, outEnd: buf.Len(), kept: kept})
	}

	// keep writes text[from:to], tagging the parts of it that are hidden.
	keep := func(from, to int) {
		for _, h := range hidden {
			if h.end <= from {
				continue
			}
			if h.start >= to {
				break
			}
			if h.start > from {
				write(from, h.start, text[from:h.start], true)
				from = h.start
			}
			end := min(h.end, to)
			write(from, end, hashText(text[from:end]), false)
			from = end
		}
		if from < to {
			write(from, to, text[from:to], true)
		}
	}

	pos := 0
	for _, rep := range replaced {
		if rep.start < pos {
			// Overlaps a range already replaced. Philter does not apply
			// overlapping spans, so tag the rest rather than guess.
			if rep.end > pos {
				write(pos, rep.end, hashText(text[pos:rep.end]), false)
				pos = rep.end
			}
			continue
		}
		keep(pos, rep.start)
		out := rep.replacement
		if out == "" {
			out = hashText(text[rep.start:rep.end])
		}
		write(rep.start, rep.end, out, false)
		pos = rep.end
	}
	keep(pos, len(text))

	length := buf.Len()
	move := func(offset int, isEnd bool) int {
		for _, p := range pieces {
			if offset >= p.end {
				continue
			}
			if offset <= p.start {
				return p.outStart
			}
			if p.kept {
				return p.outStart + offset - p.start
			}
			if isEnd {
				return p.outEnd
			}
			return p.outStart
		}
		return length
	}

	return buf.String(), move
}

// moveSpan moves a span to the obfuscated text and replaces its text with its
// tag.
func moveSpan(s model.Span, move func(offset int, isEnd bool) int) model.Span {
	s.CharacterStart, s.CharacterEnd = move(s.CharacterStart, false), move(s.CharacterEnd, true)
	s.Text = hashText(s.Text)
	return s
}
