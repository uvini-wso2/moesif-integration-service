package moesif

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds the settings needed to construct a Client.
type Config struct {
	APIKey  string
	BaseURL string
}

// Client talks to Moesif's Management/Search API.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient creates a Moesif client from the given Config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// do builds and executes an HTTP request against Moesif, returning the raw
// response body. It centralizes header-setting, request execution, and
// status-code checking so individual API methods stay thin.
func (c *Client) do(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
	}

	url := c.cfg.BaseURL + path

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call moesif: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("moesif returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

// SearchEvents calls Moesif's Search API for events matching a simple
// post_filter query, over the given time range.
func (c *Client) SearchEvents(from, to string, postFilter map[string]interface{}) ([]byte, error) {
	path := fmt.Sprintf("/search/~/search/events?from=%s&to=%s", from, to)

	body := map[string]interface{}{
		"post_filter": postFilter,
		"size":        50,
	}

	return c.do(http.MethodPost, path, body)
}

// Search combines BuildPostFilter and SearchEvents, then parses the raw
// response into a SearchResponse — the real, confirmed Moesif shape.
func (c *Client) Search(criteria FilterCriteria) (SearchResponse, error) {
	postFilter := BuildPostFilter(criteria)

	raw, err := c.SearchEvents(criteria.From, criteria.To, postFilter)
	if err != nil {
		return SearchResponse{}, err
	}

	var resp SearchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return SearchResponse{}, fmt.Errorf("parse moesif response: %w", err)
	}

	return resp, nil
}
