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
			Text:  content,
			Start: start,
			End:   end,
			Label: tagName,
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

	// Sort spans by start offset for consistency
	sort.Slice(js.Labels, func(i, j int) bool {
		return js.Labels[i].Start < js.Labels[j].Start
	})

	return js.Text, js.Labels, nil
}
