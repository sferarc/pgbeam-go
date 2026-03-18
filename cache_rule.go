package pgbeam

import (
	"context"
	"fmt"
)

// CacheRulesService handles operations on per-query cache rules.
// Mirrors the TypeScript SDK's api.projects.{listCacheRules,updateCacheRule}.
type CacheRulesService struct {
	client *Client
}

// Get retrieves a cache rule by listing all rules and filtering by query hash.
// The PgBeam API does not have an individual GET endpoint for cache rules.
func (s *CacheRulesService) Get(ctx context.Context, projectID, databaseID, queryHash string) (*CacheRule, error) {
	pageToken := ""
	for {
		params := map[string]string{"page_size": "100"}
		if pageToken != "" {
			params["page_token"] = pageToken
		}

		var resp ListCacheRulesResponse
		path := addQueryParams(fmt.Sprintf("/v1/projects/%s/databases/%s/cache-rules", projectID, databaseID), params)
		if err := s.client.get(ctx, path, &resp); err != nil {
			return nil, fmt.Errorf("list cache rules: %w", err)
		}

		for i := range resp.Entries {
			if resp.Entries[i].QueryHash == queryHash {
				return &resp.Entries[i], nil
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return nil, &APIError{StatusCode: 404, Status: "Not Found", Body: fmt.Sprintf("cache rule %s not found", queryHash)}
}

// List lists cache rules for a database.
func (s *CacheRulesService) List(ctx context.Context, projectID, databaseID string, params *ListParams) (*ListCacheRulesResponse, error) {
	qp := map[string]string{}
	if params != nil {
		if params.PageSize > 0 {
			qp["page_size"] = fmt.Sprintf("%d", params.PageSize)
		}
		if params.PageToken != "" {
			qp["page_token"] = params.PageToken
		}
	}
	var resp ListCacheRulesResponse
	path := addQueryParams(fmt.Sprintf("/v1/projects/%s/databases/%s/cache-rules", projectID, databaseID), qp)
	if err := s.client.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("list cache rules: %w", err)
	}
	return &resp, nil
}

// Update creates or updates a cache rule for a specific query hash.
func (s *CacheRulesService) Update(ctx context.Context, projectID, databaseID, queryHash string, req UpdateCacheRuleRequest) (*CacheRule, error) {
	var resp UpdateCacheRuleResponse
	path := fmt.Sprintf("/v1/projects/%s/databases/%s/cache-rules/%s", projectID, databaseID, queryHash)
	if err := s.client.put(ctx, path, req, &resp); err != nil {
		return nil, fmt.Errorf("update cache rule %s: %w", queryHash, err)
	}
	return &resp.Entry, nil
}

// Disable disables caching for a query shape (soft delete).
// The PgBeam API does not have a DELETE endpoint for cache rules.
func (s *CacheRulesService) Disable(ctx context.Context, projectID, databaseID, queryHash string) error {
	req := UpdateCacheRuleRequest{CacheEnabled: false}
	path := fmt.Sprintf("/v1/projects/%s/databases/%s/cache-rules/%s", projectID, databaseID, queryHash)
	if err := s.client.put(ctx, path, req, nil); err != nil {
		return fmt.Errorf("disable cache rule %s: %w", queryHash, err)
	}
	return nil
}
