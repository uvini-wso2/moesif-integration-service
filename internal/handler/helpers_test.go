package handler

import "github.com/uvini-wso2/moesif-integration-service/internal/moesif"

// mockMoesifClient is a test double for eventsClient. Configure Response
// and Err before use in a test; LastCriteria captures the FilterCriteria
// passed to the most recent Search call, so tests can assert the handler
// built the right query.
type mockMoesifClient struct {
	Response     moesif.SearchResponse
	Err          error
	LastCriteria moesif.FilterCriteria
}

func (m *mockMoesifClient) Search(criteria moesif.FilterCriteria) (moesif.SearchResponse, error) {
	m.LastCriteria = criteria
	if m.Err != nil {
		return moesif.SearchResponse{}, m.Err
	}
	return m.Response, nil
}
