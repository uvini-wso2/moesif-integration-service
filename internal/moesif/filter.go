package moesif

// FilterCriteria describes what subset of Moesif events to retrieve.
// At least one of CompanyID or UserID must be set.
type FilterCriteria struct {
	CompanyID   string   // Moesif "company_id" — optional if UserID is set
	UserID      string   // Moesif "user_id" — optional if CompanyID is set
	ActionTypes []string // optional — if empty, no action-type filter is applied
	From        string   // Moesif relative/absolute time, e.g. "-30d"
	To          string   // e.g. "now"
}

// actionField is the Moesif event field carrying the specific action name
// (e.g. "organization_created", "user_created") for Asgardeo activity.
//
// CONFIRMED against real Moesif responses on 2026-09-07/08 — "event_type"
// is always "user_action" and is NOT useful for distinguishing action
// types; "action_name" is the field that actually varies.
const actionField = "action_name"

// BuildPostFilter turns a FilterCriteria into the Elasticsearch-style
// post_filter DSL Moesif's Search API expects. CompanyID and UserID are
// combined with AND when both are set; either alone is also valid.
func BuildPostFilter(criteria FilterCriteria) map[string]interface{} {
	must := []map[string]interface{}{}

	if criteria.CompanyID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"company_id": criteria.CompanyID,
			},
		})
	}

	if criteria.UserID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"user_id": criteria.UserID,
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
