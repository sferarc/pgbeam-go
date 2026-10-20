// Package pgbeam provides a Go client for the PgBeam Control Plane API.
//
// Usage:
//
//	client := pgbeam.NewClient(&pgbeam.ClientOptions{
//	    APIKey:  "pgb_your_api_key",
//	    BaseURL: "https://api.pgbeam.com", // optional
//	})
//
//	project, err := client.Projects.GetProject(ctx, "prj_abc123")
//	databases, err := client.Databases.ListDatabases(ctx, "prj_abc123", nil)
package pgbeam

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
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
	// TotalBudget caps the whole call, measured from the first attempt and
	// including time spent in requests. A retry whose delay would land past
	// it is not made and the last error is returned. Default: 120s. Zero or
	// negative disables the budget.
	TotalBudget time.Duration
}

var defaultRetry = RetryConfig{
	MaxRetries:   5,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	TotalBudget:  120 * time.Second,
}

// DefaultTimeout bounds a single attempt when ClientOptions.Timeout is zero.
// It is long enough for the slowest legitimate call (an audit log CSV export).
const DefaultTimeout = 30 * time.Second

// maxErrorBodyBytes caps how much of an error response is read into APIError.
const maxErrorBodyBytes = 1 << 20

// ClientOptions configures the PgBeam client.
type ClientOptions struct {
	// APIKey is the PgBeam API key (required). Prefix: pgb_.
	APIKey string
	// BaseURL overrides the default API base URL. Defaults to https://api.pgbeam.com.
	BaseURL string
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
	// Retry configures automatic retry with exponential backoff. A nil
	// value uses defaults (5 retries, 500ms initial delay, 120s total
	// budget). A non-nil value is used as given, so fields left zero keep
	// their zero meaning. Set &RetryConfig{MaxRetries: 0} to disable retries.
	Retry *RetryConfig
	// Timeout bounds each attempt, including reading the response body.
	// Zero means DefaultTimeout (30s); a negative value disables it. It is
	// applied through the request context, so it holds for a caller-supplied
	// HTTPClient too.
	Timeout time.Duration
}

// transport handles HTTP communication with the PgBeam API. It is not
// exported — users interact with the generated Client type.
type transport struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	retry      RetryConfig
	timeout    time.Duration
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
	retry.MaxRetries = max(retry.MaxRetries, 0)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &transport{
		baseURL:    baseURL,
		apiKey:     opts.APIKey,
		httpClient: httpClient,
		retry:      retry,
		timeout:    timeout,
	}
}

var retryableStatusCodes = map[int]bool{
	408: true, 429: true, 502: true, 503: true, 504: true,
}

func isMutatingMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPatch
}

type refusesIdempotencyKeyCtx struct{}

// withoutIdempotencyKey marks a call to an operation that answers 400 to an
// Idempotency-Key, so the transport sends none even when retries are on.
func withoutIdempotencyKey(ctx context.Context) context.Context {
	return context.WithValue(ctx, refusesIdempotencyKeyCtx{}, true)
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
	if t.retry.MaxRetries > 0 && isMutatingMethod(method) && ctx.Value(refusesIdempotencyKeyCtx{}) == nil {
		idempotencyKey = generateUUID()
	}

	// Fail fast on a malformed URL rather than retrying it as a network error.
	if _, err := http.NewRequestWithContext(ctx, method, u, nil); err != nil {
		return fmt.Errorf("pgbeam: create request: %w", err)
	}

	startedAt := time.Now()
	var lastErr error
	for attempt := 0; attempt <= t.retry.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		resp, err := t.attempt(ctx, method, u, bodyData, idempotencyKey)
		if err != nil {
			// A cancelled caller context is final; anything else is a
			// network failure or a per-attempt timeout and is retried.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			lastErr = err
			if attempt == t.retry.MaxRetries {
				return lastErr
			}
			delay := backoff(attempt, t.retry.InitialDelay, t.retry.MaxDelay)
			if t.outOfBudget(startedAt, delay) {
				return lastErr
			}
			sleepWithContext(ctx, delay)
			continue
		}

		if resp.StatusCode < 400 {
			if result != nil && len(resp.Body) > 0 {
				if err := json.Unmarshal(resp.Body, result); err != nil {
					return fmt.Errorf("pgbeam: unmarshal response: %w", err)
				}
			}
			return nil
		}

		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(resp.Body),
		}
		apiErr.parseProblem()

		// Non-retryable or last attempt — return error.
		if !retryableStatusCodes[resp.StatusCode] || attempt == t.retry.MaxRetries {
			return apiErr
		}

		// Retryable — wait and retry, unless the wait outlives the budget.
		lastErr = apiErr
		delay, hasRetryAfter := parseRetryAfter(resp.Header)
		if !hasRetryAfter {
			delay = backoff(attempt, t.retry.InitialDelay, t.retry.MaxDelay)
		}
		if t.outOfBudget(startedAt, delay) {
			return apiErr
		}
		sleepWithContext(ctx, delay)
	}

	return lastErr
}

// attemptResponse is a response whose body has been read and closed.
type attemptResponse struct {
	StatusCode int
	Status     string
	Header     http.Header
	Body       []byte
}

// attempt sends one request under the per-attempt timeout and reads its body.
// An error response body is read up to maxErrorBodyBytes.
func (t *transport) attempt(ctx context.Context, method, u string, bodyData []byte, idempotencyKey string) (*attemptResponse, error) {
	if t.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.timeout)
		defer cancel()
	}

	var bodyReader io.Reader
	if bodyData != nil {
		bodyReader = bytes.NewReader(bodyData)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("pgbeam: create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	if bodyData != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pgbeam: execute request: %w", err)
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.StatusCode >= 400 {
		reader = io.LimitReader(resp.Body, maxErrorBodyBytes)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("pgbeam: read response body: %w", err)
	}
	return &attemptResponse{StatusCode: resp.StatusCode, Status: resp.Status, Header: resp.Header, Body: body}, nil
}

// outOfBudget reports whether waiting delay and trying again would land past
// the total budget.
func (t *transport) outOfBudget(startedAt time.Time, delay time.Duration) bool {
	if t.retry.TotalBudget <= 0 {
		return false
	}
	return time.Since(startedAt)+delay >= t.retry.TotalBudget
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
// Only fields with a form tag are encoded.
func doQuery[T any](t *transport, ctx context.Context, path string, params any) (*T, error) {
	if params != nil {
		if q := encodeQuery(params); q != "" {
			path = path + "?" + q
		}
	}
	return doJSON[T](t, ctx, "GET", path, nil)
}

// encodeQuery encodes a struct's form-tagged fields as URL query parameters.
// Non-pointer zero values are skipped; a non-nil pointer is always sent.
// Fields without a form tag are header parameters and never reach the URL.
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
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("form"), ",")
		if name == "" || name == "-" {
			continue
		}

		fv := v.Field(i)
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				continue
			}
			fv = fv.Elem()
		} else if fv.IsZero() {
			continue
		}

		if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
			for j := 0; j < fv.Len(); j++ {
				vals.Add(name, formatQueryValue(fv.Index(j)))
			}
			continue
		}
		vals.Add(name, formatQueryValue(fv))
	}
	return vals.Encode()
}

func formatQueryValue(v reflect.Value) string {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	switch x := v.Interface().(type) {
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return x.String()
	}
	switch v.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	}
	return fmt.Sprint(v.Interface())
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
func parseRetryAfter(h http.Header) (time.Duration, bool) {
	header := h.Get("Retry-After")
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
