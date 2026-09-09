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
	// though nothing currently classifies on them.
	ActionNameOrganizationCreated     = "organization_created"
	ActionNameOrganizationSubscribed  = "organization_subscribed"
	ActionNameUserCreated             = "user_created"
	ActionNameOnboardingStepCompleted = "Onboarding-Step-Completed"
	ActionNameOnboardingSkipped       = "Onboarding-Skipped"

	// CONFIRMED UNAVAILABLE for Asgardeo (2026-09-08): authentication/login
	// events are NOT tracked for Asgardeo's own product analytics in any
	// Moesif environment checked (Prod/Dev/Staging/Test). Per the team's
	// Moesif admin, login/token tracking only exists as a custom,
	// per-customer opt-in feature for specific end-customer orgs.
	//
	// NOTE: this is Asgardeo-specific, not a Moesif-wide limitation — the
	// other 4 SaaS products this API will eventually cover DO have this
	// kind of data. These constants are kept here as reference for when
	// another product's classification logic is built (likely its own
	// ProductActivity-equivalent type), even though Asgardeo's
	// ProductActivity below deliberately has no field for them.
	ActionNameAuthenticationAttempt = "authentication_attempt"
	ActionNameAPICall               = "api_call"
)

// timeOutputLayout formats output timestamps as UTC (no offset), matching
// the assumption used when parsing Moesif's request.time (see
// parseMoesifTime) — Moesif's raw timestamps carry no timezone suffix and
// are treated as UTC throughout.
const timeOutputLayout = "2006-01-02T15:04:05Z"

// ProductActivity holds signals specific to THIS product (Asgardeo). Per
// team decision (2026-09-09): each product gets its own child-object shape
// under Summary.ProductActivity, since different products have different
// concepts — e.g. only Asgardeo has "onboarding steps". Fields here should
// NOT be assumed to apply to any other product's ProductActivity shape.
type ProductActivity struct {
	ApplicationCreated bool `json:"applicationCreated"`
	// HasSkippedOnboarding is true if at least one Onboarding-Skipped event
	// occurred. A boolean per team decision (2026-09-09) — a count wasn't
	// considered meaningful enough to track separately.
	HasSkippedOnboarding bool `json:"hasSkippedOnboarding"`
	// SkippedStepNumber / SkippedStepName describe the step at which
	// onboarding was skipped. nil/"" if no skip has occurred. If multiple
	// skip events somehow exist, the chronologically most recent one is
	// used as the representative value (HasSkippedOnboarding intentionally
	// doesn't track a count — see team decision 2026-09-09). StepNumber is
	// a pointer because step 0 ("welcome_option_selected") is a real,
	// valid step — nil means "never skipped", not "skipped at step 0".
	SkippedStepNumber *int   `json:"skippedStepNumber"`
	SkippedStepName   string `json:"skippedStepName,omitempty"`
}

// Summary is the normalized, per-customer signal set consumed downstream by
// the PLG backend / Claude interpretation step. Fields here are meant to
// stay CONSISTENT across all products (per team decision 2026-09-09);
// product-specific signals live in ProductActivity instead.
type Summary struct {
	// OrganizationName comes from whichever event in the set happens to
	// carry company.metadata.account_name — not every event includes it,
	// so this is best-effort. Omitted from JSON if never found.
	OrganizationName string `json:"organizationName,omitempty"`
	// FirstSeen / LastActivity are full timestamps (not just dates) of the
	// earliest/latest event found, across ALL event types — not just
	// recognized ones. Together they show overall tenure.
	FirstSeen    string `json:"firstSeen"`
	LastActivity string `json:"lastActivity"`
	// AverageTimePerActiveDayMinutes averages (last-event-time minus
	// first-event-time) across every calendar day that had 2+ events. Days
	// with only 1 event are excluded — a single event can't establish a
	// real duration, and counting it as 0 would understate activity
	// dishonestly. nil/omitted if no day had 2+ events.
	AverageTimePerActiveDayMinutes *float64        `json:"averageTimePerActiveDayMinutes,omitempty"`
	ProductActivity                ProductActivity `json:"productActivity"`
}

// Normalize aggregates a slice of raw Moesif hits (already filtered to a
// single company/user) into a Summary.
func Normalize(hits []RawHit) Summary {
	var summary Summary
	var earliest, latest time.Time
	var latestSkipTime time.Time
	var skippedCount int

	// Per-calendar-day first/last event time and count, for
	// AverageTimePerActiveDayMinutes.
	dayFirst := map[string]time.Time{}
	dayLast := map[string]time.Time{}
	dayCount := map[string]int{}

	for _, hit := range hits {
		src := hit.Source

		if summary.OrganizationName == "" && src.Company.Metadata.AccountName != "" {
			summary.OrganizationName = src.Company.Metadata.AccountName
		}

		eventTime, timeErr := parseMoesifTime(src.Request.Time)

		switch src.ActionName {
		case ActionNameOnboardingStepCompleted:
			summary.ProductActivity.ApplicationCreated = true
		case ActionNameOnboardingSkipped:
			skippedCount++
			if timeErr == nil && (latestSkipTime.IsZero() || eventTime.After(latestSkipTime)) {
				latestSkipTime = eventTime
				summary.ProductActivity.SkippedStepNumber = src.Metadata.StepNumber
				summary.ProductActivity.SkippedStepName = src.Metadata.StepName
			}
		}

		if timeErr == nil {
			if eventTime.After(latest) {
				latest = eventTime
			}
			if earliest.IsZero() || eventTime.Before(earliest) {
				earliest = eventTime
			}

			dayKey := eventTime.Format("2006-01-02")
			dayCount[dayKey]++
			if t, ok := dayFirst[dayKey]; !ok || eventTime.Before(t) {
				dayFirst[dayKey] = eventTime
			}
			if t, ok := dayLast[dayKey]; !ok || eventTime.After(t) {
				dayLast[dayKey] = eventTime
			}
		}
	}

	summary.ProductActivity.HasSkippedOnboarding = skippedCount > 0

	if !latest.IsZero() {
		summary.LastActivity = latest.Format(timeOutputLayout)
	}
	if !earliest.IsZero() {
		summary.FirstSeen = earliest.Format(timeOutputLayout)
	}

	var totalMinutes float64
	var activeDayCount int
	for day, count := range dayCount {
		if count < 2 {
			continue // can't establish a real duration from a single event
		}
		totalMinutes += dayLast[day].Sub(dayFirst[day]).Minutes()
		activeDayCount++
	}
	if activeDayCount > 0 {
		avg := totalMinutes / float64(activeDayCount)
		summary.AverageTimePerActiveDayMinutes = &avg
	}

	return summary
}

// parseMoesifTime parses the observed Moesif request.time format, e.g.
// "2026-09-07T02:00:58.646" — no timezone suffix, treated as UTC.
func parseMoesifTime(raw string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05.000", raw)
}
