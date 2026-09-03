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
	"github.com/philterd/philterscope/pkg/model"
)

// CalculateMetricsByOverlap calculates precision, recall, and f1-score based on overlaps.
func CalculateMetricsByOverlap(overlaps []model.Overlap) (tp, fp, fn int) {
	for _, o := range overlaps {
		switch o.Type {
		case model.OverlapExact:
			tp++
		case model.OverlapPartial:
			// Partial can be treated as TP for some metrics, or a fraction.
			// For simplicity, let's count it as TP but maybe we want to be stricter.
			// The requirement just says "detecting if ... matched exactly, partially, or not at all".
			tp++
		case model.OverlapNone:
			if o.Golden.Text != "" {
				fn++ // Golden existed but not matched -> False Negative
			} else if o.Actual.Text != "" && o.Actual.CharacterStart != o.Actual.CharacterEnd {
				fp++ // Actual existed but no Golden -> False Positive
			}
		}
	}
	return tp, fp, fn
}

// GenerateAuditResult compiles the full metrics for a run.
func GenerateAuditResult(results []model.Result) model.AuditResult {
	var totalTP, totalFP, totalFN int
	for _, res := range results {
		totalTP += res.TP
		totalFP += res.FP
		totalFN += res.FN
	}

	precision := 0.0
	if totalTP+totalFP > 0 {
		precision = float64(totalTP) / float64(totalTP+totalFP)
	}

	recall := 0.0
	if totalTP+totalFN > 0 {
		recall = float64(totalTP) / float64(totalTP+totalFN)
	}

	f1 := 0.0
	if precision+recall > 0 {
		f1 = 2 * (precision * recall) / (precision + recall)
	}

	entityStats := CalculateEntityStats(results)
	entityMetrics := CalculateEntityMetrics(results)
	confusionMatrix := CalculateConfusionMatrix(results)

	return model.AuditResult{
		TotalDocuments:  len(results),
		Precision:       precision,
		Recall:          recall,
		F1Score:         f1,
		Details:         results,
		EntityMetrics:   entityMetrics,
		EntityStats:     entityStats,
		ConfusionMatrix: confusionMatrix,
	}
}

// CalculateConfusionMatrix builds a matrix of expected label -> actual label -> count.
// For missed entities (FN), the actual label is "(missed)".
// For spurious detections (FP), the expected label is "(none)".
func CalculateConfusionMatrix(results []model.Result) map[string]map[string]int {
	matrix := make(map[string]map[string]int)

	ensure := func(key string) {
		if matrix[key] == nil {
			matrix[key] = make(map[string]int)
		}
	}

	for _, res := range results {
		for _, o := range res.Overlaps {
			switch o.Type {
			case model.OverlapExact, model.OverlapPartial:
				expected := entityLabel(o.Golden)
				actual := entityLabel(o.Actual)
				if expected == "" {
					continue
				}
				if actual == "" {
					actual = expected
				}
				ensure(expected)
				matrix[expected][actual]++
			case model.OverlapNone:
				if o.Golden.Text != "" {
					expected := entityLabel(o.Golden)
					if expected == "" {
						continue
					}
					ensure(expected)
					matrix[expected]["(missed)"]++
				} else if o.Actual.Text != "" && o.Actual.CharacterStart != o.Actual.CharacterEnd {
					actual := entityLabel(o.Actual)
					if actual == "" {
						continue
					}
					ensure("(none)")
					matrix["(none)"][actual]++
				}
			}
		}
	}

	return matrix
}

func entityLabel(s model.Span) string {
	if s.Label != "" {
		return s.Label
	}
	return s.FilterType
}

// CalculateEntityStats counts true positives, false positives, and false
// negatives per entity type and derives both rates from them.
//
// False positives are attributed to the label of the span Philter produced,
// since a spurious detection has no golden span to take a label from. That is
// the only place an entity can appear with detections but no golden spans.
func CalculateEntityStats(results []model.Result) map[string]model.EntityStat {
	tpCount := make(map[string]int)
	fpCount := make(map[string]int)
	fnCount := make(map[string]int)
	labels := make(map[string]struct{})

	note := func(counts map[string]int, label string) {
		if label == "" {
			return
		}
		counts[label]++
		labels[label] = struct{}{}
	}

	for _, res := range results {
		for _, o := range res.Overlaps {
			switch o.Type {
			case model.OverlapExact, model.OverlapPartial:
				// A partial counts as found, matching CalculateMetricsByOverlap,
				// and is credited to the golden label even where Philter gave
				// the span a different one.
				note(tpCount, entityLabel(o.Golden))
			case model.OverlapNone:
				if o.Golden.Text != "" {
					note(fnCount, entityLabel(o.Golden))
				} else if o.Actual.Text != "" && o.Actual.CharacterStart != o.Actual.CharacterEnd {
					note(fpCount, entityLabel(o.Actual))
				}
			}
		}
	}

	stats := make(map[string]model.EntityStat, len(labels))
	for label := range labels {
		tp, fp, fn := tpCount[label], fpCount[label], fnCount[label]
		stat := model.EntityStat{
			TruePositives:  tp,
			FalsePositives: fp,
			FalseNegatives: fn,
		}
		if tp+fn > 0 {
			stat.Recall = float64(tp) / float64(tp+fn)
		}
		if tp+fp > 0 {
			stat.Precision = float64(tp) / float64(tp+fp)
		}
		stats[label] = stat
	}

	return stats
}

// CalculateEntityMetrics calculates recall per entity type.
//
// An entity with detections but no golden spans has no recall to report and is
// left out, so a caller iterating this map is iterating entities the gold
// standard actually covers. CalculateEntityStats carries the rest.
func CalculateEntityMetrics(results []model.Result) map[string]float64 {
	metrics := make(map[string]float64)
	for label, stat := range CalculateEntityStats(results) {
		if stat.TruePositives+stat.FalseNegatives > 0 {
			metrics[label] = stat.Recall
		}
	}
	return metrics
}
