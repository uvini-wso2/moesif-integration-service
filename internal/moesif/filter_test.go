package moesif

import "testing"

// hasTermFilter checks whether the given post_filter's "bool.must" array
// contains a "term" clause matching field=value.
func hasTermFilter(postFilter map[string]interface{}, field, value string) bool {
	must, ok := postFilter["bool"].(map[string]interface{})["must"].([]map[string]interface{})
	if !ok {
		return false
	}
	for _, clause := range must {
		term, ok := clause["term"].(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := term[field].(string); ok && v == value {
			return true
		}
	}
	return false
}

// hasTermsFilter checks whether the "bool.must" array contains a "terms"
// clause for the given field.
func hasTermsFilter(postFilter map[string]interface{}, field string) bool {
	must, ok := postFilter["bool"].(map[string]interface{})["must"].([]map[string]interface{})
	if !ok {
		return false
	}
	for _, clause := range must {
		if _, ok := clause["terms"].(map[string]interface{}); ok {
			return true
		}
	}
	return false
}

func mustCount(t *testing.T, postFilter map[string]interface{}) int {
	t.Helper()
	must, ok := postFilter["bool"].(map[string]interface{})["must"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected bool.must to be []map[string]interface{}")
	}
	return len(must)
}

func TestBuildPostFilter_CompanyIDOnly(t *testing.T) {
	pf := BuildPostFilter(FilterCriteria{CompanyID: "company_456"})

	if mustCount(t, pf) != 1 {
		t.Fatalf("expected exactly 1 must clause, got %d", mustCount(t, pf))
	}
	if !hasTermFilter(pf, "company_id", "company_456") {
		t.Error("expected a term filter on company_id=company_456")
	}
}

func TestBuildPostFilter_UserIDOnly(t *testing.T) {
	pf := BuildPostFilter(FilterCriteria{UserID: "user_123"})

	if mustCount(t, pf) != 1 {
		t.Fatalf("expected exactly 1 must clause, got %d", mustCount(t, pf))
	}
	if !hasTermFilter(pf, "user_id", "user_123") {
		t.Error("expected a term filter on user_id=user_123")
	}
}

func TestBuildPostFilter_BothIdentifiers(t *testing.T) {
	pf := BuildPostFilter(FilterCriteria{CompanyID: "company_456", UserID: "user_123"})

	if mustCount(t, pf) != 2 {
		t.Fatalf("expected exactly 2 must clauses (AND), got %d", mustCount(t, pf))
	}
	if !hasTermFilter(pf, "company_id", "company_456") {
		t.Error("expected a term filter on company_id=company_456")
	}
	if !hasTermFilter(pf, "user_id", "user_123") {
		t.Error("expected a term filter on user_id=user_123")
	}
}

func TestBuildPostFilter_WithActionTypes(t *testing.T) {
	pf := BuildPostFilter(FilterCriteria{
		CompanyID:   "company_456",
		ActionTypes: []string{"organization_created", "user_created"},
	})

	if mustCount(t, pf) != 2 {
		t.Fatalf("expected exactly 2 must clauses (company_id + action types), got %d", mustCount(t, pf))
	}
	if !hasTermsFilter(pf, actionField) {
		t.Errorf("expected a terms filter on %q", actionField)
	}
}

func TestBuildPostFilter_NoActionTypes(t *testing.T) {
	pf := BuildPostFilter(FilterCriteria{CompanyID: "company_456"})

	if hasTermsFilter(pf, actionField) {
		t.Error("expected NO terms filter on action_name when ActionTypes is empty")
	}
}
