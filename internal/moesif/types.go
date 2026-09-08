package moesif

// SearchResponse mirrors Moesif's Search Events API response envelope.
//
// CONFIRMED (2026-09-07): the real response nests hits+total inside a
// "hits" object (Elasticsearch-style), not at the top level.
type SearchResponse struct {
	Result   HitsResult `json:"hits"`
	Took     int        `json:"took"`
	TimedOut bool       `json:"timed_out"`
}

// HitsResult holds the actual array of hits and the total count.
type HitsResult struct {
	Hits  []RawHit `json:"hits"`
	Total int      `json:"total"`
}

// RawHit is a single search result — Moesif wraps the actual event fields
// under "_source". Sort holds the sort key(s) for this hit, which Moesif
// requires to be echoed back as search_after when requesting the next
// page — see client.go's Search method and Moesif's documented
// keyset/seek pagination mechanism.
type RawHit struct {
	ID     string        `json:"_id"`
	Source RawSource     `json:"_source"`
	Sort   []interface{} `json:"sort"`
}

// RawSource is the actual event data, confirmed against a real Moesif
// response for Asgardeo activity (2026-09-07/08).
type RawSource struct {
	CompanyID  string      `json:"company_id"`
	UserID     string      `json:"user_id"`
	EventType  string      `json:"event_type"`  // observed: always "user_action" — not useful for filtering
	ActionName string      `json:"action_name"` // the real distinguishing field, e.g. "organization_created"
	Request    RawRequest  `json:"request"`
	Metadata   RawMetadata `json:"metadata"`
}

// RawRequest holds request-level details, including the event's timestamp.
type RawRequest struct {
	Time string `json:"time"` // e.g. "2026-09-07T02:00:58.646" (no timezone suffix — treat as UTC)
	Verb string `json:"verb"`
	URI  string `json:"uri"`
}

// RawMetadata holds event-specific metadata. Not all events populate all
// (or any) of these fields — e.g. some events have metadata: null entirely,
// which unmarshals fine into a zero-value RawMetadata. CONFIRMED
// (2026-09-08): both Onboarding-Step-Completed and Onboarding-Skipped
// events carry StepNumber/StepName, e.g. step_number: 1,
// step_name: "app_name_entered".
//
// StepNumber is a pointer because 0 is a real, valid step
// ("welcome_option_selected") — nil distinguishes "no step data present"
// from "genuinely step 0".
type RawMetadata struct {
	StepNumber *int   `json:"step_number"`
	StepName   string `json:"step_name"`
}
