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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/philterd/philterscope/pkg/model"
)

func scrape(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	req := httptest.NewRequest("GET", MetricsPath, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from %s, got %d", MetricsPath, w.Code)
	}
	return w.Body.String()
}

func TestMetricsExposesPrometheusText(t *testing.T) {
	mux := NewServerMux(&mockStorage{}, false)

	// A labelled metric family appears in a scrape once it has a child, so
	// make a request before looking for the request families.
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/history", nil))

	body := scrape(t, mux)

	for _, want := range []string{
		"# HELP philterscope_http_requests_total",
		"# TYPE philterscope_http_requests_total counter",
		"# HELP philterscope_http_request_duration_seconds",
		"# TYPE philterscope_http_request_duration_seconds histogram",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected the scrape to contain %q", want)
		}
	}
}

func TestMetricsCountsAPIRequests(t *testing.T) {
	mux := NewServerMux(&mockStorage{}, false)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/history", nil)
		mux.ServeHTTP(httptest.NewRecorder(), req)
	}

	body := scrape(t, mux)
	want := `philterscope_http_requests_total{code="200",method="GET",route="/api/history"} 3`
	if !strings.Contains(body, want) {
		t.Errorf("expected %q in the scrape, got:\n%s", want, body)
	}
	if !strings.Contains(body, `philterscope_http_request_duration_seconds_count{method="GET",route="/api/history"} 3`) {
		t.Error("expected the duration histogram to have counted the same three requests")
	}
}

// The response code has to be recorded, not assumed to be 200.
func TestMetricsRecordsErrorCodes(t *testing.T) {
	mux := NewServerMux(&mockStorage{}, false)

	// No id parameter, which the handler rejects with 400.
	req := httptest.NewRequest("GET", "/api/audit", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	body := scrape(t, mux)
	want := `philterscope_http_requests_total{code="400",method="GET",route="/api/audit"} 1`
	if !strings.Contains(body, want) {
		t.Errorf("expected %q in the scrape, got:\n%s", want, body)
	}
}

// An ID in the query string must not become a label value, or a scrape grows
// without bound as audits accumulate.
func TestMetricsRouteLabelIsThePatternNotThePath(t *testing.T) {
	mux := NewServerMux(&mockStorage{}, false)

	for _, id := range []string{"507f1f77bcf86cd799439011", "507f1f77bcf86cd799439012", "nope"} {
		req := httptest.NewRequest("GET", "/api/audit?id="+id, nil)
		mux.ServeHTTP(httptest.NewRecorder(), req)
	}

	body := scrape(t, mux)
	for _, id := range []string{"507f1f77bcf86cd799439011", "507f1f77bcf86cd799439012"} {
		if strings.Contains(body, id) {
			t.Errorf("audit ID %q leaked into a metric label", id)
		}
	}
	if strings.Count(body, `route="/api/audit"`) == 0 {
		t.Error("expected the requests to be counted under the route pattern")
	}
}

// Scraping every few seconds would otherwise swamp the counters it reports.
func TestMetricsDoesNotCountItself(t *testing.T) {
	mux := NewServerMux(&mockStorage{}, false)

	scrape(t, mux)
	scrape(t, mux)
	body := scrape(t, mux)

	if strings.Contains(body, `route="/metrics"`) {
		t.Error("the metrics endpoint should not instrument itself")
	}
}

// Serving a single report registers no API routes, so its scrape carries the
// runtime metrics and nothing else. The endpoint still has to answer.
func TestMetricsInStandaloneMode(t *testing.T) {
	mux, err := NewStandaloneServerMux(model.AuditResult{F1Score: 0.85}, false)
	if err != nil {
		t.Fatalf("NewStandaloneServerMux: %v", err)
	}
	body := scrape(t, mux)
	if !strings.Contains(body, "go_goroutines") {
		t.Error("expected the standalone server to expose runtime metrics")
	}
}

// Two servers in one process must not share counters or fail to register.
func TestMetricsArePerServer(t *testing.T) {
	first := NewServerMux(&mockStorage{}, false)
	second := NewServerMux(&mockStorage{}, false)

	req := httptest.NewRequest("GET", "/api/history", nil)
	first.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(scrape(t, first), `route="/api/history"} 1`) {
		t.Error("expected the first server to have counted its request")
	}
	if strings.Contains(scrape(t, second), `route="/api/history"} 1`) {
		t.Error("the second server should have its own counters")
	}
}
