package pgbeam

import (
	"context"
	"fmt"
)

// ProjectsService handles operations on PgBeam projects.
// Mirrors the TypeScript SDK's api.projects.* namespace.
type ProjectsService struct {
	client *Client
}

// Create creates a new project with its primary database atomically.
func (s *ProjectsService) Create(ctx context.Context, req CreateProjectRequest) (*CreateProjectResponse, error) {
	var resp CreateProjectResponse
	if err := s.client.post(ctx, "/v1/projects", req, &resp); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return &resp, nil
}

// Get retrieves a project by ID.
func (s *ProjectsService) Get(ctx context.Context, projectID string) (*Project, error) {
	var resp Project
	if err := s.client.get(ctx, "/v1/projects/"+projectID, &resp); err != nil {
		return nil, fmt.Errorf("get project %s: %w", projectID, err)
	}
	return &resp, nil
}

// ListParams configures pagination for list operations.
type ListParams struct {
	PageSize  int    `json:"page_size,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

// ListProjectsResponse is the response from listing projects.
type ListProjectsResponse struct {
	Projects      []Project `json:"projects"`
	NextPageToken string    `json:"next_page_token,omitempty"`
}

// List lists projects for an organization.
func (s *ProjectsService) List(ctx context.Context, orgID string, params *ListParams) (*ListProjectsResponse, error) {
	qp := map[string]string{"org_id": orgID}
	if params != nil {
		if params.PageSize > 0 {
			qp["page_size"] = fmt.Sprintf("%d", params.PageSize)
		}
		if params.PageToken != "" {
			qp["page_token"] = params.PageToken
		}
	}
	var resp ListProjectsResponse
	if err := s.client.get(ctx, addQueryParams("/v1/projects", qp), &resp); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return &resp, nil
}

// Update updates a project. Only provided fields are modified.
func (s *ProjectsService) Update(ctx context.Context, projectID string, req UpdateProjectRequest) (*Project, error) {
	var resp Project
	if err := s.client.patch(ctx, "/v1/projects/"+projectID, req, &resp); err != nil {
		return nil, fmt.Errorf("update project %s: %w", projectID, err)
	}
	return &resp, nil
}

// Delete soft-deletes a project.
func (s *ProjectsService) Delete(ctx context.Context, projectID string) error {
	if err := s.client.del(ctx, "/v1/projects/"+projectID); err != nil {
		return fmt.Errorf("delete project %s: %w", projectID, err)
	}
	return nil
}
