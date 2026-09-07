package moesif

import "time"

// Action name values for Moesif's "action_name" field.
const (
	// CONFIRMED — observed in a real Moesif response for Asgardeo activity
	// on 2026-09-07.
	ActionNameOrganizationCreated     = "organization_created"
	ActionNameOrganizationSubscribed  = "organization_subscribed"
	ActionNameUserCreated             = "user_created"
	ActionNameOnboardingStepCompleted = "Onboarding-Step-Completed"

	// ASSUMPTION — NOT yet observed in real data. A 318-event sample pulled
	// on 2026-09-07 contained no action resembling a login/auth attempt.
	// Confirm the real action name with the Moesif admin, or a
	// broader/different query, before relying on this.
	ActionNameAuthenticationAttempt = "authentication_attempt"

	// ASSUMPTION — same caveat as above.
	ActionNameAPICall = "api_call"
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

// Normalize aggregates a slice of raw Moesif hits (already filtered to a
// single company) into a Summary.
//
// ASSUMPTION — ApplicationCreated is currently derived from
// ActionNameOnboardingStepCompleted as a stand-in, since no confirmed
// "application created" action name has been observed yet. Revisit once
// confirmed.
//
// AuthenticationAttempts / AuthenticationSuccessful currently cannot be
// computed from any real data seen so far — no authentication-related
// action name has been confirmed, and no "status" field exists on the real
// event shape. These will stay at zero/false until that signal is found.
func Normalize(hits []RawHit) Summary {
	var summary Summary
	var latest time.Time

	for _, hit := range hits {
		src := hit.Source

		switch src.ActionName {
		case ActionNameOnboardingStepCompleted:
			summary.ApplicationCreated = true
		case ActionNameAuthenticationAttempt:
			summary.AuthenticationAttempts++
			// TODO: no confirmed way to detect success/failure yet —
			// revisit once a real authentication_attempt event is seen.
		case ActionNameAPICall:
			summary.ApiUsageDetected = true
		}

		if ts, err := parseMoesifTime(src.Request.Time); err == nil {
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

// parseMoesifTime parses the observed Moesif request.time format, e.g.
// "2026-09-07T02:00:58.646" — no timezone suffix, treated as UTC.
func parseMoesifTime(raw string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000", raw)
}
