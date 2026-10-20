package pgbeam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimeout_StalledAttemptIsRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			<-r.Context().Done()
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	tr := newTransport(&ClientOptions{
		APIKey:     "test",
		BaseURL:    srv.URL,
		HTTPClient: &http.Client{},
		Timeout:    100 * time.Millisecond,
		Retry:      &RetryConfig{MaxRetries: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})

	start := time.Now()
	if err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil); err != nil {
		t.Fatalf("expected success after the stalled attempt, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stalled attempt was not cut off by Timeout, took %v", elapsed)
	}
}

func TestTimeout_LastStalledAttemptReturnsDeadlineError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	tr := newTransport(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Timeout: 50 * time.Millisecond,
		Retry:   &RetryConfig{MaxRetries: 0},
	})

	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected a deadline error, got: %v", err)
	}
}

func TestTimeout_Defaults(t *testing.T) {
	if tr := newTransport(&ClientOptions{APIKey: "test"}); tr.timeout != DefaultTimeout {
		t.Fatalf("expected default timeout %v, got %v", DefaultTimeout, tr.timeout)
	}
	if tr := newTransport(&ClientOptions{APIKey: "test", Timeout: -1}); tr.timeout > 0 {
		t.Fatalf("expected a negative Timeout to disable the bound, got %v", tr.timeout)
	}
	if tr := newTransport(&ClientOptions{APIKey: "test"}); tr.retry.TotalBudget != 120*time.Second {
		t.Fatalf("expected default TotalBudget=120s, got %v", tr.retry.TotalBudget)
	}
}

func TestBudget_RetryAfterPastBudgetReturnsImmediately(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	tr := newTestTransport(srv, nil)

	start := time.Now()
	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected the 429 APIError, got: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", calls.Load())
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected no sleep, took %v", elapsed)
	}
}

func TestBudget_ExhaustionStopsRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	tr := newTestTransport(srv, &RetryConfig{
		MaxRetries:   1000,
		InitialDelay: 40 * time.Millisecond,
		MaxDelay:     40 * time.Millisecond,
		TotalBudget:  300 * time.Millisecond,
	})

	start := time.Now()
	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected the 503 APIError, got: %v", err)
	}
	if n := calls.Load(); n < 2 || n > 50 {
		t.Fatalf("expected a handful of attempts within the budget, got %d", n)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("budget did not stop retries, took %v", elapsed)
	}
}

func TestRetry_NegativeMaxRetriesMakesOneAttempt(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: -1})

	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an APIError, got: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected exactly 1 call, got %d", calls.Load())
	}
}

func TestRetry_BodyReadErrorIsRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "100")
			w.Write([]byte(`{"id":`))
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond})

	var out struct{ ID string }
	if err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, &out); err != nil {
		t.Fatalf("expected success after a truncated body, got: %v", err)
	}
	if calls.Load() != 2 || out.ID != "ok" {
		t.Fatalf("expected 2 calls and id=ok, got %d calls and %q", calls.Load(), out.ID)
	}
}

func TestErrorBodyIsCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(strings.Repeat("x", 2*maxErrorBodyBytes)))
	}))
	defer srv.Close()

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 0})

	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an APIError, got: %v", err)
	}
	if len(apiErr.Body) != maxErrorBodyBytes {
		t.Fatalf("expected the body capped at %d bytes, got %d", maxErrorBodyBytes, len(apiErr.Body))
	}
}

func TestUserAgentMatchesPackageVersion(t *testing.T) {
	raw, err := os.ReadFile("package.json")
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("package.json is dropped from the published mirror")
	}
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ Version string }
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != sdkVersion {
		t.Fatalf("sdkVersion is %q but package.json says %q; run `pnpm run script sync-go-version`", sdkVersion, manifest.Version)
	}

	var got atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("User-Agent"))
	}))
	defer srv.Close()

	if err := newTestTransport(srv, nil).do(context.Background(), http.MethodGet, "/v1/test", nil, nil); err != nil {
		t.Fatal(err)
	}
	if want := "pgbeam-go/" + manifest.Version; got.Load() != want {
		t.Fatalf("expected User-Agent %q, got %q", want, got.Load())
	}
}

func TestMalformedURLIsNotRetried(t *testing.T) {
	tr := newTransport(&ClientOptions{APIKey: "test", BaseURL: "http://bad host", Retry: &RetryConfig{MaxRetries: 3, InitialDelay: time.Second}})

	start := time.Now()
	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "create request") {
		t.Fatalf("expected a create request error, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("malformed URL was retried, took %v", elapsed)
	}
}
