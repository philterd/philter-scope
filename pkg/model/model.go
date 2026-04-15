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

package model

import "time"

// Redactor defines the interface for redaction services.
type Redactor interface {
	Redact(text string) (string, []Span, error)
}

// AuditResult holds the outcome of an auditing run.
type AuditResult struct {
	ID              interface{}            `json:"id" bson:"_id,omitempty"`
	Timestamp       time.Time              `json:"timestamp" bson:"timestamp"`
	TotalDocuments  int                    `json:"total_documents" bson:"total_documents"`
	Precision       float64                `json:"precision" bson:"precision"`
	Recall          float64                `json:"recall" bson:"recall"`
	F1Score         float64                `json:"f1_score" bson:"f1_score"`
	Details         []Result               `json:"details" bson:"details"`
	Policy          map[string]interface{} `json:"policy" bson:"policy"`                   // Philter JSON configuration
	Recommendations []Recommendation       `json:"recommendations" bson:"recommendations"` // Suggested policy changes
	EntityMetrics   map[string]float64     `json:"entity_metrics" bson:"entity_metrics"`   // Recall per entity type
	Threshold       float64                `json:"threshold" bson:"threshold"`             // Threshold used for suggestions
	Notes           string                 `json:"notes" bson:"notes"`                     // User-provided notes
}

// HistoryEntry represents a past audit result.
type HistoryEntry struct {
	ID        interface{}            `json:"id" bson:"_id,omitempty"`
	Timestamp time.Time              `json:"timestamp" bson:"timestamp"`
	Precision float64                `json:"precision" bson:"precision"`
	Recall    float64                `json:"recall" bson:"recall"`
	F1Score   float64                `json:"f1_score" bson:"f1_score"`
	Policy    map[string]interface{} `json:"policy" bson:"policy"`
}

// AuditHistory represents a collection of past audit results.
type AuditHistory struct {
	Entries []HistoryEntry `json:"entries"`
}

// Recommendation holds a suggested policy change.
type Recommendation struct {
	Entity      string `json:"entity"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Snippet     string `json:"snippet"`
	Resolved    bool   `json:"resolved" bson:"resolved"`   // Mark as resolved in the database
	Dismissed   bool   `json:"dismissed" bson:"dismissed"` // Mark as dismissed in the database
}

// Result is a single comparison outcome.
type Result struct {
	Filename string    `json:"filename"`
	Expected string    `json:"expected"`
	Actual   string    `json:"actual"`
	Spans    []Span    `json:"spans"` // Detected or labeled spans
	TP       int       `json:"tp"`    // True Positives
	FP       int       `json:"fp"`    // False Positives
	FN       int       `json:"fn"`    // False Negatives
	Overlaps []Overlap `json:"overlaps"`
}

// Span represents a labeled PII fragment.
type Span struct {
	Id             string  `json:"id"`
	CharacterStart int     `json:"characterStart"`
	CharacterEnd   int     `json:"characterEnd"`
	FilterType     string  `json:"filterType"`
	Context        string  `json:"context"`
	DocumentId     string  `json:"documentId"`
	Confidence     float64 `json:"confidence"`
	Text           string  `json:"text"`
	Replacement    string  `json:"replacement"`
	Salt           string  `json:"salt"`
	Ignored        bool    `json:"ignored"`

	// Compatibility aliases for the audit engine and UI
	Start int    `json:"start"`
	End   int    `json:"end"`
	Label string `json:"label"`
}

// Overlap type
const (
	OverlapExact   = "EXACT"
	OverlapPartial = "PARTIAL"
	OverlapNone    = "NONE"
)

// Overlap describes the relationship between a Philter span and a Golden span.
type Overlap struct {
	Golden Span   `json:"golden"`
	Actual Span   `json:"actual"`
	Type   string `json:"type"` // EXACT, PARTIAL, NONE
}
