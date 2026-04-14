package model

import "time"

// Redactor defines the interface for redaction services.
type Redactor interface {
	Redact(text string) (string, []Span, error)
}

// AuditResult holds the outcome of an auditing run.
type AuditResult struct {
	Timestamp       time.Time              `json:"timestamp"`
	TotalDocuments  int                    `json:"total_documents"`
	Precision       float64                `json:"precision"`
	Recall          float64                `json:"recall"`
	F1Score         float64                `json:"f1_score"`
	Details         []Result               `json:"details"`
	Policy          map[string]interface{} `json:"policy"`          // Philter JSON configuration
	Recommendations []Recommendation       `json:"recommendations"` // Suggested policy changes
	EntityMetrics   map[string]float64     `json:"entity_metrics"`  // Recall per entity type
}

// HistoryEntry represents a past audit result.
type HistoryEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Precision float64                `json:"precision"`
	Recall    float64                `json:"recall"`
	F1Score   float64                `json:"f1_score"`
	Policy    map[string]interface{} `json:"policy"`
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
	Text  string `json:"text"`
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
