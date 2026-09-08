package moesif

import "time"

// Action name values for Moesif's "action_name" field.
const (
	// CONFIRMED — observed in real Moesif responses for Asgardeo activity,
	// across Prod/Dev/Staging/Test environments (2026-09-08).
	//
	// Only ActionNameOnboardingStepCompleted is currently wired into
	// Normalize()'s classification below. OrganizationCreated,
	// OrganizationSubscribed, and UserCreated are kept here as confirmed
	// real values (useful reference/building blocks) even though nothing
	// currently classifies on them — not dead code, just not yet needed
	// for any Summary field.
	ActionNameOrganizationCreated     = "organization_created"
	ActionNameOrganizationSubscribed  = "organization_subscribed"
	ActionNameUserCreated             = "user_created"
	ActionNameOnboardingStepCompleted = "Onboarding-Step-Completed"

	// CONFIRMED UNAVAILABLE (2026-09-08): authentication/login events are
	// NOT tracked for Asgardeo's own product analytics in any Moesif
	// environment (Prod/Dev/Staging/Test all checked). Per the team's
	// Moesif admin, login/token tracking only exists as a custom,
	// per-customer opt-in feature for specific end-customer orgs — it is
	// not enabled for Asgardeo itself today. These constants are kept so
	// the classification logic below is ready if this ever changes, but
	// they will not match any real data currently.
	ActionNameAuthenticationAttempt = "authentication_attempt"
	ActionNameAPICall               = "api_call"
)

// unavailableSignals lists Summary fields that are confirmed NOT
// obtainable from Moesif today (see ActionNameAuthenticationAttempt /
// ActionNameAPICall above). This is a platform-wide limitation, not a
// per-company one, so every Summary reports the same list.
var unavailableSignals = []string{
	"authenticationAttempts",
	"authenticationSuccessful",
	"apiUsageDetected",
}

// Summary is the normalized, per-customer signal set consumed downstream by
// the PLG backend / Claude interpretation step.
type Summary struct {
	ApplicationCreated       bool   `json:"applicationCreated"`
	AuthenticationAttempts   int    `json:"authenticationAttempts"`
	AuthenticationSuccessful bool   `json:"authenticationSuccessful"`
	ApiUsageDetected         bool   `json:"apiUsageDetected"`
	LastActivity             string `json:"lastActivity"`
	// UnavailableSignals names fields above that are NOT real data today —
	// see the confirmed-unavailable comment on ActionNameAuthenticationAttempt.
	// A consumer should treat these fields' zero-values as "unknown", not
	// "confirmed zero activity".
	UnavailableSignals []string `json:"unavailableSignals"`
}

// Normalize aggregates a slice of raw Moesif hits (already filtered to a
// single company) into a Summary.
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

	summary.UnavailableSignals = unavailableSignals

	return summary
}

// parseMoesifTime parses the observed Moesif request.time format, e.g.
// "2026-09-07T02:00:58.646" — no timezone suffix, treated as UTC.
func parseMoesifTime(raw string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000", raw)
}
