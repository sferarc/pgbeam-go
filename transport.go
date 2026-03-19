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
	"reflect"
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

// transport handles HTTP communication with the PgBeam API. It is not
// exported — users interact with the generated Client type.
type transport struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	retry      RetryConfig
}

func newTransport(opts *ClientOptions) *transport {
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

	return &transport{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		httpClient: httpClient,
		retry:      retry,
	}
}

var retryableStatusCodes = map[int]bool{
	408: true, 429: true, 502: true, 503: true, 504: true,
}

func isMutatingMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch
}

func (t *transport) do(ctx context.Context, method, path string, body any, result any) error {
	u := t.baseURL + path

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
	if t.retry.MaxRetries > 0 && isMutatingMethod(method) {
		idempotencyKey = generateUUID()
	}

	var lastErr error
	for attempt := 0; attempt <= t.retry.MaxRetries; attempt++ {
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

		req.Header.Set("Authorization", "Bearer "+t.apiKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "pgbeam-go/1.0")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := t.httpClient.Do(req)
		if err != nil {
			// Network error — retry if attempts remain.
			lastErr = fmt.Errorf("pgbeam: execute request: %w", err)
			if attempt == t.retry.MaxRetries {
				return lastErr
			}
			sleepWithContext(ctx, backoff(attempt, t.retry.InitialDelay, t.retry.MaxDelay))
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
		if !retryableStatusCodes[resp.StatusCode] || attempt == t.retry.MaxRetries {
			return apiErr
		}

		// Retryable — wait and retry.
		lastErr = apiErr
		delay, hasRetryAfter := parseRetryAfter(resp)
		if !hasRetryAfter {
			delay = backoff(attempt, t.retry.InitialDelay, t.retry.MaxDelay)
		}
		sleepWithContext(ctx, delay)
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("pgbeam: exhausted all retry attempts")
}

// doJSON executes a request and unmarshals the JSON response into *T.
func doJSON[T any](t *transport, ctx context.Context, method, path string, body any) (*T, error) {
	var result T
	if err := t.do(ctx, method, path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// doVoid executes a request that returns no body (e.g. 204 No Content).
func doVoid(t *transport, ctx context.Context, method, path string, body any) error {
	return t.do(ctx, method, path, body, nil)
}

// doQuery executes a GET request with query parameters encoded from a struct.
// Struct fields are encoded using their json tags.
func doQuery[T any](t *transport, ctx context.Context, path string, params any) (*T, error) {
	if params != nil {
		if q := encodeQuery(params); q != "" {
			path = path + "?" + q
		}
	}
	return doJSON[T](t, ctx, "GET", path, nil)
}

// encodeQuery encodes a struct's non-zero fields as URL query parameters.
// It uses json struct tags for parameter names and skips zero-value fields.
func encodeQuery(params any) string {
	v := reflect.ValueOf(params)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}

	vals := url.Values{}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		// Use json tag for param name.
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			tag = field.Tag.Get("form")
		}
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]

		// Handle pointer fields.
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				continue
			}
			fv = fv.Elem()
		}

		// Skip zero values.
		if fv.IsZero() {
			continue
		}

		vals.Set(name, fmt.Sprintf("%v", fv.Interface()))
	}
	return vals.Encode()
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
