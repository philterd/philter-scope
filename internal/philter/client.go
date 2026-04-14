package philter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/philterd/philterscope/pkg/model"
)

// PhilterClient makes calls to the Philter API.
type PhilterClient struct {
	BaseURL string
	Token   string
}

// PhilterResponse matches the basic Philter API response.
type PhilterResponse struct {
	Value string       `json:"value"`
	Spans []model.Span `json:"spans"`
}

// Redact calls the Philter API's redact endpoint.
func (c *PhilterClient) Redact(text string) (string, []model.Span, error) {
	// Simple JSON request body
	requestBody, err := json.Marshal(map[string]string{
		"text": text,
	})
	if err != nil {
		return "", nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/filter", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("philter API error: %d - %s", resp.StatusCode, string(body))
	}

	var response PhilterResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", nil, err
	}

	return response.Value, response.Spans, nil
}

// GetPolicy retrieves the current Philter policy.
func (c *PhilterClient) GetPolicy() (map[string]any, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/policy", nil)
	if err != nil {
		return nil, err
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("philter API error: %d", resp.StatusCode)
	}

	var policy map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return nil, err
	}

	return policy, nil
}
