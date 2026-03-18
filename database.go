package pgbeam

import (
	"context"
	"fmt"
)

// DatabasesService handles operations on upstream databases.
// Mirrors the TypeScript SDK's api.databases.* namespace.
type DatabasesService struct {
	client *Client
}

// Create creates a new database under a project.
func (s *DatabasesService) Create(ctx context.Context, projectID string, req CreateDatabaseRequest) (*Database, error) {
	var resp Database
	if err := s.client.post(ctx, fmt.Sprintf("/v1/projects/%s/databases", projectID), req, &resp); err != nil {
		return nil, fmt.Errorf("create database in project %s: %w", projectID, err)
	}
	return &resp, nil
}

// Get retrieves a database by ID.
func (s *DatabasesService) Get(ctx context.Context, projectID, databaseID string) (*Database, error) {
	var resp Database
	if err := s.client.get(ctx, fmt.Sprintf("/v1/projects/%s/databases/%s", projectID, databaseID), &resp); err != nil {
		return nil, fmt.Errorf("get database %s: %w", databaseID, err)
	}
	return &resp, nil
}

// List lists databases under a project.
func (s *DatabasesService) List(ctx context.Context, projectID string, params *ListParams) (*ListDatabasesResponse, error) {
	qp := map[string]string{}
	if params != nil {
		if params.PageSize > 0 {
			qp["page_size"] = fmt.Sprintf("%d", params.PageSize)
		}
		if params.PageToken != "" {
			qp["page_token"] = params.PageToken
		}
	}
	var resp ListDatabasesResponse
	if err := s.client.get(ctx, addQueryParams(fmt.Sprintf("/v1/projects/%s/databases", projectID), qp), &resp); err != nil {
		return nil, fmt.Errorf("list databases in project %s: %w", projectID, err)
	}
	return &resp, nil
}

// Update updates a database. Only provided fields are modified.
func (s *DatabasesService) Update(ctx context.Context, projectID, databaseID string, req UpdateDatabaseRequest) (*Database, error) {
	var resp Database
	if err := s.client.patch(ctx, fmt.Sprintf("/v1/projects/%s/databases/%s", projectID, databaseID), req, &resp); err != nil {
		return nil, fmt.Errorf("update database %s: %w", databaseID, err)
	}
	return &resp, nil
}

// Delete deletes a database.
func (s *DatabasesService) Delete(ctx context.Context, projectID, databaseID string) error {
	if err := s.client.del(ctx, fmt.Sprintf("/v1/projects/%s/databases/%s", projectID, databaseID)); err != nil {
		return fmt.Errorf("delete database %s: %w", databaseID, err)
	}
	return nil
}

// TestConnectionResponse is the response from testing a database connection.
type TestConnectionResponse struct {
	Ok            bool     `json:"ok"`
	LatencyMs     *float32 `json:"latency_ms,omitempty"`
	Error         *string  `json:"error,omitempty"`
	ServerVersion *string  `json:"server_version,omitempty"`
}

// TestConnection tests connectivity to an upstream database.
func (s *DatabasesService) TestConnection(ctx context.Context, projectID, databaseID string) (*TestConnectionResponse, error) {
	var resp TestConnectionResponse
	path := fmt.Sprintf("/v1/projects/%s/databases/%s/test-connection", projectID, databaseID)
	if err := s.client.post(ctx, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("test connection for database %s: %w", databaseID, err)
	}
	return &resp, nil
}

// Database represents an upstream database connection.
type Database struct {
	ID               string      `json:"id"`
	ProjectID        string      `json:"project_id"`
	Host             string      `json:"host"`
	Port             int         `json:"port"`
	Name             string      `json:"name"`
	Username         string      `json:"username"`
	SSLMode          string      `json:"ssl_mode,omitempty"`
	Role             string      `json:"role,omitempty"`
	PoolRegion       *string     `json:"pool_region,omitempty"`
	CacheConfig      CacheConfig `json:"cache_config"`
	PoolConfig       PoolConfig  `json:"pool_config"`
	ConnectionString *string     `json:"connection_string,omitempty"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
}

// CacheConfig configures query caching for a database.
type CacheConfig struct {
	Enabled    bool `json:"enabled"`
	TTLSeconds int  `json:"ttl_seconds"`
	MaxEntries int  `json:"max_entries"`
	SWRSeconds int  `json:"swr_seconds"`
}

// PoolConfig configures connection pooling for a database.
type PoolConfig struct {
	PoolSize    int    `json:"pool_size"`
	MinPoolSize int    `json:"min_pool_size"`
	PoolMode    string `json:"pool_mode"`
	MaxActive   *int   `json:"max_active,omitempty"`
}

// CreateDatabaseRequest is the request body for creating a database.
type CreateDatabaseRequest struct {
	Host        string       `json:"host"`
	Port        int          `json:"port"`
	Name        string       `json:"name"`
	Username    string       `json:"username"`
	Password    string       `json:"password"`
	SSLMode     string       `json:"ssl_mode,omitempty"`
	Role        string       `json:"role,omitempty"`
	PoolRegion  *string      `json:"pool_region,omitempty"`
	CacheConfig *CacheConfig `json:"cache_config,omitempty"`
	PoolConfig  *PoolConfig  `json:"pool_config,omitempty"`
}

// UpdateDatabaseRequest is the request body for updating a database.
type UpdateDatabaseRequest struct {
	Host        *string      `json:"host,omitempty"`
	Port        *int         `json:"port,omitempty"`
	Name        *string      `json:"name,omitempty"`
	Username    *string      `json:"username,omitempty"`
	Password    *string      `json:"password,omitempty"`
	SSLMode     *string      `json:"ssl_mode,omitempty"`
	Role        *string      `json:"role,omitempty"`
	PoolRegion  *string      `json:"pool_region,omitempty"`
	CacheConfig *CacheConfig `json:"cache_config,omitempty"`
	PoolConfig  *PoolConfig  `json:"pool_config,omitempty"`
}

// ListDatabasesResponse is the response from listing databases.
type ListDatabasesResponse struct {
	Databases     []Database `json:"databases"`
	NextPageToken string     `json:"next_page_token,omitempty"`
}
