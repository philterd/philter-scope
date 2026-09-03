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

// Package suggest turns a measured audit into advisory policy changes.
//
// Recall is the objective. Missing PII is the expensive error, so every
// suggestion that asks for a policy change asks for one that finds more, and
// thresholds are set against recall.
//
// Precision is a guardrail on that advice, not a second objective. Advice
// driven by recall alone points one way forever, since redacting everything
// scores perfect recall, and a report that only measures recall can never say
// the policy has gone too far. So a recall suggestion carries what it is
// expected to cost, and an entity whose precision has collapsed is surfaced as
// a probable misconfiguration. Neither path ever proposes weakening a filter to
// buy precision.
package suggest

import (
	"fmt"
	"os"
	"sort"

	"github.com/philterd/philterscope/internal/ollama"
	"github.com/philterd/philterscope/pkg/model"
)

// DefaultPrecisionFloor is the precision below which an entity is treated as
// likely misconfigured rather than deliberately wide. It is deliberately low:
// a wide net is the point, and only a filter matching several times more than
// it should ought to trip it.
const DefaultPrecisionFloor = 0.25

// Suggester defines the interface for suggesting policy changes.
type Suggester interface {
	Suggest(result model.AuditResult) []model.Recommendation
}

// BasicSuggester implements Suggester with simple threshold logic.
type BasicSuggester struct {
	Threshold        float64
	EntityThresholds map[string]float64
	PrecisionFloor   float64
}

// NewBasicSuggester creates a new BasicSuggester with the default precision floor.
func NewBasicSuggester(threshold float64, entityThresholds map[string]float64) *BasicSuggester {
	return NewBasicSuggesterWithFloor(threshold, entityThresholds, DefaultPrecisionFloor)
}

// NewBasicSuggesterWithFloor creates a BasicSuggester with an explicit precision floor.
func NewBasicSuggesterWithFloor(threshold float64, entityThresholds map[string]float64, precisionFloor float64) *BasicSuggester {
	return &BasicSuggester{
		Threshold:        threshold,
		EntityThresholds: entityThresholds,
		PrecisionFloor:   precisionFloor,
	}
}

// thresholdFor returns the recall threshold an entity is measured against.
func (s *BasicSuggester) thresholdFor(entity string) float64 {
	if t, ok := s.EntityThresholds[entity]; ok {
		return t
	}
	return s.Threshold
}

// Suggest returns recommendations for an audit: a recall gap for every entity
// below its threshold, then a warning for every entity whose precision has
// collapsed. Entities are visited in name order so a report is stable between
// runs over the same data.
func (s *BasicSuggester) Suggest(result model.AuditResult) []model.Recommendation {
	policy := parsePolicy(result.Policy)

	var recs []model.Recommendation

	for _, entity := range sortedKeys(result.EntityMetrics) {
		recall := result.EntityMetrics[entity]
		threshold := s.thresholdFor(entity)
		if recall >= threshold {
			continue
		}
		recs = append(recs, s.recallGap(result, policy, entity, recall, threshold))
	}

	for _, entity := range sortedStatKeys(result.EntityStats) {
		stat := result.EntityStats[entity]
		if stat.TruePositives+stat.FalsePositives == 0 {
			// Nothing was detected for this entity, so it has no precision.
			continue
		}
		if stat.Precision >= s.PrecisionFloor {
			continue
		}
		recs = append(recs, s.precisionCollapse(result, policy, entity, stat))
	}

	return recs
}

