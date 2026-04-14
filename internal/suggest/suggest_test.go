package suggest

import (
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestBasicSuggester_Suggest(t *testing.T) {
	s := NewBasicSuggester(0.5)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{
			"PHONE_NUMBER": 0.4, // Below threshold
			"NAME":         0.8, // Above threshold
		},
	}

	recs := s.Suggest(result)

	if len(recs) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(recs))
	}

	if recs[0].Entity != "PHONE_NUMBER" {
		t.Errorf("Expected recommendation for PHONE_NUMBER, got %s", recs[0].Entity)
	}
}

func TestBasicSuggester_Suggest_Empty(t *testing.T) {
	s := NewBasicSuggester(0.5)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{
			"NAME": 0.9,
		},
	}

	recs := s.Suggest(result)

	if len(recs) != 0 {
		t.Errorf("Expected 0 recommendations, got %d", len(recs))
	}
}

func TestGetSuggestions(t *testing.T) {
	result := model.AuditResult{
		EntityMetrics: map[string]float64{
			"PHONE_NUMBER": 0.4,
		},
	}
	// This function prints to stdout, so we just check it doesn't panic
	// and maybe later capture stdout if needed.
	GetSuggestions(result)
}

func TestGetSuggestions_Empty(t *testing.T) {
	result := model.AuditResult{
		EntityMetrics: map[string]float64{
			"NAME": 0.9,
		},
	}
	GetSuggestions(result)
}

func TestGenerateSnippet(t *testing.T) {
	snippet := generateSnippet("CREDIT_CARD")
	expected := `{
  "filters": [
    {
      "filterType": "CREDIT_CARD",
      "strategy": "REDACT"
    }
  ]
}`
	if snippet != expected {
		t.Errorf("Expected snippet:\n%s\nGot:\n%s", expected, snippet)
	}
}
