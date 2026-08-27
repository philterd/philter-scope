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
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsPath is the path of the Prometheus metrics endpoint.
const MetricsPath = "/metrics"

// metrics holds the collectors for one server. They live on the server rather
// than in package state so two servers in one process, and successive servers
// in a test binary, do not share counters or fail to register.
type metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newMetrics() *metrics {
	registry := prometheus.NewRegistry()

	// Go runtime and process collectors, the same baseline any Prometheus
	// exporter is expected to provide.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m := &metrics{
		registry: registry,
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "philterscope_http_requests_total",
				Help: "Total API requests, by route, method and response code.",
			},
			[]string{"route", "method", "code"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "philterscope_http_request_duration_seconds",
				Help:    "API request duration, by route and method.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"route", "method"},
		),
	}

	registry.MustRegister(m.requests, m.duration)
	return m
}

// statusRecorder captures the response code, which the handler writes but
// http.ResponseWriter does not expose.
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// instrument wraps a handler so its requests are counted and timed. The route
// label is the registered pattern rather than the request path, so an ID in the
// query string cannot multiply the label values.
func (m *metrics) instrument(route string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		start := time.Now()

		h(rec, r)

		if rec.code == 0 {
			rec.code = http.StatusOK
		}
		m.duration.WithLabelValues(route, r.Method).Observe(time.Since(start).Seconds())
		m.requests.WithLabelValues(route, r.Method, strconv.Itoa(rec.code)).Inc()
	}
}

// register adds the metrics endpoint to mux. It is unauthenticated, like the
// health and readiness endpoints, and is not itself instrumented: a scrape
// every few seconds would otherwise dominate the counters it reports.
func (m *metrics) register(mux *http.ServeMux) {
	mux.Handle(MetricsPath, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
}
