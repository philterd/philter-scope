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
	"time"
)

// ReadyPath is the path of the readiness endpoint.
const ReadyPath = "/api/readyz"

// readyTimeout bounds the backend check, so a hung database answers the probe
// as not ready instead of leaving it waiting.
const readyTimeout = 3 * time.Second

// Backend is the part of a Storage that readiness needs. Storage that does not
// implement it is reported ready, since there is no dependency to check.
type Backend interface {
	// Ping reports whether the backing store can be reached.
	Ping(ctx context.Context) error
	// Mode names the backend in the readiness response.
	Mode() string
}

// ReadyResponse is the body of the readiness endpoint.
type ReadyResponse struct {
	Status string `json:"status"`
	Mode   string `json:"mode"`
	Reason string `json:"reason,omitempty"`
}

// registerReadiness adds the unauthenticated readiness endpoint to mux.
//
// This is separate from /api/health on purpose. Health is a liveness probe: it
// answers UP whenever the process is serving, and a container runtime that
// treated a database blip as a liveness failure would restart a server that is
// working. Readiness is where the dependency check belongs, so a load balancer
// can stop sending requests that would fail without anything being restarted.
//
// store is nil when serving a single report file, which needs no backend and is
// therefore always ready.
func registerReadiness(mux *http.ServeMux, store Storage) {
	mux.HandleFunc(ReadyPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := ReadyResponse{Status: "READY", Mode: "report"}
		status := http.StatusOK

		if store != nil {
			// A store with no reachability check is reported ready, but must
			// not claim to be the single-report mode, which has no store.
			response.Mode = "unknown"
		}

		if backend, ok := store.(Backend); ok {
			response.Mode = backend.Mode()

			ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
			defer cancel()

			if err := backend.Ping(ctx); err != nil {
				response.Status = "NOT_READY"
				response.Reason = err.Error()
				status = http.StatusServiceUnavailable
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			fmt.Printf("Error encoding readiness response: %v\n", err)
		}
	})
}
