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
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestBasicSuggester_Suggest(t *testing.T) {
	s := NewBasicSuggester(0.5, nil)

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

	if recs[0].IsAI {
		t.Error("Expected IsAI to be false for basic recommendation")
	}
}

func TestBasicSuggester_Suggest_Empty(t *testing.T) {
	s := NewBasicSuggester(0.5, nil)

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
	GetSuggestions(result, 0.5, nil)
}

func TestGetSuggestions_Empty(t *testing.T) {
	result := model.AuditResult{
		EntityMetrics: map[string]float64{
			"NAME": 0.9,
		},
	}
	GetSuggestions(result, 0.5, nil)
}

func TestBasicSuggester_Suggest_EntityThreshold(t *testing.T) {
	s := NewBasicSuggester(0.5, map[string]float64{
		"NAME": 0.9,
	})

	result := model.AuditResult{
		EntityMetrics: map[string]float64{
			"PHONE_NUMBER": 0.4, // Below global threshold (0.5)
			"NAME":         0.8, // Above global threshold (0.5) but below entity threshold (0.9)
			"SSN":          0.6, // Above global threshold (0.5)
		},
	}

	recs := s.Suggest(result)

	if len(recs) != 2 {
		t.Errorf("Expected 2 recommendations, got %d", len(recs))
	}

	foundPhone := false
	foundName := false
	for _, r := range recs {
		if r.Entity == "PHONE_NUMBER" {
			foundPhone = true
		}
		if r.Entity == "NAME" {
			foundName = true
		}
	}

	if !foundPhone {
		t.Error("Expected recommendation for PHONE_NUMBER")
	}
	if !foundName {
		t.Error("Expected recommendation for NAME")
	}
}

// The snippet is now a Philter policy fragment keyed by the policy's own
// identifier name, rather than the "filters" array the earlier version emitted,
// which did not match any schema Philter accepts.
func TestEnableSnippet(t *testing.T) {
	snippet := enableSnippet("creditCard")
	expected := `{
  "identifiers": {
    "creditCard": {
      "enabled": true
    }
  }
}`
	if snippet != expected {
		t.Errorf("Expected snippet:\n%s\nGot:\n%s", expected, snippet)
	}
}
