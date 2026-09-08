package moesif

import "testing"

func TestNormalize(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingStepCompleted,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"}, // earliest — should win as FirstSeen
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOrganizationCreated,
				Request:    RawRequest{Time: "2026-08-20T10:30:00.000"},
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-22T11:00:00.000"},
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-25T12:00:00.000"},
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameAPICall,
				Request:    RawRequest{Time: "2026-08-31T08:00:00.000"}, // latest — should win as LastActivity
			},
		},
	}

	summary := Normalize(hits)

	if !summary.ApplicationCreated {
		t.Error("expected ApplicationCreated to be true (via Onboarding-Step-Completed)")
	}
	if summary.LastActivity != "2026-08-31" {
		t.Errorf("expected LastActivity = 2026-08-31, got %q", summary.LastActivity)
	}
	if summary.FirstSeen != "2026-08-15" {
		t.Errorf("expected FirstSeen = 2026-08-15, got %q", summary.FirstSeen)
	}
	if summary.OnboardingSkippedCount != 2 {
		t.Errorf("expected OnboardingSkippedCount = 2, got %d", summary.OnboardingSkippedCount)
	}

	// Confirm the unavailable-signals flag is always present, since
	// authentication/API-usage tracking is a confirmed platform-wide gap.
	wantUnavailable := []string{"authenticationAttempts", "authenticationSuccessful", "apiUsageDetected"}
	if len(summary.UnavailableSignals) != len(wantUnavailable) {
		t.Fatalf("expected %d unavailable signals, got %d: %v", len(wantUnavailable), len(summary.UnavailableSignals), summary.UnavailableSignals)
	}
	for i, want := range wantUnavailable {
		if summary.UnavailableSignals[i] != want {
			t.Errorf("UnavailableSignals[%d] = %q, want %q", i, summary.UnavailableSignals[i], want)
		}
	}
}

// TestNormalize_SingleEvent confirms FirstSeen and LastActivity are equal
// when only one event exists — a simple edge case worth its own check.
func TestNormalize_SingleEvent(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
			},
		},
	}

	summary := Normalize(hits)

	if summary.FirstSeen != "2026-08-15" {
		t.Errorf("expected FirstSeen = 2026-08-15, got %q", summary.FirstSeen)
	}
	if summary.LastActivity != "2026-08-15" {
		t.Errorf("expected LastActivity = 2026-08-15, got %q", summary.LastActivity)
	}
	if summary.OnboardingSkippedCount != 1 {
		t.Errorf("expected OnboardingSkippedCount = 1, got %d", summary.OnboardingSkippedCount)
	}
}
