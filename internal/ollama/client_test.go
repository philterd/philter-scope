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

package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewClient(t *testing.T) {
	os.Setenv("PHILTERSCOPE_OLLAMA_URL", "http://test-ollama:11434")
	os.Setenv("PHILTERSCOPE_OLLAMA_MODEL", "test-model")
	defer os.Unsetenv("PHILTERSCOPE_OLLAMA_URL")
	defer os.Unsetenv("PHILTERSCOPE_OLLAMA_MODEL")

	client := NewClient()
	if client.BaseURL != "http://test-ollama:11434" {
		t.Errorf("Expected BaseURL http://test-ollama:11434, got %s", client.BaseURL)
	}
	if client.Model != "test-model" {
		t.Errorf("Expected Model test-model, got %s", client.Model)
	}
}

func TestNewClient_Default(t *testing.T) {
	os.Unsetenv("PHILTERSCOPE_OLLAMA_URL")
	os.Unsetenv("PHILTERSCOPE_OLLAMA_MODEL")

	client := NewClient()
	if client.BaseURL != "http://localhost:11434" {
		t.Errorf("Expected default BaseURL http://localhost:11434, got %s", client.BaseURL)
	}
	if client.Model != "gemma4" {
		t.Errorf("Expected default Model gemma4, got %s", client.Model)
	}
}

func TestGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("Expected path /api/generate, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}

		if req.Prompt != "test prompt" {
			t.Errorf("Expected prompt 'test prompt', got %s", req.Prompt)
		}

		resp := GenerateResponse{Response: "test response"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		Model:   "llama3",
	}

	response, err := client.Generate(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if response != "test response" {
		t.Errorf("Expected response 'test response', got %s", response)
	}
}
