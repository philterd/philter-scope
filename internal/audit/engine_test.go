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
