package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/uvini-wso2/moesif-integration-service/internal/moesif"
)

// eventsClient is the minimal interface this handler needs from the Moesif
// client — kept small on purpose, following the repo's handler conventions.
type eventsClient interface {
	Search(criteria moesif.FilterCriteria) (moesif.SearchResponse, error)
}

// Events handles GET /events?company_id=...&from=...&to=...
//
// It builds a FilterCriteria from query params, searches Moesif, normalizes
// the result, and writes the Summary as JSON.
func Events(client eventsClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		companyID := strings.TrimSpace(r.URL.Query().Get("company_id"))
		if companyID == "" {
			http.Error(w, `{"error":"company_id query parameter is required"}`, http.StatusBadRequest)
			return
		}

		from := r.URL.Query().Get("from")
		if from == "" {
			from = "-30d" // default: last 30 days
		}
		to := r.URL.Query().Get("to")
		if to == "" {
			to = "now"
		}

		// Deliberately no ActionTypes filter here — we fetch ALL of this
		// company's events so Normalize() sees every activity type,
		// including ones we haven't discovered/whitelisted yet. This keeps
		// lastActivity accurate even when new action names appear.
		criteria := moesif.FilterCriteria{
			CompanyID: companyID,
			From:      from,
			To:        to,
		}

		result, err := client.Search(criteria)
		if err != nil {
			slog.Error("moesif search failed", "error", err)
			http.Error(w, `{"error":"failed to fetch events from moesif"}`, http.StatusBadGateway)
			return
		}

		summary := moesif.Normalize(result.Result.Hits)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(summary); err != nil {
			http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
			return
		}
	}
}
