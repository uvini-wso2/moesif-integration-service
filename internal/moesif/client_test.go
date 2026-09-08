package moesif

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// buildPage constructs a fake Moesif search response with the given
// number of hits, each carrying a unique, incrementing sort value so
// pagination can be followed realistically.
func buildPage(startIndex, count, total int) SearchResponse {
	hits := make([]RawHit, count)
	for i := 0; i < count; i++ {
		idx := startIndex + i
		hits[i] = RawHit{
			ID: fmt.Sprintf("hit-%d", idx),
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOrganizationCreated,
				Request:    RawRequest{Time: "2026-08-20T10:30:00.000"},
			},
			Sort: []interface{}{float64(1700000000000 - idx)}, // descending, unique per hit
		}
	}
	return SearchResponse{
		Result: HitsResult{Hits: hits, Total: total},
	}
}

// TestSearch_Pagination confirms that Search() correctly follows Moesif's
// keyset/seek pagination across multiple pages: the first page returns a
// full pageSize batch (triggering a second request), the second page
// returns fewer than pageSize (signaling the end), and the combined
// result contains every hit from both pages with the correct total.
func TestSearch_Pagination(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var page SearchResponse
		if requestCount == 1 {
			// First page: a full batch of pageSize hits.
			page = buildPage(0, pageSize, pageSize+50)
		} else {
			// Second page: a partial batch — signals no further pages.
			page = buildPage(pageSize, 50, pageSize+50)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()

	client := NewClient(Config{APIKey: "test-key", BaseURL: server.URL})

	result, err := client.Search(FilterCriteria{CompanyID: "company_456", From: "-30d", To: "now"})
	if err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}

	if requestCount != 2 {
		t.Errorf("expected exactly 2 requests to Moesif, got %d", requestCount)
	}
	if len(result.Result.Hits) != pageSize+50 {
		t.Errorf("expected %d combined hits, got %d", pageSize+50, len(result.Result.Hits))
	}
	if result.Result.Total != pageSize+50 {
		t.Errorf("expected Total = %d (from first page), got %d", pageSize+50, result.Result.Total)
	}
}

// TestSearch_SinglePage confirms the common case: when the first page
// already returns fewer than pageSize hits, Search() makes exactly one
// request and does not attempt to paginate further.
func TestSearch_SinglePage(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		page := buildPage(0, 9, 9)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()

	client := NewClient(Config{APIKey: "test-key", BaseURL: server.URL})

	result, err := client.Search(FilterCriteria{CompanyID: "company_456", From: "-30d", To: "now"})
	if err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected exactly 1 request to Moesif, got %d", requestCount)
	}
	if len(result.Result.Hits) != 9 {
		t.Errorf("expected 9 hits, got %d", len(result.Result.Hits))
	}
}

// TestSearch_RespectsMaxPages confirms the safety cap: even if Moesif
// keeps returning full pages indefinitely, Search() stops after maxPages
// requests rather than looping forever.
func TestSearch_RespectsMaxPages(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Always return a full page, as if there were infinite data.
		page := buildPage(requestCount*pageSize, pageSize, 999999)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	}))
	defer server.Close()

	client := NewClient(Config{APIKey: "test-key", BaseURL: server.URL})

	result, err := client.Search(FilterCriteria{CompanyID: "company_456", From: "-30d", To: "now"})
	if err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}

	if requestCount != maxPages {
		t.Errorf("expected exactly %d requests (safety cap), got %d", maxPages, requestCount)
	}
	if len(result.Result.Hits) != pageSize*maxPages {
		t.Errorf("expected %d hits (capped), got %d", pageSize*maxPages, len(result.Result.Hits))
	}
	if result.Result.Total != 999999 {
		t.Errorf("expected Total = 999999 (Moesif's real total, even though capped), got %d", result.Result.Total)
	}
}
