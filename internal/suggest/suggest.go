package suggest

import (
	"fmt"

	"github.com/philterd/philterscope/pkg/model"
)

// Suggester defines the interface for suggesting policy changes.
type Suggester interface {
	Suggest(result model.AuditResult) []model.Recommendation
}

// BasicSuggester implements Suggester with simple threshold logic.
type BasicSuggester struct {
	Threshold float64
}

// NewBasicSuggester creates a new BasicSuggester.
func NewBasicSuggester(threshold float64) *BasicSuggester {
	return &BasicSuggester{Threshold: threshold}
}

// Suggest returns recommendations based on recall thresholds.
func (s *BasicSuggester) Suggest(result model.AuditResult) []model.Recommendation {
	var recs []model.Recommendation

	for entity, recall := range result.EntityMetrics {
		if recall < s.Threshold {
			recs = append(recs, model.Recommendation{
				Entity:      entity,
				Description: fmt.Sprintf("Recall for %s is %.1f%%, which is below the %.0f%% threshold.", entity, recall*100, s.Threshold*100),
				Action:      fmt.Sprintf("Enable or strengthen the %s filter.", entity),
				Snippet:     generateSnippet(entity),
			})
		}
	}

	return recs
}

// generateSnippet returns a pre-defined JSON snippet for the entity.
func generateSnippet(entity string) string {
	return fmt.Sprintf(`{
  "filters": [
    {
      "filterType": "%s",
      "strategy": "REDACT"
    }
  ]
}`, entity)
}

// GetSuggestions is a helper for the suggest command (existing from previous work).
func GetSuggestions(result model.AuditResult) {
	s := NewBasicSuggester(0.5)
	recs := s.Suggest(result)
	if len(recs) == 0 {
		fmt.Println("No suggestions. Philter is performing well!")
		return
	}

	fmt.Println("Recommended Policy Actions:")
	for _, r := range recs {
		fmt.Printf("- %s: %s\n", r.Entity, r.Description)
		fmt.Printf("  Action: %s\n", r.Action)
		fmt.Printf("  Snippet:\n%s\n\n", r.Snippet)
	}
}
