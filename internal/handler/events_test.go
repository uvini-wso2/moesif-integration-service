package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uvini-wso2/moesif-integration-service/internal/moesif"
)

// sampleHits is a small set of realistic hits, matching the confirmed real
// Moesif response shape, used across several test cases below.
func sampleHits() []moesif.RawHit {
	return []moesif.RawHit{
		{
			Source: moesif.RawSource{
				CompanyID:  "company_456",
				UserID:     "user_123",
				ActionName: moesif.ActionNameOnboardingStepCompleted,
				Request:    moesif.RawRequest{Time: "2026-08-20T10:30:00.000"},
			},
		},
	}
}

// Case 1: missing both identifiers must be rejected with 400, before any
// call to Moesif is attempted — protects against silently querying with
// an empty/unbounded filter.
func TestEvents_MissingIdentifiers(t *testing.T) {
	mock := &mockMoesifClient{}
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// Case 2: a valid company_id alone should succeed, and the handler must
// pass CompanyID through to the Moesif client correctly.
func TestEvents_CompanyIDOnly(t *testing.T) {
	mock := &mockMoesifClient{
		Response: moesif.SearchResponse{
			Result: moesif.HitsResult{Hits: sampleHits(), Total: 1},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/events?company_id=company_456", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if mock.LastCriteria.CompanyID != "company_456" {
		t.Errorf("expected CompanyID passed through as %q, got %q", "company_456", mock.LastCriteria.CompanyID)
	}
	if mock.LastCriteria.UserID != "" {
		t.Errorf("expected UserID to be empty, got %q", mock.LastCriteria.UserID)
	}
}

// Case 3: a valid user_id alone should succeed, mirroring case 2.
func TestEvents_UserIDOnly(t *testing.T) {
	mock := &mockMoesifClient{
		Response: moesif.SearchResponse{
			Result: moesif.HitsResult{Hits: sampleHits(), Total: 1},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/events?user_id=user_123", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if mock.LastCriteria.UserID != "user_123" {
		t.Errorf("expected UserID passed through as %q, got %q", "user_123", mock.LastCriteria.UserID)
	}
	if mock.LastCriteria.CompanyID != "" {
		t.Errorf("expected CompanyID to be empty, got %q", mock.LastCriteria.CompanyID)
	}
}

// Case 4: both identifiers together should both be passed through to
// Moesif, combined (BuildPostFilter ANDs them — tested separately in the
// moesif package, but here we just confirm the handler forwards both).
func TestEvents_BothIdentifiers(t *testing.T) {
	mock := &mockMoesifClient{
		Response: moesif.SearchResponse{
			Result: moesif.HitsResult{Hits: sampleHits(), Total: 1},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/events?company_id=company_456&user_id=user_123", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mock.LastCriteria.CompanyID != "company_456" || mock.LastCriteria.UserID != "user_123" {
		t.Errorf("expected both identifiers passed through, got CompanyID=%q UserID=%q",
			mock.LastCriteria.CompanyID, mock.LastCriteria.UserID)
	}
}

// Case 5: if the Moesif client itself fails (network error, auth error,
// etc.), the handler must return 502, not crash or leak the raw error.
func TestEvents_MoesifError(t *testing.T) {
	mock := &mockMoesifClient{
		Err: errors.New("simulated moesif failure"),
	}
	req := httptest.NewRequest(http.MethodGet, "/events?company_id=company_456", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Error("expected a non-empty error body")
	}
}

// Case 6: zero events found must produce eventsFound: 0 and an all-empty
// Summary — this is the "no data exists for this ID" signal, not an error.
func TestEvents_ZeroEventsFound(t *testing.T) {
	mock := &mockMoesifClient{
		Response: moesif.SearchResponse{
			Result: moesif.HitsResult{Hits: []moesif.RawHit{}, Total: 0},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/events?company_id=does-not-exist", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if got := body["eventsFound"]; got != float64(0) {
		t.Errorf("expected eventsFound = 0, got %v", got)
	}
	if got := body["applicationCreated"]; got != false {
		t.Errorf("expected applicationCreated = false, got %v", got)
	}
}

// Case 7: contract check — confirms the exact JSON field names the
// frontend/PLG backend will depend on. If someone accidentally renames a
// field later, this test catches it immediately.
func TestEvents_ResponseFieldNames(t *testing.T) {
	mock := &mockMoesifClient{
		Response: moesif.SearchResponse{
			Result: moesif.HitsResult{Hits: sampleHits(), Total: 1},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/events?company_id=company_456", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	requiredFields := []string{
		"applicationCreated",
		"authenticationAttempts",
		"authenticationSuccessful",
		"apiUsageDetected",
		"lastActivity",
		"firstSeen",
		"onboardingSkippedCount",
		"unavailableSignals",
		"eventsFound",
	}
	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Errorf("expected response to contain field %q, but it was missing", field)
		}
	}
}

// Case 8: an excessively long parameter (e.g. an accidental paste of a
// large blob of text) must be rejected with 400, not silently truncated
// or sent to Moesif as-is.
func TestEvents_ParamTooLong(t *testing.T) {
	mock := &mockMoesifClient{}
	tooLong := strings.Repeat("a", maxParamLength+1)
	req := httptest.NewRequest(http.MethodGet, "/events?company_id="+tooLong, nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

// Case 9: real Moesif user_id values are not always UUID-shaped (confirmed
// against real data — some are anonymous/session-style IDs). The handler
// must accept any reasonably-sized value, not just strict UUIDs.
func TestEvents_NonUUIDUserIDAccepted(t *testing.T) {
	mock := &mockMoesifClient{
		Response: moesif.SearchResponse{
			Result: moesif.HitsResult{Hits: sampleHits(), Total: 1},
		},
	}
	nonUUIDUserID := "1a07f690def2bd-01adf07d5c33fb-1d525630-1d73c0"
	req := httptest.NewRequest(http.MethodGet, "/events?user_id="+nonUUIDUserID, nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for non-UUID user_id, got %d", rec.Code)
	}
	if mock.LastCriteria.UserID != nonUUIDUserID {
		t.Errorf("expected UserID passed through unchanged, got %q", mock.LastCriteria.UserID)
	}
}
