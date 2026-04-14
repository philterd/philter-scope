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

// CalculateOverlap detects if Philter's redaction matched the Golden label exactly, partially, or not at all.
func CalculateOverlap(golden []model.Span, actual []model.Span) []model.Overlap {
	var overlaps []model.Overlap

	for _, g := range golden {
		foundMatch := false
		for _, a := range actual {
			if g.Start == a.Start && g.End == a.End {
				overlaps = append(overlaps, model.Overlap{
					Golden: g,
					Actual: a,
					Type:   model.OverlapExact,
				})
				foundMatch = true
				break
			}

			// Check for partial overlap
			// (StartA < EndB) and (EndA > StartB)
			if g.Start < a.End && g.End > a.Start {
				overlaps = append(overlaps, model.Overlap{
					Golden: g,
					Actual: a,
					Type:   model.OverlapPartial,
				})
				foundMatch = true
				// We don't break here to find ALL partial overlaps if one golden span matches multiple actual spans
				// But typically it's one-to-one or one-to-many. For metrics, we need to be careful.
			}
		}

		if !foundMatch {
			overlaps = append(overlaps, model.Overlap{
				Golden: g,
				Type:   model.OverlapNone,
			})
		}
	}

	// Also check for False Positives: Philter redacted something that wasn't in Golden
	for _, a := range actual {
		foundMatch := false
		for _, g := range golden {
			if a.Start < g.End && a.End > g.Start {
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			overlaps = append(overlaps, model.Overlap{
				Actual: a,
				Type:   model.OverlapNone, // This is a False Positive
			})
		}
	}

	return overlaps
}
