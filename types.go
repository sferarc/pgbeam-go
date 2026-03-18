package pgbeam

import "time"

// Project represents a PgBeam project.
type Project struct {
	ID                string   `json:"id"`
	OrgID             string   `json:"org_id"`
	Name              string   `json:"name"`
	Description       *string  `json:"description,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Cloud             string   `json:"cloud,omitempty"`
	ProxyHost         string   `json:"proxy_host,omitempty"`
	QueriesPerSecond  int32    `json:"queries_per_second,omitempty"`
	BurstSize         int32    `json:"burst_size,omitempty"`
	MaxConnections    int32    `json:"max_connections,omitempty"`
	DatabaseCount     int      `json:"database_count,omitempty"`
	ActiveConnections int      `json:"active_connections,omitempty"`
	Status            string   `json:"status"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// CreateProjectRequest is the request body for creating a project.
type CreateProjectRequest struct {
	Name        string                `json:"name"`
	OrgID       string                `json:"org_id"`
	Description *string               `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Cloud       string                `json:"cloud,omitempty"`
	Database    CreateDatabaseRequest `json:"database"`
}

// CreateProjectResponse is the response from creating a project.
type CreateProjectResponse struct {
	Project  Project   `json:"project"`
	Database *Database `json:"database,omitempty"`
}

// UpdateProjectRequest is the request body for updating a project.
type UpdateProjectRequest struct {
	Name             *string   `json:"name,omitempty"`
	Description      *string   `json:"description,omitempty"`
	Tags             *[]string `json:"tags,omitempty"`
	QueriesPerSecond *int32    `json:"queries_per_second,omitempty"`
	BurstSize        *int32    `json:"burst_size,omitempty"`
	MaxConnections   *int32    `json:"max_connections,omitempty"`
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

// CustomDomain represents a custom domain attached to a project.
type CustomDomain struct {
	ID                   string           `json:"id"`
	ProjectID            string           `json:"project_id"`
	Domain               string           `json:"domain"`
	Verified             bool             `json:"verified"`
	VerifiedAt           *time.Time       `json:"verified_at,omitempty"`
	TLSCertExpiry        *time.Time       `json:"tls_cert_expiry,omitempty"`
	DNSVerificationToken string           `json:"dns_verification_token,omitempty"`
	DNSInstructions      *DNSInstructions `json:"dns_instructions,omitempty"`
	CreatedAt            string           `json:"created_at"`
	UpdatedAt            string           `json:"updated_at"`
}

// DNSInstructions contains the DNS records needed for domain verification.
type DNSInstructions struct {
	CNAMEHost       string `json:"cname_host,omitempty"`
	CNAMETarget     string `json:"cname_target,omitempty"`
	TXTHost         string `json:"txt_host,omitempty"`
	TXTValue        string `json:"txt_value,omitempty"`
	ACMECNAMEHost   string `json:"acme_cname_host,omitempty"`
	ACMECNAMETarget string `json:"acme_cname_target,omitempty"`
}

// CreateCustomDomainRequest is the request body for creating a custom domain.
type CreateCustomDomainRequest struct {
	Domain string `json:"domain"`
}

// ListCustomDomainsResponse is the response from listing custom domains.
type ListCustomDomainsResponse struct {
	Domains       []CustomDomain `json:"domains"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

// VerifyCustomDomainResponse is the response from verifying a custom domain.
type VerifyCustomDomainResponse struct {
	Verified bool `json:"verified"`
}

// CacheRule represents a per-query cache rule.
type CacheRule struct {
	QueryHash        string  `json:"query_hash"`
	NormalizedSQL    string  `json:"normalized_sql,omitempty"`
	QueryType        string  `json:"query_type,omitempty"`
	CacheEnabled     bool    `json:"cache_enabled"`
	CacheTTLSeconds  *int    `json:"cache_ttl_seconds,omitempty"`
	CacheSWRSeconds  *int    `json:"cache_swr_seconds,omitempty"`
	CallCount        int64   `json:"call_count,omitempty"`
	AvgLatencyMs     float64 `json:"avg_latency_ms,omitempty"`
	P95LatencyMs     float64 `json:"p95_latency_ms,omitempty"`
	AvgResponseBytes int64   `json:"avg_response_bytes,omitempty"`
	StabilityRate    float64 `json:"stability_rate,omitempty"`
	Recommendation   string  `json:"recommendation,omitempty"`
	FirstSeenAt      string  `json:"first_seen_at,omitempty"`
	LastSeenAt       string  `json:"last_seen_at,omitempty"`
}

// UpdateCacheRuleRequest is the request body for updating a cache rule.
type UpdateCacheRuleRequest struct {
	CacheEnabled    bool `json:"cache_enabled"`
	CacheTTLSeconds *int `json:"cache_ttl_seconds,omitempty"`
	CacheSWRSeconds *int `json:"cache_swr_seconds,omitempty"`
}

// UpdateCacheRuleResponse is the response from updating a cache rule.
type UpdateCacheRuleResponse struct {
	Entry CacheRule `json:"entry"`
}

// ListCacheRulesResponse is the response from listing cache rules.
type ListCacheRulesResponse struct {
	Entries       []CacheRule `json:"entries"`
	NextPageToken string      `json:"next_page_token,omitempty"`
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
