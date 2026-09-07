package moesif

// FilterCriteria describes what subset of Moesif events to retrieve —
// combining a specific customer/account with specific action types.
type FilterCriteria struct {
	CompanyID   string   // Moesif "company_id" — the customer/account to scope to
	ActionTypes []string // e.g. []string{"organization_created", "user_created"}
	From        string   // Moesif relative/absolute time, e.g. "-30d"
	To          string   // e.g. "now"
}

// actionField is the Moesif event field carrying the specific action name
// (e.g. "organization_created", "user_created") for Asgardeo activity.
//
// CONFIRMED against a real Moesif response on 2026-09-07 — "event_type" is
// always "user_action" and is NOT useful for distinguishing action types;
// "action_name" is the field that actually varies.
const actionField = "action_name"

// BuildPostFilter turns a FilterCriteria into the Elasticsearch-style
// post_filter DSL Moesif's Search API expects, requiring a match on BOTH
// company_id AND at least one of the given action types.
func BuildPostFilter(criteria FilterCriteria) map[string]interface{} {
	must := []map[string]interface{}{}

	if criteria.CompanyID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"company_id": criteria.CompanyID,
			},
		})
	}

	if len(criteria.ActionTypes) > 0 {
		must = append(must, map[string]interface{}{
			"terms": map[string]interface{}{
				actionField: criteria.ActionTypes,
			},
		})
	}

	return map[string]interface{}{
		"bool": map[string]interface{}{
			"must": must,
		},
	}
}
