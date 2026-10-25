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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/philterd/philterscope/pkg/model"
)

// ExplainResponse matches the Philter API explain response.
type ExplainResponse struct {
	FilteredText string      `json:"filteredText"`
	Context      string      `json:"context"`
	DocumentId   string      `json:"documentId"`
	Explanation  Explanation `json:"explanation"`
}

// Explanation contains the spans identified by Philter.
type Explanation struct {
	AppliedSpans    []model.Span `json:"appliedSpans"`
	IdentifiedSpans []model.Span `json:"identifiedSpans"`
}

// PhilterClient makes calls to the Philter API.
type PhilterClient struct {
	BaseURL string
	Token   string
	Policy  string
	// HTTPClient is used for every request. Nil means http.DefaultClient.
	HTTPClient *http.Client
}

// NewHTTPClient returns an HTTP client for reaching Philter. Philter's Docker
// image serves HTTPS with a self-signed certificate by default, so caCertFile
// names a PEM file of certificates to trust in addition to the system roots,
// and insecure turns certificate verification off entirely.
func NewHTTPClient(caCertFile string, insecure bool) (*http.Client, error) {
	if caCertFile == "" && !insecure {
		return http.DefaultClient, nil
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}
	if caCertFile != "" {
		pem, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no PEM certificates found in %s", caCertFile)
		}
		tlsConfig.RootCAs = pool
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &http.Client{Transport: transport}, nil
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

// policiesPageSize is how many policies GetPolicyNames asks for per request.
const policiesPageSize = 100

// GetPolicyNames retrieves the names of all Philter policies, reading every
// page of the listing.
func (c *PhilterClient) GetPolicyNames() ([]string, error) {
	var names []string
	for {
		params := url.Values{}
		params.Add("offset", strconv.Itoa(len(names)))
		params.Add("limit", strconv.Itoa(policiesPageSize))

		req, err := http.NewRequest("GET", c.BaseURL+"/api/policies?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.doRequest(req)
		if err != nil {
			return nil, err
		}

		var page struct {
			Policies []struct {
				Name string `json:"name"`
			} `json:"policies"`
			Total int64 `json:"total"`
		}
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		for _, p := range page.Policies {
			names = append(names, p.Name)
		}
		if len(page.Policies) == 0 || int64(len(names)) >= page.Total {
			return names, nil
		}
	}
}

// GetPolicy retrieves a Philter policy by name.
func (c *PhilterClient) GetPolicy(name string) (string, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/policies/"+url.PathEscape(name), nil)
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

// UploadPolicy creates a Philter policy. Philter refuses a name already in use
// with 409.
func (c *PhilterClient) UploadPolicy(name string, content string) error {
	params := url.Values{}
	params.Add("name", name)

	req, err := http.NewRequest("POST", c.BaseURL+"/api/policies?"+params.Encode(), bytes.NewBufferString(content))
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

// ConnectionError is returned when Philter could not be reached at all, as
// opposed to answering with an error. The two need telling apart: one document
// Philter rejects is a per-document problem, but a Philter that cannot be
// reached fails every document and is a problem with the run.
type ConnectionError struct {
	URL string
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("could not reach Philter at %s: %v", e.URL, e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// AuthError is returned when Philter refuses the API key with 401, or with 403
// when the key lacks a scope the request needs. Like a ConnectionError, it
// fails every document the same way.
type AuthError struct {
	URL        string
	StatusCode int
	Body       string
}

func (e *AuthError) Error() string {
	if e.StatusCode == http.StatusForbidden {
		return fmt.Sprintf("Philter at %s refused the request for the API key (403): %s", e.URL, e.Body)
	}
	return fmt.Sprintf("Philter at %s refused the API key (%d): %s", e.URL, e.StatusCode, e.Body)
}

func (c *PhilterClient) doRequest(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &ConnectionError{URL: c.BaseURL, Err: err}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, &AuthError{URL: c.BaseURL, StatusCode: resp.StatusCode, Body: string(body)}
		}
		return nil, fmt.Errorf("philter API error: %d - %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
