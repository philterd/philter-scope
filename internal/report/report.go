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

package report

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"slices"
	"sort"

	"github.com/philterd/philterscope/pkg/model"
)

//go:embed index.html
var staticAssets embed.FS

// Template parses the evaluation UI page. The serving binary renders it with no data so the page
// loads results through the API; GenerateStandaloneReport renders it with one result embedded.
func Template() (*template.Template, error) {
	return template.ParseFS(staticAssets, "index.html")
}

// GenerateStandaloneReport creates a self-contained HTML file.
func GenerateStandaloneReport(result model.AuditResult, privacyMode bool) (string, error) {
	tmpl, err := Template()
	if err != nil {
		return "", err
	}

	if privacyMode {
		result = ObfuscateAuditResult(result)
	}

	reportJSON, _ := json.Marshal(result)
	data := struct {
		ReportJSON template.JS
	}{
		ReportJSON: template.JS(reportJSON),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ObfuscateAuditResult redacts PII from the audit result.
func ObfuscateAuditResult(res model.AuditResult) model.AuditResult {
	for i := range res.Details {
		res.Details[i] = obfuscateResult(res.Details[i])
	}
	return res
}

func hashText(text string) string {
	if text == "" {
		return ""
	}
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])[:5]
}

func obfuscateResult(r model.Result) model.Result {
	// Collect all spans that might contain PII and need redaction in text
	type redaction struct {
		start, end  int
		replacement string
	}
	var redactions []redaction

	collectRedactions := func(spans []model.Span) {
		for _, s := range spans {
			if s.CharacterStart < s.CharacterEnd {
				redactions = append(redactions, redaction{
					start:       s.CharacterStart,
					end:         s.CharacterEnd,
					replacement: hashText(s.Text),
				})
			}
		}
	}

	collectRedactions(r.Spans)
	for _, o := range r.Overlaps {
		if o.Golden.Text != "" {
			collectRedactions([]model.Span{o.Golden})
		}
		if o.Actual.Text != "" {
			collectRedactions([]model.Span{o.Actual})
		}
	}

	// Sort and merge overlapping redactions
	slices.SortFunc(redactions, func(a, b redaction) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})

	var merged []redaction
	if len(redactions) > 0 {
		curr := redactions[0]
		for i := 1; i < len(redactions); i++ {
			if redactions[i].start < curr.end {
				// Overlap
				if redactions[i].end > curr.end {
					curr.end = redactions[i].end
					// We need a replacement for the merged span.
					// Using hash of the original merged text would be best but we don't have it easily here.
					// Let's just keep the first replacement or re-calculate if we had the text.
				}
			} else {
				merged = append(merged, curr)
				curr = redactions[i]
			}
		}
		merged = append(merged, curr)
	}

	// Function to apply redactions and update spans
	apply := func(text string) (string, func(int) int) {
		var buf bytes.Buffer
		lastIdx := 0

		type shift struct {
			origIdx int
			newIdx  int
		}
		shifts := []shift{{0, 0}}

		for _, red := range merged {
			if red.start < lastIdx {
				continue
			} // Should not happen with merged
			if red.start >= len(text) {
				break
			}

			buf.WriteString(text[lastIdx:red.start])

			// If we didn't have the text for the merged span before, we can get it now to hash it properly
			actualReplacement := red.replacement
			if red.end > len(text) {
				red.end = len(text)
			}
			actualReplacement = hashText(text[red.start:red.end])

			newStart := buf.Len()
			buf.WriteString(actualReplacement)
			lastIdx = red.end

			shifts = append(shifts, shift{red.start, newStart})
			shifts = append(shifts, shift{red.end, buf.Len()})
		}
		buf.WriteString(text[lastIdx:])

		transform := func(orig int) int {
			// Find the last shift that is <= orig
			idx := sort.Search(len(shifts), func(i int) bool {
				return shifts[i].origIdx > orig
			})
			if idx == 0 {
				return orig
			}
			s := shifts[idx-1]

			// If orig was inside a redacted span, it should probably move to the start or end of the replacement
			// But for our purposes, linear interpolation or just using the shift at the start is enough if we only care about span boundaries.

			// Check if orig is exactly one of our tracked points
			if s.origIdx == orig {
				return s.newIdx
			}

			// If it's between points, it's either in a kept segment or a redacted segment.
			// If idx < len(shifts) and s.origIdx was a start and shifts[idx].origIdx was an end:
			// Then orig is inside a redacted span.
			if idx < len(shifts) && (idx-1)%2 == 1 {
				// It's inside a redacted span. map to start of replacement.
				return s.newIdx
			}

			return s.newIdx + (orig - s.origIdx)
		}

		return buf.String(), transform
	}

	expected, transExp := apply(r.Expected)
	actual, transAct := apply(r.Actual)

	r.Expected = expected
	r.Actual = actual

	// Update spans in r.Spans
	for i := range r.Spans {
		r.Spans[i].CharacterStart = transAct(r.Spans[i].CharacterStart)
		r.Spans[i].CharacterEnd = transAct(r.Spans[i].CharacterEnd)
		r.Spans[i].Text = hashText(r.Spans[i].Text)
	}

	// Update spans in Overlaps
	for i := range r.Overlaps {
		if r.Overlaps[i].Golden.Text != "" {
			r.Overlaps[i].Golden.CharacterStart = transExp(r.Overlaps[i].Golden.CharacterStart)
			r.Overlaps[i].Golden.CharacterEnd = transExp(r.Overlaps[i].Golden.CharacterEnd)
			r.Overlaps[i].Golden.Text = hashText(r.Overlaps[i].Golden.Text)
		}
		if r.Overlaps[i].Actual.Text != "" {
			r.Overlaps[i].Actual.CharacterStart = transAct(r.Overlaps[i].Actual.CharacterStart)
			r.Overlaps[i].Actual.CharacterEnd = transAct(r.Overlaps[i].Actual.CharacterEnd)
			r.Overlaps[i].Actual.Text = hashText(r.Overlaps[i].Actual.Text)
		}
	}

	return r
}
