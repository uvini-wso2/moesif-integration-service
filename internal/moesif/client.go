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

// pageSize is the number of events requested per page. Matches the batch
// size used in Moesif's own pagination documentation example.
const pageSize = 100

// maxPages caps how many pages Search will fetch for a single lookup
// (pageSize * maxPages = 1000 events max). Moesif's own docs state the
// Search API is intended for interactive workflows, not bulk export —
// they recommend a separate bulk-export API for large data pulls. 1000
// events is already generous for a single company/user activity summary;
// this cap exists to avoid misusing the Search API for something it
// wasn't designed for, not because 1000 is a hard technical limit.
const maxPages = 10

// SearchEvents calls Moesif's Search API for a single page of events
// matching a post_filter query, over the given time range. sort defines
// the ordering (required for pagination); searchAfter is the sort key(s)
// from the last hit of the previous page, or nil for the first page.
func (c *Client) SearchEvents(from, to string, postFilter map[string]interface{}, sort []map[string]interface{}, searchAfter []interface{}) ([]byte, error) {
	path := fmt.Sprintf("/search/~/search/events?from=%s&to=%s", from, to)

	body := map[string]interface{}{
		"post_filter": postFilter,
		"size":        pageSize,
		"sort":        sort,
	}
	if searchAfter != nil {
		body["search_after"] = searchAfter
	}

	return c.do(http.MethodPost, path, body)
}

// Search combines BuildPostFilter and SearchEvents, then paginates through
// all matching pages (up to maxPages) using Moesif's documented
// keyset/seek pagination, returning every hit combined into one
// SearchResponse. Total always reflects Moesif's true total count, even
// if the number of Hits actually retrieved is capped by maxPages.
func (c *Client) Search(criteria FilterCriteria) (SearchResponse, error) {
	postFilter := BuildPostFilter(criteria)

	// Sort by request time, descending — matches Moesif's own pagination
	// example and ensures the most recent events are processed first.
	sort := []map[string]interface{}{
		{
			"request.time": map[string]interface{}{
				"order":         "desc",
				"unmapped_type": "string",
			},
		},
	}

	var allHits []RawHit
	var total int
	var searchAfter []interface{}

	for page := 0; page < maxPages; page++ {
		raw, err := c.SearchEvents(criteria.From, criteria.To, postFilter, sort, searchAfter)
		if err != nil {
			return SearchResponse{}, err
		}

		var resp SearchResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return SearchResponse{}, fmt.Errorf("parse moesif response: %w", err)
		}

		if page == 0 {
			total = resp.Result.Total
		}

		allHits = append(allHits, resp.Result.Hits...)

		if len(resp.Result.Hits) < pageSize {
			// Fewer than a full page came back — no more pages exist.
			break
		}

		lastHit := resp.Result.Hits[len(resp.Result.Hits)-1]
		if len(lastHit.Sort) == 0 {
			// No sort key to continue with — stop rather than loop forever.
			break
		}
		searchAfter = lastHit.Sort
	}

	return SearchResponse{
		Result: HitsResult{Hits: allHits, Total: total},
	}, nil
}
