package moesif

// FilterCriteria describes what subset of Moesif events to retrieve —
// combining a specific customer/account with specific action types.
type FilterCriteria struct {
	CompanyID   string   // Moesif "company_id" — the customer/account to scope to
	ActionTypes []string // e.g. []string{"application_created", "authentication_attempt"}
	From        string   // Moesif relative/absolute time, e.g. "-30d"
	To          string   // e.g. "now"
}

// actionField is the Moesif event field carrying the event/action type
// (e.g. "application_created", "api_call") for Asgardeo activity.
//
// ASSUMPTION — based on the sample raw event shape discussed with the team,
// not yet verified against a real Moesif response. Confirm once the real API
// key is available and a sample response can be inspected.
const actionField = "event_type"

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
