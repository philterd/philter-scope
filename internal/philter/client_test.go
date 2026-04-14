package philter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestRedact(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/filter" {
			t.Errorf("Expected path /api/filter, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected auth token test-token, got %s", r.Header.Get("Authorization"))
		}

		resp := PhilterResponse{
			Value: "Hello REDACTED",
			Spans: []model.Span{
				{Text: "World", Start: 6, End: 11, Label: "NAME"},
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
		if r.URL.Path != "/api/policy" {
			t.Errorf("Expected path /api/policy, got %s", r.URL.Path)
		}

		policy := map[string]any{
			"name": "test-policy",
		}
		json.NewEncoder(w).Encode(policy)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	policy, err := client.GetPolicy()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if policy["name"] != "test-policy" {
		t.Errorf("Expected policy name 'test-policy', got %v", policy["name"])
	}
}

func TestGetPolicy_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &PhilterClient{BaseURL: ts.URL}
	_, err := client.GetPolicy()
	if err == nil {
		t.Error("Expected error for 500 status, got nil")
	}
}
