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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func TestHealth(t *testing.T) {
	original := Version
	Version = "1.2.3"
	defer func() { Version = original }()

	standaloneMux, err := NewStandaloneServerMux(model.AuditResult{F1Score: 0.85}, false)
	if err != nil {
		t.Fatalf("Error building standalone mux: %v", err)
	}

	// Both serving modes must expose the endpoint.
	muxes := map[string]*http.ServeMux{
		"NewServerMux":           NewServerMux(&mockStorage{}, false),
		"NewStandaloneServerMux": standaloneMux,
	}

	for name, mux := range muxes {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", HealthPath, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %q", ct)
			}

			var health HealthResponse
			if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
				t.Fatalf("Error decoding health response: %v", err)
			}

			if health.Status != "UP" {
				t.Errorf("Expected status UP, got %q", health.Status)
			}

			if health.ApplicationVersion != "1.2.3" {
				t.Errorf("Expected applicationVersion 1.2.3, got %q", health.ApplicationVersion)
			}
		})
	}
}

func TestHealthMethodNotAllowed(t *testing.T) {
	mux := NewServerMux(&mockStorage{}, false)

	req := httptest.NewRequest("POST", HealthPath, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
