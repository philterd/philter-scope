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
	"encoding/json"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

// policyWith builds a parsed Philter policy from its JSON, the way an audit
// receives one from the Philter API.
func policyWith(t *testing.T, raw string) map[string]any {
	t.Helper()
	var policy map[string]any
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		t.Fatalf("bad test policy: %v", err)
	}
	return policy
}

func findByKind(recs []model.Recommendation, kind string, entity string) *model.Recommendation {
	for i := range recs {
		if recs[i].Kind == kind && recs[i].Entity == entity {
			return &recs[i]
		}
	}
	return nil
}

// A recall gap for an entity the policy has no filter for asks for the filter
// to be enabled, and says plainly that the precision cost is not measured.
func TestSuggest_RecallGap_FilterAbsent(t *testing.T) {
	s := NewBasicSuggester(0.75, nil)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"PHONE_NUMBER": 0.4},
		EntityStats: map[string]model.EntityStat{
			"PHONE_NUMBER": {TruePositives: 2, FalsePositives: 1, FalseNegatives: 3, Recall: 0.4, Precision: 0.667},
		},
		Policy: policyWith(t, `{"identifiers": {"ssn": {"enabled": true}}}`),
	}

	recs := s.Suggest(result)
	rec := findByKind(recs, model.KindRecallGap, "PHONE_NUMBER")
	if rec == nil {
		t.Fatal("expected a recall recommendation for PHONE_NUMBER")
	}

	if rec.Change == nil || rec.Change.Type != model.ChangeEnableFilter {
		t.Fatalf("expected an enable_filter change, got %+v", rec.Change)
	}
	if rec.Change.Filter != "phoneNumber" {
		t.Errorf("expected the policy identifier key phoneNumber, got %q", rec.Change.Filter)
	}
	if rec.Metric != "recall" || rec.Value != 0.4 || rec.Threshold != 0.75 {
		t.Errorf("expected the measurement carried in structured fields, got metric=%s value=%v threshold=%v",
			rec.Metric, rec.Value, rec.Threshold)
	}
	if rec.CurrentPrecision == nil {
		t.Error("expected the entity's current precision to be reported as the cost side")
	}
	if rec.Projection == nil || rec.Projection.Predicted {
		t.Error("widening a filter cannot be predicted from this audit, so the projection must not claim to be")
	}
	if rec.Projection != nil && rec.Projection.Recall != nil {
		t.Error("an unpredicted projection must not carry a recall number")
	}
	if !strings.Contains(rec.Snippet, `"phoneNumber"`) {
		t.Errorf("expected the snippet keyed by the policy identifier, got:\n%s", rec.Snippet)
	}
}

// An entity whose filter is already enabled is not told to enable it again.
func TestSuggest_RecallGap_FilterAlreadyEnabled(t *testing.T) {
	s := NewBasicSuggester(0.75, nil)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"SSN": 0.5},
		EntityStats: map[string]model.EntityStat{
			"SSN": {TruePositives: 1, FalseNegatives: 1, Recall: 0.5},
		},
		Policy: policyWith(t, `{"identifiers": {"ssn": {"enabled": true}}}`),
	}

	rec := findByKind(s.Suggest(result), model.KindRecallGap, "SSN")
	if rec == nil {
		t.Fatal("expected a recall recommendation for SSN")
	}
	if rec.Change != nil && rec.Change.Type == model.ChangeEnableFilter {
		t.Error("a filter that is already enabled must not be suggested as an addition")
	}
	if strings.Contains(strings.ToLower(rec.Action), "enable the") {
		t.Errorf("action should not ask to enable an enabled filter: %s", rec.Action)
	}
}

// A filter present but switched off is a genuine enable.
func TestSuggest_RecallGap_FilterDisabled(t *testing.T) {
	s := NewBasicSuggester(0.75, nil)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"SSN": 0.0},
		EntityStats:   map[string]model.EntityStat{"SSN": {FalseNegatives: 4}},
		Policy:        policyWith(t, `{"identifiers": {"ssn": {"enabled": false}}}`),
	}

	rec := findByKind(s.Suggest(result), model.KindRecallGap, "SSN")
	if rec == nil {
		t.Fatal("expected a recall recommendation for SSN")
	}
	if rec.Change == nil || rec.Change.Type != model.ChangeEnableFilter {
		t.Fatalf("expected an enable_filter change for a disabled filter, got %+v", rec.Change)
	}
}