// recallGap builds the recommendation for an entity that is missing PII.
func (s *BasicSuggester) recallGap(result model.AuditResult, policy *policyView, entity string, recall float64, threshold float64) model.Recommendation {
	stat, hasStat := result.EntityStats[entity]

	rec := model.Recommendation{
		ID:        recommendationID(model.KindRecallGap, entity),
		Kind:      model.KindRecallGap,
		Entity:    entity,
		Metric:    "recall",
		Value:     recall,
		Threshold: threshold,
		Description: fmt.Sprintf("Recall for %s is %.1f%%, which is below the %.0f%% threshold.",
			entity, recall*100, threshold*100),
		CurrentRecall: floatPtr(recall),
	}
	if hasStat {
		rec.Description += fmt.Sprintf(" %d of %d labeled spans were missed.",
			stat.FalseNegatives, stat.TruePositives+stat.FalseNegatives)
		if stat.TruePositives+stat.FalsePositives > 0 {
			rec.CurrentPrecision = floatPtr(stat.Precision)
		}
	}

	key, filter, present := policy.lookup(entity)

	switch {
	case !policy.known():
		// The policy could not be read, so whether the filter is on is unknown
		// and claiming it is missing would be a guess.
		rec.Action = fmt.Sprintf("Review the %s filter. The policy could not be read, so its current setting is unknown.", entity)
		rec.Change = &model.PolicyChange{Type: model.ChangeReviewFilter, Filter: key}
		rec.Snippet = enableSnippet(key)

	case !present:
		if _, known := filterKey(entity); !known {
			// The gold standard labels this entity with a name that matches no
			// Philter filter. Naming a filter here would invent one, so the
			// recommendation asks for the mapping instead of proposing an edit.
			rec.Action = fmt.Sprintf("Map the %s label to a Philter filter. It matches no filter in the audited policy and no filter type Philter provides, so which filter should cover it cannot be determined from this audit.", entity)
			rec.Change = &model.PolicyChange{Type: model.ChangeReviewFilter}
			rec.Snippet = ""
			break
		}
		rec.Action = fmt.Sprintf("Enable the %s filter. The audited policy has no filter for it.", key)
		rec.Change = &model.PolicyChange{Type: model.ChangeEnableFilter, Filter: key}
		rec.Snippet = enableSnippet(key)

	case !enabled(filter):
		rec.Action = fmt.Sprintf("Enable the %s filter. It is present in the audited policy but switched off.", key)
		rec.Change = &model.PolicyChange{Type: model.ChangeEnableFilter, Filter: key}
		rec.Snippet = enableSnippet(key)

	default:
		// The filter already ran, so the gap is in how it is tuned rather than
		// in whether it is on.
		if current, supported := confidenceThreshold(filter, entity); supported {
			rec.Action = fmt.Sprintf("Lower the confidence threshold for the %s filter, which is already enabled.", key)
			rec.Change = &model.PolicyChange{
				Type:   model.ChangeLowerConfidence,
				Filter: key,
				Field:  "thresholds." + entity,
				From:   floatPtr(current),
			}
			rec.Snippet = thresholdSnippet(key, entity, current)
		} else {
			// The filter is on and takes no threshold, so there is no policy
			// edit to show. An enable snippet here would contradict the action
			// line and tell the reader to switch on what is already running.
			rec.Action = fmt.Sprintf("Review the %s filter, which is already enabled. Widening it needs a model or dictionary change rather than a policy switch.", key)
			rec.Change = &model.PolicyChange{Type: model.ChangeReviewFilter, Filter: key}
			rec.Snippet = ""
		}
	}

	// Nothing here can be predicted. Every option widens what the filter
	// admits, and the spans it would then return are not in this audit.
	rec.Projection = &model.Projection{
		Predicted: false,
		Note:      unpredictedNote(entity, rec.CurrentPrecision),
	}

	return rec
}

// unpredictedNote states the precision cost of a recall change without
// pretending to a number the audit cannot supply.
func unpredictedNote(entity string, currentPrecision *float64) string {
	const cannot = "Widening a filter admits spans this audit never saw, so the effect on precision cannot be computed from this report. Re-run the audit after the change to measure it."
	if currentPrecision == nil {
		return cannot
	}
	return fmt.Sprintf("Precision for %s is %.1f%% today and this change will not raise it. %s",
		entity, *currentPrecision*100, cannot)
}

