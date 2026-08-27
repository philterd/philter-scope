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
	"errors"
	"net/http"
	"net/http/httptest"
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

// A Philter that cannot be reached has to be distinguishable from one that
// answers with an error: the first fails every document, the second may fail
// only this one.
func TestConnectionErrorForUnreachablePhilter(t *testing.T) {
	// Port 9 is discard; nothing listens on it.
	client := &PhilterClient{BaseURL: "http://127.0.0.1:9"}

	_, _, err := client.Redact("some text")
	if err == nil {
		t.Fatal("expected an error reaching a dead port")
	}

	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected a *ConnectionError, got %T: %v", err, err)
	}
	if connErr.URL != "http://127.0.0.1:9" {
		t.Errorf("expected the URL in the error, got %q", connErr.URL)
	}
	if !strings.Contains(err.Error(), "could not reach Philter") {
		t.Errorf("expected the message to say Philter was unreachable, got %q", err.Error())
	}
}

func TestNoConnectionErrorWhenPhilterAnswersWithAnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	_, _, err := client.Redact("some text")
	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}

	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		t.Error("a 500 response is Philter answering, not a connection failure")
	}
}
