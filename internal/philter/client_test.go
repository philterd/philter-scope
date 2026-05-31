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

package philter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestRedact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/explain" {
			t.Errorf("Expected path /api/explain, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected auth token test-token, got %s", r.Header.Get("Authorization"))
		}

		resp := ExplainResponse{
			FilteredText: "Hello REDACTED",
			Explanation: Explanation{
				AppliedSpans: []model.Span{
					{Text: "World", CharacterStart: 6, CharacterEnd: 11, FilterType: "NAME"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &PhilterClient{
		BaseURL: ts.URL,
		Token:   "test-token",
	}

	value, spans, err := client.Redact("Hello World")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if value != "Hello REDACTED" {
		t.Errorf("Expected 'Hello REDACTED', got %s", value)
	}
	if len(spans) != 1 || spans[0].Text != "World" {
		t.Errorf("Unexpected spans: %v", spans)
	}
	if spans[0].CharacterStart != 6 || spans[0].CharacterEnd != 11 || spans[0].Label != "NAME" {
		t.Errorf("Fields not mapped: %v", spans[0])
	}
}

func TestRedact_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	_, _, err := client.Redact("test")
	if err == nil {
		t.Error("Expected error for 400 status, got nil")
	}
}

// TestRedact_DecodesRealPhilterExplainShape guards against contract drift with Philter. The server
// returns the exact JSON shape Philter's /api/explain emits (a serialized TextFilterResult captured
// in testdata), rather than re-encoding this package's own structs. If Philter changes that shape
// (for example dropping the "explanation" wrapper or "filteredText", as happened once), this test
// fails instead of silently passing a struct round-trip.
func TestRedact_DecodesRealPhilterExplainShape(t *testing.T) {
	fixture, err := os.ReadFile("testdata/explain_response.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/explain" {
			t.Errorf("expected /api/explain, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL, Policy: "default"}
	redacted, spans, err := client.Redact("John Doe lives in 90210.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(redacted, "REDACTED-person") {
		t.Errorf("filteredText was not decoded from Philter's response: %q", redacted)
	}
	if len(spans) != 2 {
		t.Fatalf("expected 2 applied spans decoded from explanation, got %d", len(spans))
	}
	if spans[0].FilterType != "PERSON" {
		t.Errorf("expected first span filterType PERSON, got %q", spans[0].FilterType)
	}
	if spans[0].CharacterStart != 0 || spans[0].CharacterEnd != 8 {
		t.Errorf("span character offsets not decoded: start=%d end=%d", spans[0].CharacterStart, spans[0].CharacterEnd)
	}
	if spans[0].Text != "John Doe" {
		t.Errorf("span text not decoded: %q", spans[0].Text)
	}
}

func TestGetPolicy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/policies/test-policy" {
			t.Errorf("Expected path /api/policies/test-policy, got %s", r.URL.Path)
		}

		policy := `{"name": "test-policy"}`
		w.Write([]byte(policy))
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	policy, err := client.GetPolicy("test-policy")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if policy != `{"name": "test-policy"}` {
		t.Errorf("Expected policy '{\"name\": \"test-policy\"}', got %s", policy)
	}
}

func TestGetPolicy_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	_, err := client.GetPolicy("test-policy")
	if err == nil {
		t.Error("Expected error for 500 status, got nil")
	}
}

func TestStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/status" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		// Emit the raw shape Philter 4.0.0 actually returns (applicationVersion, not version).
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"Healthy","applicationVersion":"4.0.0","redactionPolicySchemaVersion":"1.0.0","gitCommit":"abc123"}`))
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	resp, err := client.Status()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Status != "Healthy" {
		t.Errorf("Expected status Healthy, got %s", resp.Status)
	}
	if resp.ApplicationVersion != "4.0.0" {
		t.Errorf("Expected applicationVersion 4.0.0, got %q", resp.ApplicationVersion)
	}
}

func TestExplain(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/explain" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(ExplainResponse{FilteredText: "redacted"})
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	resp, err := client.Explain("test")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.FilteredText != "redacted" {
		t.Errorf("Expected filtered text 'redacted', got %s", resp.FilteredText)
	}
}

func TestGetPolicyNames(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/policies" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode([]string{"policy1", "policy2"})
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	names, err := client.GetPolicyNames()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "policy1" {
		t.Errorf("Unexpected names: %v", names)
	}
}

func TestGetPolicyNames_PagesThroughAllResults(t *testing.T) {
	// Philter caps each page at 100 names. The client must page until a short page is returned.
	const total = 230
	var requestedOffsets []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/policies" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}

		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			t.Errorf("Expected a positive limit, got %d", limit)
		}
		requestedOffsets = append(requestedOffsets, offset)

		page := []string{}
		for i := offset; i < offset+limit && i < total; i++ {
			page = append(page, fmt.Sprintf("policy%d", i))
		}
		json.NewEncoder(w).Encode(page)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	names, err := client.GetPolicyNames()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(names) != total {
		t.Fatalf("Expected %d names, got %d", total, len(names))
	}
	if names[0] != "policy0" || names[total-1] != fmt.Sprintf("policy%d", total-1) {
		t.Errorf("Unexpected first/last names: %s ... %s", names[0], names[total-1])
	}
	// 230 names at 100/page => offsets 0, 100, 200 (the third page is short and ends paging).
	if len(requestedOffsets) != 3 {
		t.Errorf("Expected 3 page requests, got %d (offsets %v)", len(requestedOffsets), requestedOffsets)
	}
}

func TestUploadPolicy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/policies" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	err := client.UploadPolicy("new-policy", `{"content": "here"}`)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}
