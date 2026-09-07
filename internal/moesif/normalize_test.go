package moesif

import "testing"

func TestNormalize(t *testing.T) {
	events := []RawEvent{
		{
			CompanyID: "company_456",
			EventType: EventTypeApplicationCreated,
			Status:    200,
			Timestamp: "2026-08-15T09:00:00Z",
		},
		{
			CompanyID: "company_456",
			EventType: EventTypeAuthenticationEvent,
			Status:    200,
			Timestamp: "2026-08-20T10:30:00Z",
		},
		{
			CompanyID: "company_456",
			EventType: EventTypeAuthenticationEvent,
			Status:    401,
			Timestamp: "2026-08-25T14:00:00Z",
		},
		{
			CompanyID: "company_456",
			EventType: EventTypeAPICall,
			Status:    200,
			Timestamp: "2026-08-31T08:00:00Z", // latest — should win as LastActivity
		},
	}

	summary := Normalize(events)

	if !summary.ApplicationCreated {
		t.Error("expected ApplicationCreated to be true")
	}
	if summary.AuthenticationAttempts != 2 {
		t.Errorf("expected AuthenticationAttempts = 2, got %d", summary.AuthenticationAttempts)
	}
	if !summary.AuthenticationSuccessful {
		t.Error("expected AuthenticationSuccessful to be true (one attempt had status 200)")
	}
	if !summary.ApiUsageDetected {
		t.Error("expected ApiUsageDetected to be true")
	}
	if summary.LastActivity != "2026-08-31" {
		t.Errorf("expected LastActivity = 2026-08-31, got %q", summary.LastActivity)
	}
}
