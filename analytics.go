package pgbeam

import (
	"context"
	"fmt"
	"time"
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

// OrganizationPlan represents an organization's billing plan.
type OrganizationPlan struct {
	OrgID              string     `json:"org_id"`
	Plan               string     `json:"plan"`
	BillingProvider    *string    `json:"billing_provider,omitempty"`
	SubscriptionStatus *string    `json:"subscription_status,omitempty"`
	CurrentPeriodEnd   *time.Time `json:"current_period_end,omitempty"`
	Enabled            *bool      `json:"enabled,omitempty"`
	CustomPricing      *bool      `json:"custom_pricing,omitempty"`
	SpendLimit         *float64   `json:"spend_limit"`
	Limits             PlanLimits `json:"limits"`
	CreatedAt          *string    `json:"created_at,omitempty"`
	UpdatedAt          *string    `json:"updated_at,omitempty"`
}

// PlanLimits defines the limits for a billing plan.
type PlanLimits struct {
	QueriesPerDay    int64 `json:"queries_per_day"`
	MaxProjects      int   `json:"max_projects"`
	MaxDatabases     int   `json:"max_databases"`
	MaxConnections   int   `json:"max_connections"`
	QueriesPerSecond int   `json:"queries_per_second"`
	BytesPerMonth    int64 `json:"bytes_per_month"`
	MaxQueryShapes   int   `json:"max_query_shapes"`
	IncludedSeats    int   `json:"included_seats"`
}

// UpdateSpendLimitRequest is the request body for updating a spend limit.
type UpdateSpendLimitRequest struct {
	SpendLimit *float64 `json:"spend_limit"`
}
