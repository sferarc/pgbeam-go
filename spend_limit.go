package pgbeam

import (
	"context"
	"fmt"
)

// AnalyticsService handles operations on organization plans and spend limits.
// Mirrors the TypeScript SDK's api.analytics.* namespace.
type AnalyticsService struct {
	client *Client
}

// GetOrganizationPlan retrieves the organization's plan, which includes the spend limit.
func (s *AnalyticsService) GetOrganizationPlan(ctx context.Context, orgID string) (*OrganizationPlan, error) {
	var resp OrganizationPlan
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s/plan", orgID), &resp); err != nil {
		return nil, fmt.Errorf("get organization plan %s: %w", orgID, err)
	}
	return &resp, nil
}

// UpdateSpendLimit sets or removes the spend limit for an organization.
// Pass nil SpendLimit to remove the cap.
func (s *AnalyticsService) UpdateSpendLimit(ctx context.Context, orgID string, req UpdateSpendLimitRequest) (*OrganizationPlan, error) {
	var resp OrganizationPlan
	if err := s.client.put(ctx, fmt.Sprintf("/v1/organizations/%s/spend-limit", orgID), req, &resp); err != nil {
		return nil, fmt.Errorf("update spend limit for org %s: %w", orgID, err)
	}
	return &resp, nil
}

// RemoveSpendLimit removes the spend limit by setting it to nil (no cap).
func (s *AnalyticsService) RemoveSpendLimit(ctx context.Context, orgID string) error {
	req := UpdateSpendLimitRequest{SpendLimit: nil}
	if err := s.client.put(ctx, fmt.Sprintf("/v1/organizations/%s/spend-limit", orgID), req, nil); err != nil {
		return fmt.Errorf("remove spend limit for org %s: %w", orgID, err)
	}
	return nil
}