// precisionCollapse builds the guardrail warning for an entity that is
// redacting far past its golden spans.
//
// It proposes a change only where the sweep finds a confidence cutoff that
// costs no recall against the entity's own target. Where none exists, the
// recommendation reports the problem and stops: it never asks for less
// redaction to make a number look better.
func (s *BasicSuggester) precisionCollapse(result model.AuditResult, policy *policyView, entity string, stat model.EntityStat) model.Recommendation {
	detections := stat.TruePositives + stat.FalsePositives

	rec := model.Recommendation{
		ID:        recommendationID(model.KindPrecisionCollapsed, entity),
		Kind:      model.KindPrecisionCollapsed,
		Entity:    entity,
		Metric:    "precision",
		Value:     stat.Precision,
		Threshold: s.PrecisionFloor,
		Description: fmt.Sprintf("Precision for %s is %.1f%%, below the %.0f%% floor: %d of %d detections did not match a labeled span. A filter matching this far past the gold standard is usually misconfigured rather than deliberately wide.",
			entity, stat.Precision*100, s.PrecisionFloor*100, stat.FalsePositives, detections),
		CurrentPrecision: floatPtr(stat.Precision),
	}
	if stat.TruePositives+stat.FalseNegatives > 0 {
		rec.CurrentRecall = floatPtr(stat.Recall)
	}

	key, filter, present := policy.lookup(entity)

	// The floor is the entity's recall target or its current recall, whichever
	// is higher. Letting a cutoff fall back to the target would trade away
	// recall the policy is already achieving, which is the one thing this must
	// not do: a proposal only survives here if it costs no recall at all.
	recallFloor := s.thresholdFor(entity)
	if stat.Recall > recallFloor {
		recallFloor = stat.Recall
	}

	var sweep *Sweep
	if present {
		if _, supported := confidenceThreshold(filter, entity); supported {
			sweep = SweepConfidence(result, entity, recallFloor)
		}
	}

	if sweep != nil {
		rec.Action = fmt.Sprintf("Raise the confidence threshold for the %s filter to %s. Recall is unchanged at %.1f%%: this drops only detections that matched no labeled span.",
			key, formatFloat(sweep.Cutoff), sweep.Recall*100)
		rec.Change = &model.PolicyChange{
			Type:   model.ChangeRaiseConfidence,
			Filter: key,
			Field:  "thresholds." + entity,
			To:     floatPtr(sweep.Cutoff),
		}
		rec.Snippet = thresholdSnippet(key, entity, sweep.Cutoff)
		rec.Projection = &model.Projection{
			Predicted: true,
			Recall:    floatPtr(sweep.Recall),
			Precision: floatPtr(sweep.Precision),
			Note: fmt.Sprintf("Computed from this audit's own detections: recall %.1f%% (from %.1f%%), precision %.1f%% (from %.1f%%). Raising a cutoff only drops detections already measured, so these are exact for this dataset.",
				sweep.Recall*100, stat.Recall*100, sweep.Precision*100, stat.Precision*100),
		}
		return rec
	}

	// Name the filter only where it is one Philter actually has. An entity the
	// gold standard labels with its own name has no filter to point at, and
	// inventing one would send the reader looking for something absent.
	named := key
	if _, known := filterKey(entity); !present && !known {
		named = ""
	}
	if named == "" {
		rec.Action = fmt.Sprintf("Investigate what is producing %s detections. The label matches no filter in the audited policy, and no confidence threshold narrows it without costing recall, so do not narrow it on these numbers alone.", entity)
	} else {
		rec.Action = fmt.Sprintf("Investigate what the %s filter is matching. No confidence threshold narrows it without costing recall, so do not narrow it on these numbers alone.", named)
	}
	rec.Change = &model.PolicyChange{Type: model.ChangeNone, Filter: named}
	rec.Snippet = ""
	rec.Projection = &model.Projection{
		Predicted: false,
		Note:      "No change is proposed. Recall is the target, and every cutoff that would raise precision here costs recall.",
	}

	return rec
}

// recommendationID is stable for a given kind and entity, so resolving or
// dismissing one recommendation never touches another for the same entity.
func recommendationID(kind string, entity string) string {
	return kind + ":" + entity
}

func floatPtr(f float64) *float64 {
	return &f
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedStatKeys(m map[string]model.EntityStat) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// GetSuggestions prints the recommendations for an audit.
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
		if r.Snippet != "" {
			fmt.Printf("  Snippet:\n%s\n", r.Snippet)
		}
		fmt.Println()
	}
}
