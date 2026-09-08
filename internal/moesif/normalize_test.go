package moesif

import "testing"

func intPtr(i int) *int {
	return &i
}

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
			// An earlier skip — should NOT win, since a later skip exists below.
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-22T11:00:00.000"},
				Metadata:   RawMetadata{StepNumber: intPtr(0), StepName: "welcome_option_selected"},
			},
		},
		{
			// The most recent skip — this one should win.
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-25T12:00:00.000"},
				Metadata:   RawMetadata{StepNumber: intPtr(3), StepName: "redirect_url_configured"},
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
	if summary.LastSkippedStepNumber == nil {
		t.Fatal("expected LastSkippedStepNumber to be set, got nil")
	}
	if *summary.LastSkippedStepNumber != 3 {
		t.Errorf("expected LastSkippedStepNumber = 3 (the MOST RECENT skip), got %d", *summary.LastSkippedStepNumber)
	}
	if summary.LastSkippedStepName != "redirect_url_configured" {
		t.Errorf("expected LastSkippedStepName = redirect_url_configured, got %q", summary.LastSkippedStepName)
	}

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

func TestNormalize_SingleEvent(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
				Metadata:   RawMetadata{StepNumber: intPtr(1), StepName: "app_name_entered"},
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

// TestNormalize_NoSkip confirms LastSkippedStepNumber stays nil (not 0)
// when no skip ever occurred — since 0 is itself a valid real step, this
// distinction matters.
func TestNormalize_NoSkip(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingStepCompleted,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
			},
		},
	}

	summary := Normalize(hits)

	if summary.OnboardingSkippedCount != 0 {
		t.Errorf("expected OnboardingSkippedCount = 0, got %d", summary.OnboardingSkippedCount)
	}
	if summary.LastSkippedStepNumber != nil {
		t.Errorf("expected LastSkippedStepNumber = nil (never skipped), got %v", *summary.LastSkippedStepNumber)
	}
}

// TestNormalize_SkipAtStepZero confirms step 0 is correctly distinguished
// from "no skip" — a genuine skip at step 0 must show *0, not nil.
func TestNormalize_SkipAtStepZero(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
				Metadata:   RawMetadata{StepNumber: intPtr(0), StepName: "welcome_option_selected"},
			},
		},
	}

	summary := Normalize(hits)

	if summary.LastSkippedStepNumber == nil {
		t.Fatal("expected LastSkippedStepNumber to be set (step 0 is a real skip), got nil")
	}
	if *summary.LastSkippedStepNumber != 0 {
		t.Errorf("expected LastSkippedStepNumber = 0, got %d", *summary.LastSkippedStepNumber)
	}
	if summary.LastSkippedStepName != "welcome_option_selected" {
		t.Errorf("expected LastSkippedStepName = welcome_option_selected, got %q", summary.LastSkippedStepName)
	}
}
