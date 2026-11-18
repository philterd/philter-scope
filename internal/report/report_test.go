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
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestGenerateStandaloneReport(t *testing.T) {
	result := model.AuditResult{
		TotalDocuments: 1,
		Recall:         0.9,
		Precision:      0.8,
		F1Score:        0.85,
		Details: []model.Result{
			{Filename: "test.txt", Expected: "Hello World", Actual: "Hello REDACTED", TP: 1},
		},
	}

	report, err := GenerateStandaloneReport(result, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(report, "<title>PhilterScope Evaluation</title>") {
		t.Error("Report missing expected title")
	}

	if !strings.Contains(report, "test.txt") {
		t.Error("Report missing filename")
	}

	// Check if JSON data is embedded
	if !strings.Contains(report, `"total_documents":1`) {
		t.Errorf("Report missing JSON data (checked for '\"total_documents\":1')\nGot: %s", report)
	}
}

func TestObfuscateAuditResult(t *testing.T) {
	res := model.AuditResult{
		Details: []model.Result{
			{
				Filename: "test.txt",
				Expected: "My name is John Doe.",
				Actual:   "My name is John Doe.",
				Spans: []model.Span{
					{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19, Label: "NAME"},
				},
				Overlaps: []model.Overlap{
					{
						Golden: model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19},
						Actual: model.Span{Text: "John Doe", CharacterStart: 11, CharacterEnd: 19},
						Type:   "EXACT",
					},
				},
			},
		},
	}

	obfuscated := ObfuscateAuditResult(res)

	if obfuscated.Details[0].Expected != "My name is 4c2a9." {
		t.Errorf("Expected obfuscated 'Expected' text to be 'My name is 4c2a9.', got %q", obfuscated.Details[0].Expected)
	}

	if obfuscated.Details[0].Actual != "My name is 4c2a9." {
		t.Errorf("Expected obfuscated 'Actual' text to be 'My name is 4c2a9.', got %q", obfuscated.Details[0].Actual)
	}

	if obfuscated.Details[0].Spans[0].Text != "4c2a9" {
		t.Errorf("Expected obfuscated span text to be '4c2a9', got %q", obfuscated.Details[0].Spans[0].Text)
	}

	if obfuscated.Details[0].Spans[0].CharacterEnd != 16 {
		t.Errorf("Expected obfuscated span characterEnd to be 16, got %d", obfuscated.Details[0].Spans[0].CharacterEnd)
	}

	if obfuscated.Details[0].Overlaps[0].Golden.Text != "4c2a9" {
		t.Errorf("Expected obfuscated golden span text to be '4c2a9', got %q", obfuscated.Details[0].Overlaps[0].Golden.Text)
	}

	if obfuscated.Details[0].Overlaps[0].Golden.CharacterEnd != 16 {
		t.Errorf("Expected obfuscated golden span characterEnd to be 16, got %d", obfuscated.Details[0].Overlaps[0].Golden.CharacterEnd)
	}
}
