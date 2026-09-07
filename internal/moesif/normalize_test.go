package moesif

import "testing"

func TestNormalize(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingStepCompleted,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
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
			// Testing our current (unconfirmed) api_call mapping logic —
			// not yet verified this string matches a real Moesif event.
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
	if !summary.ApiUsageDetected {
		t.Error("expected ApiUsageDetected to be true")
	}
	if summary.LastActivity != "2026-08-31" {
		t.Errorf("expected LastActivity = 2026-08-31, got %q", summary.LastActivity)
	}
	// AuthenticationAttempts / AuthenticationSuccessful are intentionally
	// not asserted here — no confirmed real-data mapping exists yet.
}
