package moesif

import "time"

// Action name values for Moesif's "action_name" field.
const (
	// CONFIRMED — observed in real Moesif responses for Asgardeo activity,
	// across Prod/Dev/Staging/Test environments (2026-09-08).
	//
	// Only ActionNameOnboardingStepCompleted and ActionNameOnboardingSkipped
	// are currently wired into Normalize()'s classification below.
	// OrganizationCreated, OrganizationSubscribed, and UserCreated are kept
	// here as confirmed real values (useful reference/building blocks) even
	// though nothing currently classifies on them — not dead code, just not
	// yet needed for any Summary field.
	ActionNameOrganizationCreated     = "organization_created"
	ActionNameOrganizationSubscribed  = "organization_subscribed"
	ActionNameUserCreated             = "user_created"
	ActionNameOnboardingStepCompleted = "Onboarding-Step-Completed"
	ActionNameOnboardingSkipped       = "Onboarding-Skipped"

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
	// FirstSeen is the date of the EARLIEST event found, across all event
	// types. Combined with LastActivity, this gives overall tenure — how
	// long this account has existed and whether it's still active.
	FirstSeen string `json:"firstSeen"`
	// OnboardingSkippedCount is how many times this company/user triggered
	// an Onboarding-Skipped event.
	OnboardingSkippedCount int `json:"onboardingSkippedCount"`
	// LastSkippedStepNumber / LastSkippedStepName describe the step at
	// which the MOST RECENT skip occurred (chronologically last skip, not
	// first). nil/"" if no skip has occurred. StepNumber is a pointer
	// because step 0 ("welcome_option_selected") is a real, valid step —
	// nil means "never skipped", not "skipped at step 0".
	LastSkippedStepNumber *int   `json:"lastSkippedStepNumber"`
	LastSkippedStepName   string `json:"lastSkippedStepName,omitempty"`
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
	var earliest, latest time.Time
	var latestSkipTime time.Time

	for _, hit := range hits {
		src := hit.Source

		eventTime, timeErr := parseMoesifTime(src.Request.Time)

		switch src.ActionName {
		case ActionNameOnboardingStepCompleted:
			summary.ApplicationCreated = true
		case ActionNameOnboardingSkipped:
			summary.OnboardingSkippedCount++
			// Track the step info from whichever skip is chronologically
			// most recent, not just the last one encountered in the slice
			// (hits are not guaranteed to arrive in time order).
			if timeErr == nil && (latestSkipTime.IsZero() || eventTime.After(latestSkipTime)) {
				latestSkipTime = eventTime
				summary.LastSkippedStepNumber = src.Metadata.StepNumber
				summary.LastSkippedStepName = src.Metadata.StepName
			}
		case ActionNameAuthenticationAttempt:
			summary.AuthenticationAttempts++
		case ActionNameAPICall:
			summary.ApiUsageDetected = true
		}

		if timeErr == nil {
			if eventTime.After(latest) {
				latest = eventTime
			}
			if earliest.IsZero() || eventTime.Before(earliest) {
				earliest = eventTime
			}
		}
	}

	if !latest.IsZero() {
		summary.LastActivity = latest.Format("2006-01-02")
	}
	if !earliest.IsZero() {
		summary.FirstSeen = earliest.Format("2006-01-02")
	}

	summary.UnavailableSignals = unavailableSignals

	return summary
}

// parseMoesifTime parses the observed Moesif request.time format, e.g.
// "2026-09-07T02:00:58.646" — no timezone suffix, treated as UTC.
func parseMoesifTime(raw string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000", raw)
}
