package pgbeam

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetProject_EscapesPathParam(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"a/b?c"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).Projects.GetProject(context.Background(), "a/b?c"); err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if want := "/v1/projects/a%2Fb%3Fc"; gotURI != want {
		t.Fatalf("request URI = %q, want %q", gotURI, want)
	}
}

func TestUpdateCacheRule_EscapesEveryPathParam(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Projects.UpdateCacheRule(context.Background(), "prj_1", "db#1", "50%/x", UpdateCacheRuleRequest{})
	if err != nil {
		t.Fatalf("UpdateCacheRule: %v", err)
	}
	if want := "/v1/projects/prj_1/databases/db%231/cache-rules/50%25%2Fx"; gotURI != want {
		t.Fatalf("request URI = %q, want %q", gotURI, want)
	}
}

func TestExportAuditLogs_SendsFiltersAndReturnsCSV(t *testing.T) {
	csv := []byte("timestamp,event,decision\n2026-10-14T00:00:00Z,blocked,block\n\xff\xfe raw bytes\n")
	var gotDecision, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/projects/prj_1/audit-logs/export" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		gotDecision = r.URL.Query().Get("decision")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write(csv)
	}))
	defer srv.Close()

	decision := AuditDecisionBlock
	got, err := newTestClient(srv).Agents.ExportAuditLogs(context.Background(), "prj_1", &ExportAuditLogsParams{Decision: &decision})
	if err != nil {
		t.Fatalf("ExportAuditLogs: %v", err)
	}
	if gotDecision != "block" {
		t.Errorf("decision query = %q, want block", gotDecision)
	}
	if gotAccept != "text/csv" {
		t.Errorf("Accept = %q, want text/csv", gotAccept)
	}
	if !bytes.Equal(got, csv) {
		t.Errorf("body = %q, want %q", got, csv)
	}
}

func TestExportAuditLogs_NilParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/csv")
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Agents.ExportAuditLogs(context.Background(), "prj_1", nil)
	if err != nil {
		t.Fatalf("ExportAuditLogs: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
	if len(got) != 0 {
		t.Errorf("body = %q, want empty", got)
	}
}

func TestExportAuditLogs_ErrorIsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NOT_FOUND","detail":"project not found"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Agents.ExportAuditLogs(context.Background(), "prj_missing", nil)
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false, err = %v", err)
	}
}

func TestExportAuditLogs_RetriesThroughTransport(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("a,b\n"))
	}))
	defer srv.Close()

	c := NewClient(&ClientOptions{
		APIKey:  "test",
		BaseURL: srv.URL,
		Retry:   &RetryConfig{MaxRetries: 2, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	got, err := c.Agents.ExportAuditLogs(context.Background(), "prj_1", nil)
	if err != nil {
		t.Fatalf("ExportAuditLogs: %v", err)
	}
	if string(got) != "a,b\n" || calls.Load() != 2 {
		t.Fatalf("body = %q after %d calls, want \"a,b\\n\" after 2", got, calls.Load())
	}
}

func TestDeleteSchemaAnnotation_SendsQuery(t *testing.T) {
	var gotMethod, gotTable string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotTable = r.URL.Query().Get("table_name")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	err := newTestClient(srv).Schemaannotations.DeleteSchemaAnnotation(context.Background(), "prj_1", &DeleteSchemaAnnotationParams{TableName: "users"})
	if err != nil {
		t.Fatalf("DeleteSchemaAnnotation: %v", err)
	}
	if gotMethod != http.MethodDelete || gotTable != "users" {
		t.Fatalf("got %s table_name=%q, want DELETE table_name=users", gotMethod, gotTable)
	}
}

func TestTestWebhookEndpoint_DecodesAcceptedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"delivery_id":"dlv_1","status":"pending"}`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Webhooks.TestWebhookEndpoint(context.Background(), "prj_1", "whk_1")
	if err != nil {
		t.Fatalf("TestWebhookEndpoint: %v", err)
	}
	if got.DeliveryId != "dlv_1" {
		t.Fatalf("DeliveryId = %q, want dlv_1", got.DeliveryId)
	}
}
