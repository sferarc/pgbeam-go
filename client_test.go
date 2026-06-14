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

// newTestTransport creates a transport pointing at the given test server.
func newTestTransport(srv *httptest.Server, retry *RetryConfig) *transport {
	return newTransport(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   retry,
	})
}

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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 5, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond})

	var result map[string]string
	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, &result)
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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond})

	err := tr.do(context.Background(), http.MethodPost, "/v1/test", map[string]string{"name": "x"}, nil)
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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 5, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond})

	err := tr.do(context.Background(), http.MethodPost, "/v1/projects", map[string]string{"name": "test"}, nil)
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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 3, InitialDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond})

	err := tr.do(context.Background(), http.MethodGet, "/v1/projects/prj_1", nil, nil)
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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 10, InitialDelay: 1 * time.Second, MaxDelay: 5 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := tr.do(ctx, http.MethodGet, "/v1/test", nil, nil)
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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 0})

	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call (retries disabled), got %d", calls.Load())
	}
}

func TestRetry_DefaultConfig(t *testing.T) {
	tr := newTransport(&ClientOptions{APIKey: "test"})
	if tr.retry.MaxRetries != 5 {
		t.Fatalf("expected default MaxRetries=5, got %d", tr.retry.MaxRetries)
	}
	if tr.retry.InitialDelay != 500*time.Millisecond {
		t.Fatalf("expected default InitialDelay=500ms, got %v", tr.retry.InitialDelay)
	}
	if tr.retry.MaxDelay != 30*time.Second {
		t.Fatalf("expected default MaxDelay=30s, got %v", tr.retry.MaxDelay)
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

	tr := newTestTransport(srv, &RetryConfig{MaxRetries: 3, InitialDelay: 5 * time.Second, MaxDelay: 10 * time.Second})

	err := tr.do(context.Background(), http.MethodGet, "/v1/test", nil, nil)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	// Retry-After: 0 should override the 5s initial delay.
	if elapsed := time.Since(start); elapsed > 1*time.Second {
		t.Fatalf("expected fast retry with Retry-After: 0, took %v", elapsed)
	}
}

func TestEncodeQuery(t *testing.T) {
	type params struct {
		OrgID    string `json:"org_id"`
		PageSize *int   `json:"page_size,omitempty"`
	}

	ps := 20
	q := encodeQuery(params{OrgID: "org_123", PageSize: &ps})
	if q == "" {
		t.Fatal("expected non-empty query string")
	}
	if !contains(q, "org_id=org_123") {
		t.Fatalf("expected org_id in query, got: %s", q)
	}
	if !contains(q, "page_size=20") {
		t.Fatalf("expected page_size in query, got: %s", q)
	}
}

func TestEncodeQuery_NilPointer(t *testing.T) {
	type params struct {
		OrgID    string `json:"org_id"`
		PageSize *int   `json:"page_size,omitempty"`
	}

	q := encodeQuery(params{OrgID: "org_123"})
	if contains(q, "page_size") {
		t.Fatalf("expected no page_size in query, got: %s", q)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Service methods (httptest-backed): verify method, path, headers, body, and
// response decoding through the generated service layer.
// ---------------------------------------------------------------------------

func newTestClient(srv *httptest.Server) *Client {
	// Default retry config (MaxRetries > 0) so mutating requests carry an
	// idempotency key; these handlers all succeed on the first attempt, so
	// retries never actually fire.
	return NewClient(&ClientOptions{
		APIKey:  "pgb_test_key",
		BaseURL: srv.URL,
	})
}

func TestProjectsService_GetProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/projects/proj_abc" {
			t.Errorf("path = %s, want /v1/projects/proj_abc", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer pgb_test_key" {
			t.Errorf("Authorization = %q, want Bearer pgb_test_key", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"proj_abc","name":"My Project","created_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	proj, err := newTestClient(srv).Projects.GetProject(context.Background(), "proj_abc")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.Id != "proj_abc" {
		t.Errorf("Id = %q, want proj_abc", proj.Id)
	}
	if proj.Name != "My Project" {
		t.Errorf("Name = %q, want My Project", proj.Name)
	}
}

func TestProjectsService_CreateProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/projects" {
			t.Errorf("path = %s, want /v1/projects", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		// Mutating requests carry an idempotency key for safe retries.
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing Idempotency-Key on POST")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["name"] != "New Project" {
			t.Errorf("body.name = %v, want New Project", body["name"])
		}
		if body["org_id"] != "org_1" {
			t.Errorf("body.org_id = %v, want org_1", body["org_id"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"project":{"id":"proj_new","name":"New Project","created_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	resp, err := newTestClient(srv).Projects.CreateProject(context.Background(), CreateProjectRequest{
		Name:  "New Project",
		OrgId: "org_1",
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if resp.Project.Id != "proj_new" {
		t.Errorf("Project.Id = %q, want proj_new", resp.Project.Id)
	}
}

func TestProjectsService_ListProjects_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("org_id") != "org_1" {
			t.Errorf("org_id = %q, want org_1", q.Get("org_id"))
		}
		if q.Get("page_size") != "50" {
			t.Errorf("page_size = %q, want 50", q.Get("page_size"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projects":[],"total":7}`))
	}))
	defer srv.Close()

	ps := PageSize(50)
	resp, err := newTestClient(srv).Projects.ListProjects(context.Background(), &ListProjectsParams{
		OrgId:    "org_1",
		PageSize: &ps,
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if resp.Total != 7 {
		t.Errorf("Total = %d, want 7", resp.Total)
	}
}

func TestDatabasesService_ListDatabases_Path(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/proj_abc/databases" {
			t.Errorf("path = %s, want /v1/projects/proj_abc/databases", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"databases":[]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Databases.ListDatabases(context.Background(), "proj_abc", nil)
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
}

func TestProjectsService_DeleteProject_NoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestClient(srv).Projects.DeleteProject(context.Background(), "proj_abc"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
}

func TestProjectsService_GetProject_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"project not found"}}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Projects.GetProject(context.Background(), "proj_missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound = false, want true (err: %v)", err)
	}
}

func TestAPIError_Error_MessageExtraction(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"nested error.message", `{"error":{"message":"boom"}}`, "pgbeam: 400 Bad Request (400): boom"},
		{"flat message", `{"message":"flat boom"}`, "pgbeam: 400 Bad Request (400): flat boom"},
		{"raw body", `not json`, "pgbeam: 400 Bad Request (400): not json"},
		{"empty body", ``, "pgbeam: 400 Bad Request (400)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &APIError{StatusCode: 400, Status: "400 Bad Request", Body: tt.body}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
