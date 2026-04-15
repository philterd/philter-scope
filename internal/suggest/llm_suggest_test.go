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

package suggest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/philterd/philterscope/internal/ollama"
	"github.com/philterd/philterscope/pkg/model"
)

func TestLLMSuggester_Suggest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollama.GenerateResponse{
			Response: `[{"entity": "NAME", "description": "High recall for NAME", "action": "None", "snippet": "{}"}]`,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &ollama.Client{
		BaseURL: server.URL,
		Model:   "llama3",
	}

	s := NewLLMSuggester(client)
	result := model.AuditResult{
		F1Score: 0.8,
		EntityMetrics: map[string]float64{
			"NAME": 1.0,
		},
	}

	recs := s.Suggest(result)
	if len(recs) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(recs))
	}
	if recs[0].Entity != "NAME" {
		t.Errorf("Expected recommendation for NAME, got %s", recs[0].Entity)
	}
	if !recs[0].IsAI {
		t.Error("Expected IsAI to be true for LLM recommendation")
	}
}

func TestLLMSuggester_Suggest_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &ollama.Client{
		BaseURL: server.URL,
		Model:   "llama3",
	}

	s := NewLLMSuggester(client)
	recs := s.Suggest(model.AuditResult{})

	if len(recs) != 1 || recs[0].Entity != "AI_ERROR" {
		t.Errorf("Expected AI_ERROR recommendation on server error, got %v", recs)
	}
	if !recs[0].IsAI {
		t.Error("Expected IsAI to be true for LLM error recommendation")
	}
}
