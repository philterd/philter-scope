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
	if res.EntityMetrics["NAME"] != 1.0 {
		t.Errorf("Expected Recall for NAME 1.0, got %f", res.EntityMetrics["NAME"])
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

	if cm["NAME"]["NAME"] != 1 {
		t.Errorf("Expected NAME->NAME=1, got %d", cm["NAME"]["NAME"])
	}
	if cm["NAME"]["ADDRESS"] != 1 {
		t.Errorf("Expected NAME->ADDRESS=1, got %d", cm["NAME"]["ADDRESS"])
	}
	if cm["SSN"]["(missed)"] != 1 {
		t.Errorf("Expected SSN->(missed)=1, got %d", cm["SSN"]["(missed)"])
	}
	if cm["(none)"]["PHONE"] != 1 {
		t.Errorf("Expected (none)->PHONE=1, got %d", cm["(none)"]["PHONE"])
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

	if metrics["PHONE"] != 0.5 {
		t.Errorf("Expected Recall for PHONE 0.5, got %f", metrics["PHONE"])
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

	phone := stats["PHONE"]
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
	url, ok := stats["URL"]
	if !ok {
		t.Fatal("expected URL to appear in the stats on false positives alone")
	}
	if url.FalsePositives != 2 || url.Precision != 0 {
		t.Errorf("expected 2 false positives and zero precision, got fp=%d precision=%f",
			url.FalsePositives, url.Precision)
	}

	// It has no recall to report, so it stays out of the recall map.
	if _, present := CalculateEntityMetrics(results)["URL"]; present {
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

	if stats := CalculateEntityStats(results); stats["SSN"].TruePositives != 1 {
		t.Errorf("expected the filter type to be used as the label, got %+v", stats)
	}
}
