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
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// Philter 4 lists policies a page at a time, as objects, with the total.
func TestGetPolicyNames(t *testing.T) {
	const total = policiesPageSize + 5
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/policies" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		var policies []map[string]any
		for i := offset; i < total && i < offset+limit; i++ {
			policies = append(policies, map[string]any{"name": fmt.Sprintf("policy%d", i), "revision": 0})
		}
		json.NewEncoder(w).Encode(map[string]any{"policies": policies, "total": total})
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	names, err := client.GetPolicyNames()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(names) != total || names[0] != "policy0" || names[total-1] != fmt.Sprintf("policy%d", total-1) {
		t.Errorf("Unexpected names: %v", names)
	}
}

func TestUploadPolicy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/policies" {
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("name") != "new-policy" {
			t.Errorf("Expected name=new-policy, got %q", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusCreated)
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

func TestUploadPolicy_Exists(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"A policy with this name already exists.","reason":"policy_exists"}`))
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	err := client.UploadPolicy("new-policy", `{}`)
	if err == nil || !strings.Contains(err.Error(), "409") {
		t.Fatalf("Expected a 409 error, got %v", err)
	}
}

// Philter's Docker image serves HTTPS with a self-signed certificate, which the
// default client rejects.
func TestNewHTTPClient_TLS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ExplainResponse{FilteredText: "redacted"})
	}))
	defer ts.Close()

	explain := func(httpClient *http.Client) error {
		client := &PhilterClient{BaseURL: ts.URL, HTTPClient: httpClient}
		_, err := client.Explain("test")
		return err
	}

	defaultClient, err := NewHTTPClient("", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := explain(defaultClient); err == nil {
		t.Error("Expected the default client to reject an untrusted certificate")
	}

	caFile := filepath.Join(t.TempDir(), "philter.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	trusting, err := NewHTTPClient(caFile, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := explain(trusting); err != nil {
		t.Errorf("Expected the certificate in --ca-cert to be trusted, got %v", err)
	}

	insecure, err := NewHTTPClient("", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := explain(insecure); err != nil {
		t.Errorf("Expected --insecure to skip verification, got %v", err)
	}
}

func TestNewHTTPClient_BadCACert(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "not-a-cert.pem")
	if err := os.WriteFile(caFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewHTTPClient(caFile, false); err == nil {
		t.Error("Expected an error for a file with no certificates")
	}
	if _, err := NewHTTPClient(filepath.Join(t.TempDir(), "missing.pem"), false); err == nil {
		t.Error("Expected an error for a missing file")
	}
}

// A refused API key fails every document, so it is reported as an AuthError
// rather than an ordinary API error.
func TestAuthErrorForRefusedKey(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"message":"refused","reason":"test_reason"}`))
		}))

		client := &PhilterClient{BaseURL: ts.URL}
		_, _, err := client.Redact("some text")
		ts.Close()

		var authErr *AuthError
		if !errors.As(err, &authErr) {
			t.Fatalf("%d: expected an *AuthError, got %T: %v", status, err, err)
		}
		if authErr.StatusCode != status {
			t.Errorf("expected status %d, got %d", status, authErr.StatusCode)
		}
		if !strings.Contains(err.Error(), "test_reason") {
			t.Errorf("%d: expected Philter's reason in the message, got %q", status, err.Error())
		}
	}
}
