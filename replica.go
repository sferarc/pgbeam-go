package pgbeam

import (
	"context"
	"fmt"
)

// ReplicasService handles operations on read replicas.
// Mirrors the TypeScript SDK's api.projects.{createReplica,listReplicas,deleteReplica}.
type ReplicasService struct {
	client *Client
}

// Create creates a new replica under a database.
func (s *ReplicasService) Create(ctx context.Context, databaseID string, req CreateReplicaRequest) (*Replica, error) {
	var resp Replica
	if err := s.client.post(ctx, fmt.Sprintf("/v1/databases/%s/replicas", databaseID), req, &resp); err != nil {
		return nil, fmt.Errorf("create replica in database %s: %w", databaseID, err)
	}
	return &resp, nil
}

// Get retrieves a replica by listing all replicas and filtering by ID.
// The PgBeam API does not have an individual GET endpoint for replicas.
func (s *ReplicasService) Get(ctx context.Context, databaseID, replicaID string) (*Replica, error) {
	pageToken := ""
	for {
		params := map[string]string{"page_size": "100"}
		if pageToken != "" {
			params["page_token"] = pageToken
		}

		var resp ListReplicasResponse
		path := addQueryParams(fmt.Sprintf("/v1/databases/%s/replicas", databaseID), params)
		if err := s.client.get(ctx, path, &resp); err != nil {
			return nil, fmt.Errorf("list replicas in database %s: %w", databaseID, err)
		}

		for i := range resp.Replicas {
			if resp.Replicas[i].ID == replicaID {
				return &resp.Replicas[i], nil
			}
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return nil, &APIError{StatusCode: 404, Status: "Not Found", Body: fmt.Sprintf("replica %s not found", replicaID)}
}

// List lists replicas under a database.
func (s *ReplicasService) List(ctx context.Context, databaseID string, params *ListParams) (*ListReplicasResponse, error) {
	qp := map[string]string{}
	if params != nil {
		if params.PageSize > 0 {
			qp["page_size"] = fmt.Sprintf("%d", params.PageSize)
		}
		if params.PageToken != "" {
			qp["page_token"] = params.PageToken
		}
	}
	var resp ListReplicasResponse
	if err := s.client.get(ctx, addQueryParams(fmt.Sprintf("/v1/databases/%s/replicas", databaseID), qp), &resp); err != nil {
		return nil, fmt.Errorf("list replicas in database %s: %w", databaseID, err)
	}
	return &resp, nil
}

// Delete deletes a replica.
func (s *ReplicasService) Delete(ctx context.Context, databaseID, replicaID string) error {
	if err := s.client.del(ctx, fmt.Sprintf("/v1/databases/%s/replicas/%s", databaseID, replicaID)); err != nil {
		return fmt.Errorf("delete replica %s: %w", replicaID, err)
	}
	return nil
}

// Replica represents a read replica endpoint.
type Replica struct {
	ID         string `json:"id"`
	DatabaseID string `json:"database_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	SSLMode    string `json:"ssl_mode,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// CreateReplicaRequest is the request body for creating a replica.
type CreateReplicaRequest struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	SSLMode string `json:"ssl_mode,omitempty"`
}

// ListReplicasResponse is the response from listing replicas.
type ListReplicasResponse struct {
	Replicas      []Replica `json:"replicas"`
	NextPageToken string    `json:"next_page_token,omitempty"`
}
