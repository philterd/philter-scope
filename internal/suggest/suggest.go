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

package suggest

import (
	"fmt"
	"os"

	"github.com/philterd/philterscope/internal/ollama"
	"github.com/philterd/philterscope/pkg/model"
)

// Suggester defines the interface for suggesting policy changes.
type Suggester interface {
	Suggest(result model.AuditResult) []model.Recommendation
}

// BasicSuggester implements Suggester with simple threshold logic.
type BasicSuggester struct {
	Threshold        float64
	EntityThresholds map[string]float64
}

// NewBasicSuggester creates a new BasicSuggester.
func NewBasicSuggester(threshold float64, entityThresholds map[string]float64) *BasicSuggester {
	return &BasicSuggester{
		Threshold:        threshold,
		EntityThresholds: entityThresholds,
	}
}

// Suggest returns recommendations based on recall thresholds.
func (s *BasicSuggester) Suggest(result model.AuditResult) []model.Recommendation {
	var recs []model.Recommendation

	for entity, recall := range result.EntityMetrics {
		threshold := s.Threshold
		if et, ok := s.EntityThresholds[entity]; ok {
			threshold = et
		}

		if recall < threshold {
			recs = append(recs, model.Recommendation{
				Entity:      entity,
				Description: fmt.Sprintf("Recall for %s is %.1f%%, which is below the %.0f%% threshold.", entity, recall*100, threshold*100),
				Action:      fmt.Sprintf("Enable or strengthen the %s filter.", entity),
				Snippet:     generateSnippet(entity),
				IsAI:        false,
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

// GetSuggestions is a helper for the suggest command.
func GetSuggestions(result model.AuditResult, threshold float64, entityThresholds map[string]float64) {
	s := NewBasicSuggester(threshold, entityThresholds)
	recs := s.Suggest(result)

	// Check for Ollama configuration
	if os.Getenv("PHILTERSCOPE_OLLAMA_URL") != "" {
		fmt.Println("Generating AI recommendations...")
		client := ollama.NewClient()
		ls := NewLLMSuggester(client)
		aiRecs := ls.Suggest(result)
		recs = append(recs, aiRecs...)
	}

	if len(recs) == 0 {
		fmt.Println("No suggestions.")
		return
	}

	fmt.Println("Recommended Policy Actions:")
	for _, r := range recs {
		fmt.Printf("- %s: %s\n", r.Entity, r.Description)
		fmt.Printf("  Action: %s\n", r.Action)
		fmt.Printf("  Snippet:\n%s\n\n", r.Snippet)
	}
}
