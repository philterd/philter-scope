package audit

import (
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestParseTaggedText(t *testing.T) {
	input := "Hello <NAME>John Doe</NAME>, welcome to <LOCATION>New York</LOCATION>."
	expectedText := "Hello John Doe, welcome to New York."

	text, spans := ParseTaggedText(input)

	if text != expectedText {
		t.Errorf("Expected text %q, got %q", expectedText, text)
	}

	if len(spans) != 2 {
		t.Fatalf("Expected 2 spans, got %d", len(spans))
	}

	if spans[0].Text != "John Doe" || spans[0].Label != "NAME" || spans[0].Start != 6 || spans[0].End != 14 {
		t.Errorf("Unexpected span 0: %+v", spans[0])
	}

	if spans[1].Text != "New York" || spans[1].Label != "LOCATION" || spans[1].Start != 27 || spans[1].End != 35 {
		t.Errorf("Unexpected span 1: %+v", spans[1])
	}
}

func TestParseJSONSpans(t *testing.T) {
	data := []byte(`{
		"text": "Hello John Doe",
		"labels": [
			{"text": "John Doe", "start": 6, "end": 14, "label": "NAME"}
		]
	}`)

	text, spans, err := ParseJSONSpans(data)
	if err != nil {
		t.Fatalf("ParseJSONSpans failed: %v", err)
	}

	if text != "Hello John Doe" {
		t.Errorf("Expected text 'Hello John Doe', got %q", text)
	}

	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	if spans[0].Text != "John Doe" || spans[0].Start != 6 || spans[0].End != 14 {
		t.Errorf("Unexpected span: %+v", spans[0])
	}
}

func TestCalculateOverlap(t *testing.T) {
	golden := []model.Span{
		{Text: "John Doe", Start: 0, End: 8, Label: "NAME"},
		{Text: "New York", Start: 18, End: 26, Label: "LOC"},
	}

	actual := []model.Span{
		{Text: "John Doe", Start: 0, End: 8, Label: "NAME"}, // Exact
		{Text: "York", Start: 22, End: 26, Label: "LOC"},    // Partial
		{Text: "Secret", Start: 30, End: 36, Label: "KEY"},  // False Positive
	}

	overlaps := CalculateOverlap(golden, actual)

	// Golden 0 matched Exact
	// Golden 1 matched Partial
	// Actual 2 matched None (False Positive)

	exactCount := 0
	partialCount := 0
	noneCount := 0

	for _, o := range overlaps {
		switch o.Type {
		case model.OverlapExact:
			exactCount++
		case model.OverlapPartial:
			partialCount++
		case model.OverlapNone:
			noneCount++
		}
	}

	if exactCount != 1 {
		t.Errorf("Expected 1 exact overlap, got %d", exactCount)
	}
	if partialCount != 1 {
		t.Errorf("Expected 1 partial overlap, got %d", partialCount)
	}
	// We have 1 False Negative (but it was marked as Partial because it matched Golden 1)
	// Wait, Golden 1 matched Actual 1 (Partial).
	// Actual 2 matched Nothing (None).
	// Total overlaps:
	// 1. Golden 0 - Actual 0 (EXACT)
	// 2. Golden 1 - Actual 1 (PARTIAL)
	// 3. Actual 2 - (NONE) - False Positive

	if noneCount != 1 {
		t.Errorf("Expected 1 none overlap (FP), got %d", noneCount)
	}
}