// An entity redacting far past its labels is flagged, and where a cutoff exists
// that costs no recall it is proposed with exact numbers.
func TestSuggest_PrecisionCollapse_ProposesCutoffThatKeepsRecall(t *testing.T) {
	s := NewBasicSuggesterWithFloor(0.5, nil, 0.5)

	// Two real detections at high confidence, six spurious ones at low
	// confidence. A cutoff of 0.9 keeps both hits and drops every miss.
	details := []model.Result{{
		Overlaps: []model.Overlap{
			{Type: model.OverlapExact, Golden: model.Span{Label: "PERSON", Text: "a"}, Actual: model.Span{Label: "PERSON", Text: "a", Confidence: 0.95}},
			{Type: model.OverlapExact, Golden: model.Span{Label: "PERSON", Text: "b"}, Actual: model.Span{Label: "PERSON", Text: "b", Confidence: 0.9}},
		},
	}}
	for i := 0; i < 6; i++ {
		details[0].Overlaps = append(details[0].Overlaps, model.Overlap{
			Type:   model.OverlapNone,
			Actual: model.Span{Label: "PERSON", Text: "junk", CharacterStart: i, CharacterEnd: i + 3, Confidence: 0.3},
		})
	}

	result := model.AuditResult{
		Details:       details,
		EntityMetrics: map[string]float64{"PERSON": 1.0},
		EntityStats: map[string]model.EntityStat{
			"PERSON": {TruePositives: 2, FalsePositives: 6, FalseNegatives: 0, Recall: 1.0, Precision: 0.25},
		},
		Policy: policyWith(t, `{"identifiers": {"person": {"enabled": true, "thresholds": {"PERSON": 0.1}}}}`),
	}

	rec := findByKind(s.Suggest(result), model.KindPrecisionCollapsed, "PERSON")
	if rec == nil {
		t.Fatal("expected a precision warning for PERSON")
	}
	if rec.Change == nil || rec.Change.Type != model.ChangeRaiseConfidence {
		t.Fatalf("expected a raise_confidence_threshold change, got %+v", rec.Change)
	}
	if rec.Change.To == nil || *rec.Change.To != 0.9 {
		t.Errorf("expected a cutoff of 0.9, got %v", derefOr(rec.Change.To))
	}
	if rec.Projection == nil || !rec.Projection.Predicted {
		t.Fatal("raising a cutoff is computable from the audit, so the projection should be predicted")
	}
	if rec.Projection.Recall == nil || *rec.Projection.Recall != 1.0 {
		t.Errorf("expected recall to be held at 1.0, got %v", derefOr(rec.Projection.Recall))
	}
	if rec.Projection.Precision == nil || *rec.Projection.Precision <= 0.25 {
		t.Errorf("expected precision to improve above 0.25, got %v", derefOr(rec.Projection.Precision))
	}
}

// The guardrail never buys precision with recall: where every cutoff would drop
// recall below the target, no change is proposed at all.
func TestSuggest_PrecisionCollapse_NoCutoffWithoutCostingRecall(t *testing.T) {
	s := NewBasicSuggesterWithFloor(0.9, nil, 0.5)

	// The real detections sit at the same low confidence as the spurious ones,
	// so any cutoff that removes a miss removes a hit too.
	details := []model.Result{{
		Overlaps: []model.Overlap{
			{Type: model.OverlapExact, Golden: model.Span{Label: "PERSON", Text: "a"}, Actual: model.Span{Label: "PERSON", Text: "a", Confidence: 0.3}},
			{Type: model.OverlapNone, Actual: model.Span{Label: "PERSON", Text: "junk", CharacterStart: 1, CharacterEnd: 4, Confidence: 0.9}},
			{Type: model.OverlapNone, Actual: model.Span{Label: "PERSON", Text: "junk", CharacterStart: 5, CharacterEnd: 8, Confidence: 0.9}},
			{Type: model.OverlapNone, Actual: model.Span{Label: "PERSON", Text: "junk", CharacterStart: 9, CharacterEnd: 12, Confidence: 0.9}},
		},
	}}

	result := model.AuditResult{
		Details:       details,
		EntityMetrics: map[string]float64{"PERSON": 1.0},
		EntityStats: map[string]model.EntityStat{
			"PERSON": {TruePositives: 1, FalsePositives: 3, FalseNegatives: 0, Recall: 1.0, Precision: 0.25},
		},
		Policy: policyWith(t, `{"identifiers": {"person": {"enabled": true, "thresholds": {"PERSON": 0.1}}}}`),
	}

	rec := findByKind(s.Suggest(result), model.KindPrecisionCollapsed, "PERSON")
	if rec == nil {
		t.Fatal("expected a precision warning for PERSON")
	}
	if rec.Change == nil || rec.Change.Type != model.ChangeNone {
		t.Fatalf("expected no change to be proposed, got %+v", rec.Change)
	}
	if rec.Snippet != "" {
		t.Errorf("expected no snippet where no change is proposed, got:\n%s", rec.Snippet)
	}
	if rec.Projection == nil || rec.Projection.Predicted {
		t.Error("expected an unpredicted projection where no cutoff qualifies")
	}
}

