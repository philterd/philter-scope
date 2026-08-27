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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

// pingableStorage is a mockStorage that also reports a backend mode and a
// reachability result, the way the real storage backends do.
type pingableStorage struct {
	mockStorage
	mode    string
	pingErr error
}

func (p *pingableStorage) Mode() string { return p.mode }

func (p *pingableStorage) Ping(ctx context.Context) error { return p.pingErr }

func readiness(t *testing.T, mux *http.ServeMux) (int, ReadyResponse) {
	t.Helper()
	req := httptest.NewRequest("GET", ReadyPath, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var body ReadyResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding readiness response: %v", err)
	}
	return w.Code, body
}

func TestReadinessWhenBackendIsReachable(t *testing.T) {
	mux := NewServerMux(&pingableStorage{mode: "mongodb"}, false)

	code, body := readiness(t, mux)
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if body.Status != "READY" {
		t.Errorf("expected READY, got %q", body.Status)
	}
	if body.Mode != "mongodb" {
		t.Errorf("expected mode mongodb, got %q", body.Mode)
	}
	if body.Reason != "" {
		t.Errorf("expected no reason, got %q", body.Reason)
	}
}

func TestReadinessWhenBackendIsUnreachable(t *testing.T) {
	mux := NewServerMux(&pingableStorage{
		mode:    "mongodb",
		pingErr: fmt.Errorf("connection refused"),
	}, false)

	code, body := readiness(t, mux)
	if code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", code)
	}
	if body.Status != "NOT_READY" {
		t.Errorf("expected NOT_READY, got %q", body.Status)
	}
	if body.Mode != "mongodb" {
		t.Errorf("expected mode mongodb, got %q", body.Mode)
	}
	if !strings.Contains(body.Reason, "connection refused") {
		t.Errorf("expected the reason to say why, got %q", body.Reason)
	}
}

// File storage and MongoDB have to be distinguishable, so an operator can tell
// which backend the server came up with.
func TestReadinessReportsTheBackendMode(t *testing.T) {
	for _, mode := range []string{"mongodb", "file"} {
		t.Run(mode, func(t *testing.T) {
			_, body := readiness(t, NewServerMux(&pingableStorage{mode: mode}, false))
			if body.Mode != mode {
				t.Errorf("expected mode %q, got %q", mode, body.Mode)
			}
		})
	}
}

// Serving a single report file has no backend to check.
func TestReadinessInStandaloneMode(t *testing.T) {
	mux, err := NewStandaloneServerMux(model.AuditResult{F1Score: 0.85}, false)
	if err != nil {
		t.Fatalf("NewStandaloneServerMux: %v", err)
	}

	code, body := readiness(t, mux)
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if body.Status != "READY" {
		t.Errorf("expected READY, got %q", body.Status)
	}
	if body.Mode != "report" {
		t.Errorf("expected mode report, got %q", body.Mode)
	}
}

// Storage that cannot be checked is not a reason to report not-ready, but it
// must not be reported as the single-report mode either: there is a store, it
// just cannot say whether it is reachable.
func TestReadinessWithStorageThatCannotBePinged(t *testing.T) {
	code, body := readiness(t, NewServerMux(&mockStorage{}, false))
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if body.Status != "READY" {
		t.Errorf("expected READY, got %q", body.Status)
	}
	if body.Mode != "unknown" {
		t.Errorf("expected mode unknown, got %q", body.Mode)
	}
}

// Liveness must not start failing because a backend is down: that is what
// separates it from readiness.
func TestHealthStaysUpWhenBackendIsUnreachable(t *testing.T) {
	mux := NewServerMux(&pingableStorage{
		mode:    "mongodb",
		pingErr: fmt.Errorf("connection refused"),
	}, false)

	req := httptest.NewRequest("GET", HealthPath, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected health to stay 200, got %d", w.Code)
	}

	var health HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("decoding health response: %v", err)
	}
	if health.Status != "UP" {
		t.Errorf("expected health to stay UP, got %q", health.Status)
	}
}

func TestReadinessMethodNotAllowed(t *testing.T) {
	mux := NewServerMux(&pingableStorage{mode: "file"}, false)

	req := httptest.NewRequest("POST", ReadyPath, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
