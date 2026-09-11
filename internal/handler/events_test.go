package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uvini-wso2/plg-activity-service/internal/moesif"
)

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

func TestEvents_MissingIdentifiers(t *testing.T) {
	mock := &mockMoesifClient{}
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()

	Events(mock)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

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

	productActivity, ok := body["productActivity"].(map[string]interface{})
	if !ok {
		t.Fatal("expected productActivity to be an object")
	}
	if got := productActivity["applicationCreated"]; got != false {
		t.Errorf("expected productActivity.applicationCreated = false, got %v", got)
	}
}

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

	requiredTopLevel := []string{"firstSeen", "lastActivity", "productActivity", "eventsFound"}
	for _, field := range requiredTopLevel {
		if _, ok := body[field]; !ok {
			t.Errorf("expected response to contain field %q, but it was missing", field)
		}
	}

	productActivity, ok := body["productActivity"].(map[string]interface{})
	if !ok {
		t.Fatal("expected productActivity to be an object")
	}
	requiredProductActivityFields := []string{"applicationCreated", "hasSkippedOnboarding", "skippedStepNumber"}
	for _, field := range requiredProductActivityFields {
		if _, ok := productActivity[field]; !ok {
			t.Errorf("expected productActivity to contain field %q, but it was missing", field)
		}
	}
}

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
