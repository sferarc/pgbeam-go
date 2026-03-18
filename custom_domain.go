package pgbeam

import (
	"context"
	"fmt"
)

// DomainsService handles operations on custom domains.
// Mirrors the TypeScript SDK's api.projects.{createCustomDomain,listCustomDomains,...}.
type DomainsService struct {
	client *Client
}

// Create creates a new custom domain under a project.
// Requires the Scale or Enterprise plan.
func (s *DomainsService) Create(ctx context.Context, projectID string, req CreateCustomDomainRequest) (*CustomDomain, error) {
	var resp CustomDomain
	if err := s.client.post(ctx, fmt.Sprintf("/v1/projects/%s/domains", projectID), req, &resp); err != nil {
		return nil, fmt.Errorf("create custom domain in project %s: %w", projectID, err)
	}
	return &resp, nil
}

// Get retrieves a custom domain by listing all domains and filtering by ID.
// The PgBeam API does not have an individual GET endpoint for custom domains.
func (s *DomainsService) Get(ctx context.Context, projectID, domainID string) (*CustomDomain, error) {
	pageToken := ""
	for {
		params := map[string]string{"page_size": "100"}
		if pageToken != "" {
			params["page_token"] = pageToken
		}

		var resp ListCustomDomainsResponse
		path := addQueryParams(fmt.Sprintf("/v1/projects/%s/domains", projectID), params)
		if err := s.client.get(ctx, path, &resp); err != nil {
			return nil, fmt.Errorf("list custom domains in project %s: %w", projectID, err)
		}

		for i := range resp.Domains {
			if resp.Domains[i].ID == domainID {
				return &resp.Domains[i], nil
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return nil, &APIError{StatusCode: 404, Status: "Not Found", Body: fmt.Sprintf("custom domain %s not found", domainID)}
}

// List lists custom domains under a project.
func (s *DomainsService) List(ctx context.Context, projectID string, params *ListParams) (*ListCustomDomainsResponse, error) {
	qp := map[string]string{}
	if params != nil {
		if params.PageSize > 0 {
			qp["page_size"] = fmt.Sprintf("%d", params.PageSize)
		}
		if params.PageToken != "" {
			qp["page_token"] = params.PageToken
		}
	}
	var resp ListCustomDomainsResponse
	if err := s.client.get(ctx, addQueryParams(fmt.Sprintf("/v1/projects/%s/domains", projectID), qp), &resp); err != nil {
		return nil, fmt.Errorf("list custom domains in project %s: %w", projectID, err)
	}
	return &resp, nil
}

// Delete deletes a custom domain.
func (s *DomainsService) Delete(ctx context.Context, projectID, domainID string) error {
	if err := s.client.del(ctx, fmt.Sprintf("/v1/projects/%s/domains/%s", projectID, domainID)); err != nil {
		return fmt.Errorf("delete custom domain %s: %w", domainID, err)
	}
	return nil
}

// Verify triggers DNS verification for a custom domain.
// Call this after configuring DNS records from the domain's DnsInstructions.
func (s *DomainsService) Verify(ctx context.Context, projectID, domainID string) (*VerifyCustomDomainResponse, error) {
	var resp VerifyCustomDomainResponse
	if err := s.client.post(ctx, fmt.Sprintf("/v1/projects/%s/domains/%s/verify", projectID, domainID), nil, &resp); err != nil {
		return nil, fmt.Errorf("verify custom domain %s: %w", domainID, err)
	}
	return &resp, nil
}
