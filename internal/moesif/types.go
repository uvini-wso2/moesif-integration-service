package moesif

// RawEvent mirrors the (assumed) shape of a single event as returned by
// Moesif's Search API for Asgardeo product activity.
//
// ASSUMPTION — based on the sample shape discussed, not yet verified against
// a real Moesif response. Confirm field names once the real API key arrives.
type RawEvent struct {
	UserID    string `json:"user_id"`
	CompanyID string `json:"company_id"`
	EventType string `json:"event_type"`
	Status    int    `json:"status"`
	Timestamp string `json:"timestamp"` // RFC3339, e.g. "2026-08-20T10:30:00Z"
}
