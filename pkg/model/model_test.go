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

package model

import (
	"encoding/json"
	"testing"
)

func TestSpan_Compatibility(t *testing.T) {
	s := Span{
		CharacterStart: 10,
		CharacterEnd:   20,
		FilterType:     "NAME",
		Label:          "NAME",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Failed to marshal Span: %v", err)
	}

	var s2 Span
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("Failed to unmarshal Span: %v", err)
	}

	if s2.CharacterStart != 10 || s2.CharacterEnd != 20 || s2.Label != "NAME" {
		t.Errorf("Fields lost in JSON roundtrip: %+v", s2)
	}
}

func TestAuditResult_JSON(t *testing.T) {
	ar := AuditResult{
		TotalDocuments: 5,
		F1Score:        0.88,
	}

	data, err := json.Marshal(ar)
	if err != nil {
		t.Fatalf("Failed to marshal AuditResult: %v", err)
	}

	var ar2 AuditResult
	if err := json.Unmarshal(data, &ar2); err != nil {
		t.Fatalf("Failed to unmarshal AuditResult: %v", err)
	}

	if ar2.TotalDocuments != 5 || ar2.F1Score != 0.88 {
		t.Errorf("Data lost in JSON roundtrip: %+v", ar2)
	}
}
