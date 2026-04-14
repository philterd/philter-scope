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
			} else if o.Actual.Text != "" {
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

	entityMetrics := CalculateEntityMetrics(results)

	return model.AuditResult{
		TotalDocuments: len(results),
		Precision:      precision,
		Recall:         recall,
		F1Score:        f1,
		Details:        results,
		EntityMetrics:  entityMetrics,
	}
}

// CalculateEntityMetrics calculates recall per entity type.
func CalculateEntityMetrics(results []model.Result) map[string]float64 {
	tpCount := make(map[string]int)
	fnCount := make(map[string]int)

	for _, res := range results {
		for _, o := range res.Overlaps {
			label := o.Golden.Label
			if label == "" {
				continue
			}
			switch o.Type {
			case model.OverlapExact, model.OverlapPartial:
				tpCount[label]++
			case model.OverlapNone:
				if o.Golden.Text != "" {
					fnCount[label]++
				}
			}
		}
	}

	metrics := make(map[string]float64)
	for label, tp := range tpCount {
		fn := fnCount[label]
		metrics[label] = float64(tp) / float64(tp+fn)
	}
	// Also include entities that only had FNs
	for label := range fnCount {
		if _, ok := tpCount[label]; !ok {
			metrics[label] = 0.0
		}
	}

	return metrics
}
