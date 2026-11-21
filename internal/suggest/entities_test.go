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

	"github.com/philterd/philterscope/internal/audit"
	"github.com/philterd/philterscope/pkg/model"
)

// namesFoundAsPerson is the case from philterd/philter-scope#18: nine names the
// gold standard labels NAME, found by Philter as PERSON, and one stray PERSON.
func namesFoundAsPerson() []model.Result {
	var overlaps []model.Overlap
	for i := 0; i < 9; i++ {
		overlaps = append(overlaps, model.Overlap{
			Type:   model.OverlapExact,
			Golden: model.Span{Label: "NAME", Text: "n", CharacterStart: i * 10, CharacterEnd: i*10 + 1},
			Actual: model.Span{FilterType: "PERSON", Text: "n", CharacterStart: i * 10, CharacterEnd: i*10 + 1, Confidence: 0.9},
		})
	}
	overlaps = append(overlaps, model.Overlap{
		Type:   model.OverlapNone,
		Actual: model.Span{FilterType: "PERSON", Text: "x", CharacterStart: 100, CharacterEnd: 101, Confidence: 0.4},
	})
	return []model.Result{{Overlaps: overlaps}}
}

// A filter matching nine of the ten spans it found is not flagged as
// misconfigured just because the gold standard spells its entity differently.
func TestSuggest_EntitySpelledDifferentlyIsNotAPrecisionCollapse(t *testing.T) {
	result := audit.GenerateAuditResult(namesFoundAsPerson())
	result.Policy = policyWith(t, `{"identifiers": {"person": {"enabled": true}}}`)

	recs := NewBasicSuggester(0.75, nil).Suggest(result)

	for _, rec := range recs {
		if rec.Kind == model.KindPrecisionCollapsed {
			t.Errorf("expected no precision warning, got %+v", rec)
		}
	}
	if stat := result.EntityStats["person"]; stat.Precision != 0.9 {
		t.Errorf("expected person precision 0.9, got %+v", result.EntityStats)
	}
}

// A per-entity threshold given for the gold standard's label still applies once
// the report names the entity by its policy key.
func TestSuggest_ThresholdGivenForALabelAppliesToTheEntity(t *testing.T) {
	results := namesFoundAsPerson()
	results[0].Overlaps = append(results[0].Overlaps, model.Overlap{
		Type:   model.OverlapNone,
		Golden: model.Span{Label: "NAME", Text: "m", CharacterStart: 120, CharacterEnd: 121},
	})
	result := audit.GenerateAuditResult(results)
	result.Policy = policyWith(t, `{"identifiers": {"person": {"enabled": true}}}`)

	// Recall is 0.9, under 0.95 but over the default of 0.5, so a recall gap
	// is raised only if the NAME threshold applies.
	s := NewBasicSuggester(0.5, map[string]float64{"NAME": 0.95})
	if rec := findByKind(s.Suggest(result), model.KindRecallGap, "person"); rec == nil {
		t.Error("expected the NAME threshold to apply to person")
	}
}

// A sweep over a report saved with labels, before entities were named by
// policy key, still finds the spans of that entity.
func TestSweepConfidence_ReportKeyedByLabel(t *testing.T) {
	result := model.AuditResult{
		Details: namesFoundAsPerson(),
		EntityStats: map[string]model.EntityStat{
			"NAME": {TruePositives: 9, FalsePositives: 1, Recall: 1.0, Precision: 0.9},
		},
	}

	sweep := SweepConfidence(result, "NAME", 1.0)
	if sweep == nil || sweep.Precision != 1.0 {
		t.Fatalf("expected a cutoff that drops the stray span, got %+v", sweep)
	}
}
