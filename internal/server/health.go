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
	"fmt"
	"net/http"

	"github.com/philterd/philterscope/internal/report"
)

// HealthPath is the path of the health endpoint.
const HealthPath = "/api/health"

// HealthResponse is the body of the health endpoint, matching the contract
// shared across Philterd products.
type HealthResponse struct {
	Status             string `json:"status"`
	ApplicationVersion string `json:"applicationVersion"`
}

// registerHealth adds the unauthenticated health endpoint to mux. It is a
// liveness probe: the server answering at all is what it reports on, so it
// does not touch storage and stays reachable in every serving mode.
func registerHealth(mux *http.ServeMux) {
	mux.HandleFunc(HealthPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := HealthResponse{Status: "UP", ApplicationVersion: report.Version}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			fmt.Printf("Error encoding health response: %v\n", err)
		}
	})
}
