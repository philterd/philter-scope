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

package server

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

	report, err := GenerateStandaloneReport(result)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(report, "<title>PhilterScope Privacy Lab</title>") {
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
