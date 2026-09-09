package moesif

import "testing"

func intPtr(i int) *int {
	return &i
}

// TestNormalize exercises the full aggregation across multiple days and
// multiple skips, confirming: org name extraction, full timestamp output,
// ApplicationCreated, HasSkippedOnboarding + which skip "wins" (the
// chronologically most recent one, not just the last one in the slice),
// and AverageTimePerActiveDayMinutes only counting days with 2+ events.
func TestNormalize(t *testing.T) {
	hits := []RawHit{
		{
			// Day A (2026-08-15): only 1 event — should NOT count toward
			// the average (a single event can't establish a duration).
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingStepCompleted,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"}, // earliest — FirstSeen
			},
		},
		{
			// Day B (2026-08-20): 2 events, 15 min apart — an active day.
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOrganizationCreated,
				Request:    RawRequest{Time: "2026-08-20T10:00:00.000"},
				Company:    RawCompany{Metadata: RawCompanyMetadata{AccountName: "test-org"}},
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-20T10:15:00.000"},
				Metadata:   RawMetadata{StepNumber: intPtr(0), StepName: "welcome_option_selected"},
			},
		},
		{
			// Day C (2026-08-25): only 1 event, but chronologically the
			// MOST RECENT skip — must still win for LastSkipped*, even
			// though this day doesn't count toward the average.
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOnboardingSkipped,
				Request:    RawRequest{Time: "2026-08-25T12:00:00.000"},
				Metadata:   RawMetadata{StepNumber: intPtr(3), StepName: "redirect_url_configured"},
			},
		},
		{
			// Day D (2026-08-31): 2 events, 20 min apart — an active day,
			// and the latest overall (LastActivity).
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOrganizationSubscribed,
				Request:    RawRequest{Time: "2026-08-31T08:00:00.000"},
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameUserCreated,
				Request:    RawRequest{Time: "2026-08-31T08:20:00.000"}, // latest — LastActivity
			},
		},
	}

	summary := Normalize(hits)

	if summary.OrganizationName != "test-org" {
		t.Errorf("expected OrganizationName = test-org, got %q", summary.OrganizationName)
	}
	if summary.FirstSeen != "2026-08-15T09:00:00Z" {
		t.Errorf("expected FirstSeen = 2026-08-15T09:00:00Z, got %q", summary.FirstSeen)
	}
	if summary.LastActivity != "2026-08-31T08:20:00Z" {
		t.Errorf("expected LastActivity = 2026-08-31T08:20:00Z, got %q", summary.LastActivity)
	}

	if summary.AverageTimePerActiveDayMinutes == nil {
		t.Fatal("expected AverageTimePerActiveDayMinutes to be set, got nil")
	}
	// Day B: 15 min. Day D: 20 min. Average = 17.5.
	if *summary.AverageTimePerActiveDayMinutes != 17.5 {
		t.Errorf("expected AverageTimePerActiveDayMinutes = 17.5, got %v", *summary.AverageTimePerActiveDayMinutes)
	}

	if !summary.ProductActivity.ApplicationCreated {
		t.Error("expected ProductActivity.ApplicationCreated to be true")
	}
	if !summary.ProductActivity.HasSkippedOnboarding {
		t.Error("expected ProductActivity.HasSkippedOnboarding to be true")
	}
	if summary.ProductActivity.SkippedStepNumber == nil {
		t.Fatal("expected LastSkippedStepNumber to be set, got nil")
	}
	// The CHRONOLOGICALLY latest skip is the Aug 25 one (step 3), even
	// though the Aug 20 skip (step 0) has an earlier day-of-week ordering
	// in the slice — Normalize must track by actual event time, not slice
	// order.
	if *summary.ProductActivity.SkippedStepNumber != 3 {
		t.Errorf("expected SkippedStepNumber = 3 (the chronologically latest skip), got %d", *summary.ProductActivity.SkippedStepNumber)
	}
	if summary.ProductActivity.SkippedStepName != "redirect_url_configured" {
		t.Errorf("expected LastSkippedStepName = redirect_url_configured, got %q", summary.ProductActivity.SkippedStepName)
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

	if summary.FirstSeen != "2026-08-15T09:00:00Z" {
		t.Errorf("expected FirstSeen = 2026-08-15T09:00:00Z, got %q", summary.FirstSeen)
	}
	if summary.LastActivity != "2026-08-15T09:00:00Z" {
		t.Errorf("expected LastActivity = 2026-08-15T09:00:00Z, got %q", summary.LastActivity)
	}
	if !summary.ProductActivity.HasSkippedOnboarding {
		t.Error("expected HasSkippedOnboarding = true")
	}
	// A single event can't establish a duration — average should stay nil.
	if summary.AverageTimePerActiveDayMinutes != nil {
		t.Errorf("expected AverageTimePerActiveDayMinutes = nil (only 1 event total), got %v", *summary.AverageTimePerActiveDayMinutes)
	}
}

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

	if summary.ProductActivity.HasSkippedOnboarding {
		t.Error("expected HasSkippedOnboarding = false")
	}
	if summary.ProductActivity.SkippedStepNumber != nil {
		t.Errorf("expected SkippedStepNumber = nil (never skipped), got %v", *summary.ProductActivity.SkippedStepNumber)
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

	if summary.ProductActivity.SkippedStepNumber == nil {
		t.Fatal("expected LastSkippedStepNumber to be set (step 0 is a real skip), got nil")
	}
	if *summary.ProductActivity.SkippedStepNumber != 0 {
		t.Errorf("expected LastSkippedStepNumber = 0, got %d", *summary.ProductActivity.SkippedStepNumber)
	}
}

// TestNormalize_NoOrganizationName confirms OrganizationName stays empty
// (omitted from JSON via omitempty) when no event carries it — rather
// than defaulting to some placeholder string.
func TestNormalize_NoOrganizationName(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameUserCreated,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
			},
		},
	}

	summary := Normalize(hits)

	if summary.OrganizationName != "" {
		t.Errorf("expected OrganizationName = \"\" (no event carried it), got %q", summary.OrganizationName)
	}
}

// TestNormalize_NoActiveDays confirms AverageTimePerActiveDayMinutes stays
// nil when every day only ever had a single event — no day qualifies as
// "active" under the 2+ events rule.
func TestNormalize_NoActiveDays(t *testing.T) {
	hits := []RawHit{
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameOrganizationCreated,
				Request:    RawRequest{Time: "2026-08-15T09:00:00.000"},
			},
		},
		{
			Source: RawSource{
				CompanyID:  "company_456",
				ActionName: ActionNameUserCreated,
				Request:    RawRequest{Time: "2026-08-20T10:00:00.000"}, // different day
			},
		},
	}

	summary := Normalize(hits)

	if summary.AverageTimePerActiveDayMinutes != nil {
		t.Errorf("expected AverageTimePerActiveDayMinutes = nil (no day had 2+ events), got %v", *summary.AverageTimePerActiveDayMinutes)
	}
}
