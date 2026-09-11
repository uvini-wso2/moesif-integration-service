package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/uvini-wso2/plg-activity-service/internal/moesif"
)

// eventsClient is the minimal interface this handler needs from the Moesif
// client — kept small on purpose, following the repo's handler conventions.
type eventsClient interface {
	Search(criteria moesif.FilterCriteria) (moesif.SearchResponse, error)
}

// maxParamLength caps the length of any single query parameter accepted
// by this handler. Moesif itself doesn't enforce a strict format for
// company_id/user_id (confirmed against real data — most are UUIDs, but
// at least one legitimate anonymous/session-style ID does not match UUID
// shape), so this is a loose sanity bound against obviously-wrong input
// (e.g. an accidental paste of a large blob of text), not a format check.
const maxParamLength = 200

// validateParam trims whitespace and checks the result is within
// maxParamLength. Presence/required-ness is enforced separately (see the
// "at least one of company_id or user_id" check below) since no single
// param here is unconditionally required. Returns an error message
// suitable for direct display to the caller, or "" if the value is valid.
func validateParam(name, value string) (trimmed string, errMsg string) {
	trimmed = strings.TrimSpace(value)
	if len(trimmed) > maxParamLength {
		return trimmed, fmt.Sprintf("%s must be %d characters or fewer", name, maxParamLength)
	}
	return trimmed, ""
}

// Events handles GET /events?company_id=...&user_id=...&from=...&to=...
//
// At least one of company_id or user_id is required; both may be provided
// together for a more precise lookup. It builds a FilterCriteria, searches
// Moesif, normalizes the result, and writes the Summary as JSON, alongside
// the raw eventsFound count so callers can distinguish "no data exists for
// this identifier" (eventsFound == 0) from "data exists but doesn't match
// any recognized signal".
func Events(client eventsClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		companyID, errMsg := validateParam("company_id", r.URL.Query().Get("company_id"))
		if errMsg != "" {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, errMsg), http.StatusBadRequest)
			return
		}

		userID, errMsg := validateParam("user_id", r.URL.Query().Get("user_id"))
		if errMsg != "" {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, errMsg), http.StatusBadRequest)
			return
		}

		if companyID == "" && userID == "" {
			http.Error(w, `{"error":"at least one of company_id or user_id query parameters is required"}`, http.StatusBadRequest)
			return
		}

		from, errMsg := validateParam("from", r.URL.Query().Get("from"))
		if errMsg != "" {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, errMsg), http.StatusBadRequest)
			return
		}
		if from == "" {
			from = "-30d" // default: last 30 days
		}

		to, errMsg := validateParam("to", r.URL.Query().Get("to"))
		if errMsg != "" {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, errMsg), http.StatusBadRequest)
			return
		}
		if to == "" {
			to = "now"
		}

		// Deliberately no ActionTypes filter here — we fetch ALL matching
		// events so Normalize() sees every activity type, including ones
		// we haven't discovered/whitelisted yet. This keeps lastActivity
		// accurate even when new action names appear.
		criteria := moesif.FilterCriteria{
			CompanyID: companyID,
			UserID:    userID,
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

		// EventsFound comes straight from Moesif's real total hit count.
		// 0 unambiguously means no data exists for this identifier at all
		// — every company/user record we've seen only ever comes into
		// existence via an event, so a genuine zero-activity account is
		// not distinguishable from "unknown ID" any other way.
		response := struct {
			moesif.Summary
			EventsFound int `json:"eventsFound"`
		}{
			Summary:     summary,
			EventsFound: result.Result.Total,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
			return
		}
	}
}
