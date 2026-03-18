// Package pgbeam provides a Go client for the PgBeam Control Plane API.
//
// Usage:
//
//	client := pgbeam.NewClient(&pgbeam.ClientOptions{
//	    APIKey:  "pgb_your_api_key",
//	    BaseURL: "https://api.pgbeam.com", // optional
//	})
//
//	project, err := client.Projects.Get(ctx, "proj_abc123")
//	databases, err := client.Databases.List(ctx, "proj_abc123", nil)
package pgbeam

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	mrand "math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.pgbeam.com"

// RetryConfig configures automatic retry behaviour.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts after the initial
	// request. Default: 5. Set to 0 to disable retries.
	MaxRetries int
	// InitialDelay is the initial backoff delay. Default: 500ms.
	InitialDelay time.Duration
	// MaxDelay is the maximum backoff delay. Default: 30s.
	MaxDelay time.Duration
}

var defaultRetry = RetryConfig{
	MaxRetries:   5,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     30 * time.Second,
}

// ClientOptions configures the PgBeam client.
type ClientOptions struct {
	// APIKey is the PgBeam API key (required). Prefix: pgb_.
	APIKey string
	// BaseURL overrides the default API base URL. Defaults to https://api.pgbeam.com.
	BaseURL string
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
	// Retry configures automatic retry with exponential backoff. A nil
	// value uses defaults (5 retries, 500ms initial delay). Set
	// &RetryConfig{MaxRetries: 0} to disable retries.
	Retry *RetryConfig
}

// Client is the PgBeam API client. Access API operations through the
// tag-based service fields (Projects, Databases, etc.), mirroring
// the TypeScript SDK's api.projects.*, api.databases.* pattern.
type Client struct {
	// Projects provides operations for managing PgBeam projects.
	Projects *ProjectsService
	// Databases provides operations for managing upstream databases.
	Databases *DatabasesService
	// Replicas provides operations for managing read replicas.
	Replicas *ReplicasService
	// Domains provides operations for managing custom domains.
	Domains *DomainsService
	// CacheRules provides operations for managing per-query cache rules.
	CacheRules *CacheRulesService
	// Analytics provides operations for organization plans and spend limits.
	Analytics *AnalyticsService

	baseURL    string
	apiKey     string
	httpClient *http.Client
	retry      RetryConfig
}

// NewClient creates a new PgBeam API client.
func NewClient(opts *ClientOptions) *Client {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	retry := defaultRetry
	if opts.Retry != nil {
		retry = *opts.Retry
	}

	c := &Client{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		httpClient: httpClient,
		retry:      retry,
	}

	c.Projects = &ProjectsService{client: c}
	c.Databases = &DatabasesService{client: c}
	c.Replicas = &ReplicasService{client: c}
	c.Domains = &DomainsService{client: c}
	c.CacheRules = &CacheRulesService{client: c}
	c.Analytics = &AnalyticsService{client: c}

	return c
}

// APIError represents an error response from the PgBeam API.
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	// Try to extract a structured error message like the TS SDK does.
	msg := extractMessage(e.Body)
	if msg != "" {
		return fmt.Sprintf("pgbeam: %s (%d): %s", e.Status, e.StatusCode, msg)
	}
	return fmt.Sprintf("pgbeam: %s (%d)", e.Status, e.StatusCode)
}

// IsNotFound returns true if the error is a 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}

// extractMessage attempts to extract a human-readable message from a JSON error
// body, following the same strategy as the TS SDK: body.error.message → body.message → raw.
func extractMessage(body string) string {
	if body == "" {
		return ""
	}

	// Try { "error": { "message": "..." } }
	var nested struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &nested) == nil && nested.Error.Message != "" {
		return nested.Error.Message
	}

	// Try { "message": "..." }
	var flat struct {
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(body), &flat) == nil && flat.Message != "" {
		return flat.Message
	}

	return body
}

var retryableStatusCodes = map[int]bool{
	408: true, 429: true, 502: true, 503: true, 504: true,
}

func isMutatingMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	u := c.baseURL + path

	var bodyData []byte
	if body != nil {
		var err error
		bodyData, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("pgbeam: marshal request body: %w", err)
		}
	}

	// Generate idempotency key once, reused across all attempts.
	var idempotencyKey string
	if c.retry.MaxRetries > 0 && isMutatingMethod(method) {
		idempotencyKey = generateUUID()
	}

	var lastErr error
	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		var bodyReader io.Reader
		if bodyData != nil {
			bodyReader = bytes.NewReader(bodyData)
		}

		req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
		if err != nil {
			return fmt.Errorf("pgbeam: create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "pgbeam-go/1.0")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error — retry if attempts remain.
			lastErr = fmt.Errorf("pgbeam: execute request: %w", err)
			if attempt == c.retry.MaxRetries {
				return lastErr
			}
			sleepWithContext(ctx, backoff(attempt, c.retry.InitialDelay, c.retry.MaxDelay))
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("pgbeam: read response body: %w", readErr)
		}

		if resp.StatusCode < 400 {
			if result != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, result); err != nil {
					return fmt.Errorf("pgbeam: unmarshal response: %w", err)
				}
			}
			return nil
		}

		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(respBody),
		}

		// Non-retryable or last attempt — return error.
		if !retryableStatusCodes[resp.StatusCode] || attempt == c.retry.MaxRetries {
			return apiErr
		}

		// Retryable — wait and retry.
		lastErr = apiErr
		delay, hasRetryAfter := parseRetryAfter(resp)
		if !hasRetryAfter {
			delay = backoff(attempt, c.retry.InitialDelay, c.retry.MaxDelay)
		}
		sleepWithContext(ctx, delay)
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("pgbeam: exhausted all retry attempts")
}

// backoff computes exponential backoff with jitter.
func backoff(attempt int, initialDelay, maxDelay time.Duration) time.Duration {
	delay := float64(initialDelay) * math.Pow(2, float64(attempt))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}
	jitter := 0.5 + mrand.Float64() // [0.5, 1.5)
	return time.Duration(delay * jitter)
}

// parseRetryAfter parses the Retry-After header as a duration. The bool
// indicates whether a valid header was present.
func parseRetryAfter(resp *http.Response) (time.Duration, bool) {
	header := resp.Header.Get("Retry-After")
	if header == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(header); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}

// generateUUID produces a UUID v4.
func generateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// sleepWithContext sleeps for the given duration or until the context is cancelled.
func sleepWithContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	return c.do(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body, result any) error {
	return c.do(ctx, http.MethodPost, path, body, result)
}

func (c *Client) patch(ctx context.Context, path string, body, result any) error {
	return c.do(ctx, http.MethodPatch, path, body, result)
}

func (c *Client) put(ctx context.Context, path string, body, result any) error {
	return c.do(ctx, http.MethodPut, path, body, result)
}

func (c *Client) del(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// addQueryParams adds query parameters to a path.
func addQueryParams(path string, params map[string]string) string {
	if len(params) == 0 {
		return path
	}
	v := url.Values{}
	for k, val := range params {
		if val != "" {
			v.Set(k, val)
		}
	}
	if encoded := v.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}