// Precision on its own never produces a suggestion to weaken a filter: an
// entity above the floor produces nothing at all.
func TestSuggest_PrecisionAboveFloorIsSilent(t *testing.T) {
	s := NewBasicSuggesterWithFloor(0.5, nil, 0.25)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"PERSON": 1.0},
		EntityStats: map[string]model.EntityStat{
			"PERSON": {TruePositives: 4, FalsePositives: 4, Recall: 1.0, Precision: 0.5},
		},
	}

	if recs := s.Suggest(result); len(recs) != 0 {
		t.Errorf("expected no recommendations, got %d: %+v", len(recs), recs)
	}
}

// An audit that meets every target produces nothing.
func TestSuggest_NoGaps(t *testing.T) {
	s := NewBasicSuggesterWithFloor(0.75, nil, 0.25)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"NAME": 0.9, "SSN": 1.0},
		EntityStats: map[string]model.EntityStat{
			"NAME": {TruePositives: 9, FalsePositives: 1, FalseNegatives: 1, Recall: 0.9, Precision: 0.9},
			"SSN":  {TruePositives: 4, Recall: 1.0, Precision: 1.0},
		},
		Policy: policyWith(t, `{"identifiers": {"person": {"enabled": true}}}`),
	}

	if recs := s.Suggest(result); len(recs) != 0 {
		t.Errorf("expected no recommendations, got %d: %+v", len(recs), recs)
	}
}

// One entity can carry both a recall gap and a precision warning, and the two
// must be separately addressable.
func TestSuggest_IDsAreDistinctPerKind(t *testing.T) {
	s := NewBasicSuggesterWithFloor(0.9, nil, 0.5)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"PERSON": 0.5},
		EntityStats: map[string]model.EntityStat{
			"PERSON": {TruePositives: 1, FalsePositives: 4, FalseNegatives: 1, Recall: 0.5, Precision: 0.2},
		},
	}

	recs := s.Suggest(result)
	if len(recs) != 2 {
		t.Fatalf("expected a recall gap and a precision warning, got %d", len(recs))
	}
	if recs[0].ID == recs[1].ID {
		t.Fatalf("both recommendations share the ID %q, so resolving one would hit the other", recs[0].ID)
	}
	for _, r := range recs {
		if r.ID == "" {
			t.Error("every recommendation needs an ID to be addressable")
		}
	}
}

// A policy that could not be read must not produce a claim that a filter is missing.
func TestSuggest_UnreadablePolicyDoesNotClaimFilterMissing(t *testing.T) {
	s := NewBasicSuggester(0.75, nil)

	result := model.AuditResult{
		EntityMetrics: map[string]float64{"SSN": 0.1},
		EntityStats:   map[string]model.EntityStat{"SSN": {TruePositives: 1, FalseNegatives: 9, Recall: 0.1}},
		Policy:        nil,
	}

	rec := findByKind(s.Suggest(result), model.KindRecallGap, "SSN")
	if rec == nil {
		t.Fatal("expected a recall recommendation for SSN")
	}
	if rec.Change == nil || rec.Change.Type != model.ChangeReviewFilter {
		t.Fatalf("expected a review_filter change when the policy is unknown, got %+v", rec.Change)
	}
}

// Suggestions are ordered so two runs over the same data agree.
func TestSuggest_StableOrder(t *testing.T) {
	s := NewBasicSuggester(0.9, nil)
	result := model.AuditResult{
		EntityMetrics: map[string]float64{"SSN": 0.1, "NAME": 0.2, "DATE": 0.3, "URL": 0.4},
	}

	want := []string{"DATE", "NAME", "SSN", "URL"}
	for i := 0; i < 5; i++ {
		recs := s.Suggest(result)
		for j, r := range recs {
			if r.Entity != want[j] {
				t.Fatalf("run %d position %d: expected %s, got %s", i, j, want[j], r.Entity)
			}
		}
	}
}

func derefOr(f *float64) float64 {
	if f == nil {
		return -1
	}
	return *f
}
