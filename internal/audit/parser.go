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
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/philterd/philterscope/pkg/model"
)

var tagRegex = regexp.MustCompile(`<([A-Z_]+)>`)

// ParseTaggedText parses a text file where PII is wrapped in tags like <NAME>John Doe</NAME>.
// It returns the clean text and a list of spans.
func ParseTaggedText(input string) (string, []model.Span) {
	var spans []model.Span
	cleanText := ""
	lastIdx := 0

	// We'll search for <TAG> and </TAG> manually or with simple regex
	// Since we don't have backreferences, let's find all <TAG> and then their matching </TAG>

	i := 0
	for i < len(input) {
		startTagMatch := tagRegex.FindStringSubmatchIndex(input[i:])
		if startTagMatch == nil {
			break
		}

		// Adjust indices to be relative to input start
		startTagStart := startTagMatch[0] + i
		startTagEnd := startTagMatch[1] + i
		tagName := input[i+startTagMatch[2] : i+startTagMatch[3]]

		// Add text before tag to cleanText
		cleanText += input[lastIdx:startTagStart]

		// Find matching end tag
		endTag := "</" + tagName + ">"
		endTagIdx := strings.Index(input[startTagEnd:], endTag)

		if endTagIdx == -1 {
			// No matching end tag, treat as normal text
			cleanText += input[startTagStart:startTagEnd]
			i = startTagEnd
			lastIdx = startTagEnd
			continue
		}

		// Absolute index of end tag
		endTagStart := endTagIdx + startTagEnd
		endTagEnd := endTagStart + len(endTag)

		content := input[startTagEnd:endTagStart]

		start := len(cleanText)
		cleanText += content
		end := len(cleanText)

		spans = append(spans, model.Span{
			Text:           content,
			Start:          start,
			CharacterStart: start,
			End:            end,
			CharacterEnd:   end,
			Label:          tagName,
			FilterType:     tagName,
		})

		i = endTagEnd
		lastIdx = endTagEnd
	}

	cleanText += input[lastIdx:]

	return cleanText, spans
}

// ParseJSONSpans parses a JSON format that follows the Span-style labeling (text + start/end offsets).
func ParseJSONSpans(data []byte) (string, []model.Span, error) {
	type jsonSpan struct {
		Text   string       `json:"text"`
		Labels []model.Span `json:"labels"`
	}

	var js jsonSpan
	if err := json.Unmarshal(data, &js); err != nil {
		return "", nil, fmt.Errorf("failed to parse JSON golden dataset: %w", err)
	}

	// Populate compatibility fields
	for i := range js.Labels {
		s := &js.Labels[i]
		if s.CharacterStart == 0 && s.Start != 0 {
			s.CharacterStart = s.Start
		}
		if s.CharacterEnd == 0 && s.End != 0 {
			s.CharacterEnd = s.End
		}
		if s.Start == 0 && s.CharacterStart != 0 {
			s.Start = s.CharacterStart
		}
		if s.End == 0 && s.CharacterEnd != 0 {
			s.End = s.CharacterEnd
		}
		if s.FilterType == "" && s.Label != "" {
			s.FilterType = s.Label
		}
		if s.Label == "" && s.FilterType != "" {
			s.Label = s.FilterType
		}
	}

	sort.Slice(js.Labels, func(i, j int) bool {
		return js.Labels[i].Start < js.Labels[j].Start
	})

	return js.Text, js.Labels, nil
}

// ParsePhilterExplain parses a Philter explain JSON response.
func ParsePhilterExplain(data []byte) (string, []model.Span, error) {
	// Re-define internal structures to match Philter's API without importing internal/philter
	type explanation struct {
		AppliedSpans []model.Span `json:"appliedSpans"`
		IgnoredSpans []model.Span `json:"ignoredSpans"`
	}
	type explainResponse struct {
		FilteredText string      `json:"filteredText"`
		Explanation  explanation `json:"explanation"`
	}

	var res explainResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return "", nil, fmt.Errorf("failed to parse Philter explain JSON: %w", err)
	}

	// Map CharacterStart/CharacterEnd/FilterType to Start/End/Label for compatibility
	for i := range res.Explanation.AppliedSpans {
		s := &res.Explanation.AppliedSpans[i]
		if s.CharacterStart == 0 && s.Start != 0 {
			s.CharacterStart = s.Start
		}
		if s.CharacterEnd == 0 && s.End != 0 {
			s.CharacterEnd = s.End
		}
		if s.Start == 0 && s.CharacterStart != 0 {
			s.Start = s.CharacterStart
		}
		if s.End == 0 && s.CharacterEnd != 0 {
			s.End = s.CharacterEnd
		}
		if s.FilterType == "" && s.Label != "" {
			s.FilterType = s.Label
		}
		if s.Label == "" && s.FilterType != "" {
			s.Label = s.FilterType
		}
	}

	return res.FilteredText, res.Explanation.AppliedSpans, nil
}
