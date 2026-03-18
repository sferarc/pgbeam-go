package pgbeam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_503_ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "prj_1"})
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 5, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond},
	})

	var result map[string]string
	err := client.do(context.Background(), http.MethodGet, "/v1/test", nil, &result)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if result["id"] != "prj_1" {
		t.Fatalf("unexpected result: %v", result)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls (2 failures + 1 success), got %d", calls.Load())
	}
}

func TestRetry_400_NoRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad input"}}`))
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond},
	})

	err := client.do(context.Background(), http.MethodPost, "/v1/test", map[string]string{"name": "x"}, nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Fatalf("expected status 400, got %d", apiErr.StatusCode)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected exactly 1 call (no retry for 400), got %d", calls.Load())
	}
}

func TestRetry_IdempotencyKey_POST(t *testing.T) {
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(keys) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "prj_1"})
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 5, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond},
	})

	err := client.do(context.Background(), http.MethodPost, "/v1/projects", map[string]string{"name": "test"}, nil)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	if len(keys) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(keys))
	}

	// All attempts should have the same non-empty key.
	for i, k := range keys {
		if k == "" {
			t.Fatalf("attempt %d: expected Idempotency-Key header, got empty", i)
		}
		if k != keys[0] {
			t.Fatalf("attempt %d: key %q differs from first attempt key %q", i, k, keys[0])
		}
	}
}

func TestRetry_NoIdempotencyKey_GET(t *testing.T) {
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if len(keys) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "prj_1"})
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond},
	})

	err := client.do(context.Background(), http.MethodGet, "/v1/projects/prj_1", nil, nil)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	for i, k := range keys {
		if k != "" {
			t.Fatalf("attempt %d: expected no Idempotency-Key for GET, got %q", i, k)
		}
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 10, InitialDelay: 1 * time.Second, MaxDelay: 5 * time.Second},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.do(ctx, http.MethodGet, "/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRetry_Disabled(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 0},
	})

	err := client.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call (retries disabled), got %d", calls.Load())
	}
}

func TestRetry_DefaultConfig(t *testing.T) {
	client := NewClient(&ClientOptions{APIKey: "test"})
	if client.retry.MaxRetries != 5 {
		t.Fatalf("expected default MaxRetries=5, got %d", client.retry.MaxRetries)
	}
	if client.retry.InitialDelay != 500*time.Millisecond {
		t.Fatalf("expected default InitialDelay=500ms, got %v", client.retry.InitialDelay)
	}
	if client.retry.MaxDelay != 30*time.Second {
		t.Fatalf("expected default MaxDelay=30s, got %v", client.retry.MaxDelay)
	}
}

func TestRetry_RetryAfterHeader(t *testing.T) {
	start := time.Now()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	client := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 3, InitialDelay: 5 * time.Second, MaxDelay: 10 * time.Second},
	})

	err := client.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	// Retry-After: 0 should override the 5s initial delay.
	if elapsed := time.Since(start); elapsed > 1*time.Second {
		t.Fatalf("expected fast retry with Retry-After: 0, took %v", elapsed)
	}
}
