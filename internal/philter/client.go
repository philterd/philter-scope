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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/philterd/philterscope/pkg/model"
)

// StatusResponse matches the Philter API status response.
type StatusResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// ExplainResponse matches the Philter API explain response.
type ExplainResponse struct {
	FilteredText string      `json:"filteredText"`
	Context      string      `json:"context"`
	DocumentId   string      `json:"documentId"`
	Explanation  Explanation `json:"explanation"`
}

// Explanation contains the spans identified by Philter.
type Explanation struct {
	AppliedSpans []model.Span `json:"appliedSpans"`
	IgnoredSpans []model.Span `json:"ignoredSpans"`
}

// PhilterClient makes calls to the Philter API.
type PhilterClient struct {
	BaseURL string
	Token   string
	Policy  string
}

// Redact calls the Philter API's explain endpoint to get redacted text and spans.
func (c *PhilterClient) Redact(text string) (string, []model.Span, error) {
	explainResponse, err := c.Explain(text)
	if err != nil {
		return "", nil, err
	}

	// Map FilterType to Label for compatibility
	for i := range explainResponse.Explanation.AppliedSpans {
		s := &explainResponse.Explanation.AppliedSpans[i]
		if s.FilterType == "" && s.Label != "" {
			s.FilterType = s.Label
		}
		if s.Label == "" && s.FilterType != "" {
			s.Label = s.FilterType
		}
	}

	return explainResponse.FilteredText, explainResponse.Explanation.AppliedSpans, nil
}

// Status returns the status of the Philter API.
func (c *PhilterClient) Status() (StatusResponse, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/status", nil)
	if err != nil {
		return StatusResponse{}, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return StatusResponse{}, err
	}
	defer resp.Body.Close()

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return StatusResponse{}, err
	}

	return status, nil
}

// Explain explains the filtering of the text using the Philter API.
func (c *PhilterClient) Explain(text string) (ExplainResponse, error) {
	u, err := url.Parse(c.BaseURL + "/api/explain")
	if err != nil {
		return ExplainResponse{}, err
	}

	params := url.Values{}
	params.Add("p", c.Policy)
	u.RawQuery = params.Encode()

	req, err := http.NewRequest("POST", u.String(), bytes.NewBufferString(text))
	if err != nil {
		return ExplainResponse{}, err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.doRequest(req)
	if err != nil {
		return ExplainResponse{}, err
	}
	defer resp.Body.Close()

	var explain ExplainResponse
	if err := json.NewDecoder(resp.Body).Decode(&explain); err != nil {
		return ExplainResponse{}, err
	}

	return explain, nil
}

// GetPolicyNames retrieves the names of all Philter policies.
func (c *PhilterClient) GetPolicyNames() ([]string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/policies", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var names []string
	if err := json.NewDecoder(resp.Body).Decode(&names); err != nil {
		return nil, err
	}

	return names, nil
}

// GetPolicy retrieves a Philter policy by name.
func (c *PhilterClient) GetPolicy(name string) (string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/policies/"+name, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// UploadPolicy uploads a Philter policy.
func (c *PhilterClient) UploadPolicy(name string, content string) error {
	req, err := http.NewRequest("POST", c.BaseURL+"/api/policies", bytes.NewBufferString(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *PhilterClient) doRequest(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("philter API error: %d - %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
