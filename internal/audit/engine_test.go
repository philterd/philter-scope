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

package audit

import (
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestCalculateMetricsByOverlap(t *testing.T) {
	overlaps := []model.Overlap{
		{Type: model.OverlapExact},
		{Type: model.OverlapExact},
		{Type: model.OverlapPartial},
		{Type: model.OverlapNone, Golden: model.Span{Text: "Name"}}, // FN
		{Type: model.OverlapNone, Actual: model.Span{Text: "Key", CharacterStart: 10, CharacterEnd: 13}}, // FP
	}

	tp, fp, fn := CalculateMetricsByOverlap(overlaps)

	if tp != 3 { // 2 exact + 1 partial
		t.Errorf("Expected TP 3, got %d", tp)
	}
	if fp != 1 {
		t.Errorf("Expected FP 1, got %d", fp)
	}
	if fn != 1 {
		t.Errorf("Expected FN 1, got %d", fn)
	}
}

func TestGenerateAuditResult(t *testing.T) {
	results := []model.Result{
		{
			TP: 8,
			FP: 2,
			FN: 2,
			Overlaps: []model.Overlap{
				{Type: model.OverlapExact, Golden: model.Span{Label: "NAME", Text: "John"}},
			},
		},
	}

	res := GenerateAuditResult(results)

	if res.TotalDocuments != 1 {
		t.Errorf("Expected 1 document, got %d", res.TotalDocuments)
	}
	if res.Precision != 0.8 {
		t.Errorf("Expected Precision 0.8, got %f", res.Precision)
	}
	if res.Recall != 0.8 {
		t.Errorf("Expected Recall 0.8, got %f", res.Recall)
	}
	if res.EntityMetrics["person"] != 1.0 {
		t.Errorf("Expected Recall for person 1.0, got %f", res.EntityMetrics["person"])
	}
}

func TestCalculateConfusionMatrix(t *testing.T) {
	results := []model.Result{
		{
			Overlaps: []model.Overlap{
				// Correct: NAME detected as NAME
				{Type: model.OverlapExact, Golden: model.Span{Label: "NAME", Text: "John"}, Actual: model.Span{Label: "NAME", Text: "John"}},
				// Wrong type: NAME detected as ADDRESS
				{Type: model.OverlapPartial, Golden: model.Span{Label: "NAME", Text: "Jane"}, Actual: model.Span{Label: "ADDRESS", Text: "Jane"}},
				// Missed: SSN not detected
				{Type: model.OverlapNone, Golden: model.Span{Label: "SSN", Text: "123-45-6789"}},
				// False positive: spurious PHONE detection
				{Type: model.OverlapNone, Actual: model.Span{Label: "PHONE", Text: "555", CharacterStart: 10, CharacterEnd: 13}},
			},
		},
	}

	cm := CalculateConfusionMatrix(results)

	if cm["person"]["person"] != 1 {
		t.Errorf("Expected person->person=1, got %d", cm["person"]["person"])
	}
	if cm["person"]["streetAddress"] != 1 {
		t.Errorf("Expected person->streetAddress=1, got %d", cm["person"]["streetAddress"])
	}
	if cm["ssn"]["(missed)"] != 1 {
		t.Errorf("Expected ssn->(missed)=1, got %d", cm["ssn"]["(missed)"])
	}
	if cm["(none)"]["phoneNumber"] != 1 {
		t.Errorf("Expected (none)->phoneNumber=1, got %d", cm["(none)"]["phoneNumber"])
	}
}

func TestCalculateEntityMetrics(t *testing.T) {
	results := []model.Result{
		{
			Overlaps: []model.Overlap{
				{Type: model.OverlapExact, Golden: model.Span{Label: "PHONE", Text: "123"}},
				{Type: model.OverlapNone, Golden: model.Span{Label: "PHONE", Text: "456"}}, // FN
			},
		},
	}

	metrics := CalculateEntityMetrics(results)

	if metrics["phoneNumber"] != 0.5 {
		t.Errorf("Expected Recall for phoneNumber 0.5, got %f", metrics["phoneNumber"])
	}
}

func TestCalculateEntityStats(t *testing.T) {
	results := []model.Result{
		{
			Overlaps: []model.Overlap{
				// PHONE: one found, one missed, one spurious.
				{Type: model.OverlapExact, Golden: model.Span{Label: "PHONE", Text: "123"}, Actual: model.Span{Label: "PHONE", Text: "123"}},
				{Type: model.OverlapNone, Golden: model.Span{Label: "PHONE", Text: "456"}},
				{Type: model.OverlapNone, Actual: model.Span{Label: "PHONE", Text: "789", CharacterStart: 10, CharacterEnd: 13}},
			},
		},
	}

	stats := CalculateEntityStats(results)

	phone := stats["phoneNumber"]
	if phone.TruePositives != 1 || phone.FalseNegatives != 1 || phone.FalsePositives != 1 {
		t.Fatalf("expected 1/1/1 for PHONE, got tp=%d fn=%d fp=%d",
			phone.TruePositives, phone.FalseNegatives, phone.FalsePositives)
	}
	if phone.Recall != 0.5 {
		t.Errorf("expected PHONE recall 0.5, got %f", phone.Recall)
	}
	if phone.Precision != 0.5 {
		t.Errorf("expected PHONE precision 0.5, got %f", phone.Precision)
	}
}

// An entity Philter detected but the gold standard never labels has a precision
// and no recall. It used to vanish from the report entirely, which is how an
// over-matching filter stayed invisible.
func TestCalculateEntityStats_FalsePositivesOnly(t *testing.T) {
	results := []model.Result{
		{
			Overlaps: []model.Overlap{
				{Type: model.OverlapNone, Actual: model.Span{Label: "URL", Text: "a", CharacterStart: 0, CharacterEnd: 1}},
				{Type: model.OverlapNone, Actual: model.Span{Label: "URL", Text: "b", CharacterStart: 2, CharacterEnd: 3}},
			},
		},
	}

	stats := CalculateEntityStats(results)
	url, ok := stats["url"]
	if !ok {
		t.Fatal("expected url to appear in the stats on false positives alone")
	}
	if url.FalsePositives != 2 || url.Precision != 0 {
		t.Errorf("expected 2 false positives and zero precision, got fp=%d precision=%f",
			url.FalsePositives, url.Precision)
	}

	// It has no recall to report, so it stays out of the recall map.
	if _, present := CalculateEntityMetrics(results)["url"]; present {
		t.Error("an entity with no golden spans has no recall and should not appear in entity_metrics")
	}
}

// A span carrying only a filter type is labeled the same way everywhere, so the
// stats and the confusion matrix agree on what an entity is called.
func TestCalculateEntityStats_FallsBackToFilterType(t *testing.T) {
	results := []model.Result{
		{
			Overlaps: []model.Overlap{
				{Type: model.OverlapExact, Golden: model.Span{FilterType: "SSN", Text: "1"}, Actual: model.Span{FilterType: "SSN", Text: "1"}},
			},
		},
	}

	if stats := CalculateEntityStats(results); stats["ssn"].TruePositives != 1 {
		t.Errorf("expected the filter type to be used as the label, got %+v", stats)
	}
}

// A gold standard and Philter spelling one filter differently are scored as
// one entity, under the policy key. Nine names labeled NAME and found as
// PERSON, plus one stray PERSON, are a person filter with precision 9 of 10,
// not a NAME row at 1.0 and a PERSON row at 0.0 (#18).
func TestEntitiesSpelledDifferentlyAreScoredTogether(t *testing.T) {
	var overlaps []model.Overlap
	for i := 0; i < 9; i++ {
		overlaps = append(overlaps, model.Overlap{
			Type:   model.OverlapExact,
			Golden: model.Span{Label: "NAME", Text: "n", CharacterStart: i * 10, CharacterEnd: i*10 + 1},
			Actual: model.Span{FilterType: "PERSON", Text: "n", CharacterStart: i * 10, CharacterEnd: i*10 + 1},
		})
	}
	overlaps = append(overlaps,
		model.Overlap{Type: model.OverlapNone, Actual: model.Span{FilterType: "PERSON", Text: "x", CharacterStart: 100, CharacterEnd: 101}},
		// A label that names no Philter filter keeps its spelling.
		model.Overlap{Type: model.OverlapNone, Golden: model.Span{Label: "LOCATION", Text: "y", CharacterStart: 110, CharacterEnd: 111}},
	)
	results := []model.Result{{Overlaps: overlaps}}

	stats := CalculateEntityStats(results)
	person := stats["person"]
	if person.TruePositives != 9 || person.FalsePositives != 1 || person.FalseNegatives != 0 {
		t.Fatalf("expected person tp=9 fp=1 fn=0, got %+v", person)
	}
	if person.Precision != 0.9 || person.Recall != 1.0 {
		t.Errorf("expected person precision 0.9 and recall 1.0, got %+v", person)
	}
	for _, label := range []string{"NAME", "PERSON"} {
		if _, ok := stats[label]; ok {
			t.Errorf("expected no separate %s row, got %+v", label, stats)
		}
	}
	if stats["LOCATION"].FalseNegatives != 1 {
		t.Errorf("expected LOCATION to keep its name, got %+v", stats)
	}

	cm := CalculateConfusionMatrix(results)
	if cm["person"]["person"] != 9 || cm["(none)"]["person"] != 1 || cm["LOCATION"]["(missed)"] != 1 {
		t.Errorf("unexpected confusion matrix %+v", cm)
	}
	if len(cm["person"]) != 1 {
		t.Errorf("expected every person hit on the diagonal, got %+v", cm["person"])
	}
}
