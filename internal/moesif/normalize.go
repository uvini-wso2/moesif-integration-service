package moesif

import "time"

// Event type values assumed to appear in RawEvent.EventType.
//
// ASSUMPTION — these exact strings are guesses, not confirmed against real
// Asgardeo/Moesif data. Confirm and adjust once available.
const (
	EventTypeApplicationCreated  = "application_created"
	EventTypeAuthenticationEvent = "authentication_attempt"
	EventTypeAPICall             = "api_call"
)

// Summary is the normalized, per-customer signal set consumed downstream by
// the PLG backend / Claude interpretation step.
type Summary struct {
	ApplicationCreated       bool   `json:"applicationCreated"`
	AuthenticationAttempts   int    `json:"authenticationAttempts"`
	AuthenticationSuccessful bool   `json:"authenticationSuccessful"`
	ApiUsageDetected         bool   `json:"apiUsageDetected"`
	LastActivity             string `json:"lastActivity"` // date only, e.g. "2026-08-31"
}

// Normalize aggregates a slice of raw Moesif events (already filtered to a
// single company) into a Summary.
//
// ASSUMPTION — these aggregation rules are first-pass guesses, meant to be
// reviewed against real data:
//   - AuthenticationSuccessful = true if ANY authentication_attempt event has
//     status 200. Adjust if it should instead mean the LAST attempt, or ALL.
//   - LastActivity = latest timestamp across all events, truncated to a date
//     (matches the sample's date-only format).
func Normalize(events []RawEvent) Summary {
	var summary Summary
	var latest time.Time

	for _, e := range events {
		switch e.EventType {
		case EventTypeApplicationCreated:
			summary.ApplicationCreated = true
		case EventTypeAuthenticationEvent:
			summary.AuthenticationAttempts++
			if e.Status == 200 {
				summary.AuthenticationSuccessful = true
			}
		case EventTypeAPICall:
			summary.ApiUsageDetected = true
		}

		if ts, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			if ts.After(latest) {
				latest = ts
			}
		}
	}

	if !latest.IsZero() {
		summary.LastActivity = latest.Format("2006-01-02")
	}

	return summary
}
