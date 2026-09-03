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
	"sort"

	"github.com/philterd/philterscope/pkg/model"
)

// Sweep is a confidence cutoff and the metrics the audit's own detections say
// it would produce.
type Sweep struct {
	Cutoff    float64
	Recall    float64
	Precision float64
}

// SweepConfidence finds the highest confidence cutoff for an entity that keeps
// recall at or above recallFloor while improving precision, or nil when no such
// cutoff exists.
//
// Only raising a cutoff can be predicted. Every detection the audit saw is in
// hand, so dropping the ones below a cutoff is exact arithmetic. Lowering a
// cutoff is not: the spans it would admit were filtered out before Philter
// returned anything, so nothing in the report says how many of them are real.
// That asymmetry is why a recall gap never gets a predicted projection and a
// precision warning can.
//
// The recall floor is what keeps this a guardrail. A cutoff that would buy
// precision by dropping below the entity's recall target is not returned at
// all, so the sweep can never propose trading away the metric that matters.
func SweepConfidence(result model.AuditResult, entity string, recallFloor float64) *Sweep {
	stat, ok := result.EntityStats[entity]
	if !ok {
		return nil
	}

	golden := stat.TruePositives + stat.FalseNegatives
	if golden == 0 {
		// No golden spans means no recall to protect, and a cutoff chosen
		// against precision alone is exactly what this must not suggest.
		return nil
	}

	var hitConfidences, missConfidences []float64
	for _, res := range result.Details {
		for _, o := range res.Overlaps {
			// A threshold only moves detections the entity's own filter
			// emitted, so the actual span's label decides what is in scope.
			if spanLabel(o.Actual) != entity {
				continue
			}
			switch o.Type {
			case model.OverlapExact, model.OverlapPartial:
				if spanLabel(o.Golden) == entity {
					hitConfidences = append(hitConfidences, o.Actual.Confidence)
				}
			case model.OverlapNone:
				if o.Actual.Text != "" && o.Actual.CharacterStart != o.Actual.CharacterEnd {
					missConfidences = append(missConfidences, o.Actual.Confidence)
				}
			}
		}
	}

	if len(missConfidences) == 0 {
		// Nothing to drop, so no cutoff improves precision.
		return nil
	}

	// True positives credited to this entity that its own filter did not emit,
	// such as a partial match Philter labeled as something else, are unaffected
	// by its threshold and stay counted at every cutoff.
	unaffected := stat.TruePositives - len(hitConfidences)
	if unaffected < 0 {
		unaffected = 0
	}

	candidates := uniqueSorted(append(append([]float64{}, hitConfidences...), missConfidences...))
	if len(candidates) < 2 {
		// Every detection carries the same confidence, which happens when
		// Philter reports none at all. No cutoff separates them.
		return nil
	}

	// Highest cutoff first, so the first qualifying one is the best available.
	for i := len(candidates) - 1; i >= 0; i-- {
		cutoff := candidates[i]
		keptTP := unaffected + countAtLeast(hitConfidences, cutoff)
		keptFP := countAtLeast(missConfidences, cutoff)

		recall := float64(keptTP) / float64(golden)
		if recall < recallFloor {
			continue
		}

		precision := 0.0
		if keptTP+keptFP > 0 {
			precision = float64(keptTP) / float64(keptTP+keptFP)
		}
		if precision <= stat.Precision {
			continue
		}

		return &Sweep{Cutoff: cutoff, Recall: recall, Precision: precision}
	}

	return nil
}

// spanLabel prefers the golden label and falls back to the filter type, so a
// span labeled either way resolves to the same entity.
func spanLabel(s model.Span) string {
	if s.Label != "" {
		return s.Label
	}
	return s.FilterType
}

func countAtLeast(values []float64, cutoff float64) int {
	n := 0
	for _, v := range values {
		if v >= cutoff {
			n++
		}
	}
	return n
}

func uniqueSorted(values []float64) []float64 {
	sort.Float64s(values)
	out := make([]float64, 0, len(values))
	for i, v := range values {
		if i == 0 || v != values[i-1] {
			out = append(out, v)
		}
	}
	return out
}
