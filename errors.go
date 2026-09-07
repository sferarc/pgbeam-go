package pgbeam

import (
	"encoding/json"
	"errors"
	"fmt"
)

// APIError represents an error response from the PgBeam API.
//
// The API answers every error with an RFC 9457 problem document
// (application/problem+json), so the fields of that document are parsed out
// here rather than left in Body for the caller to unmarshal.
//
// Branch on Code, not on the message. Code is the stable identity of the
// condition and separates cases that share a status: a 403 is either FORBIDDEN
// (ask an admin for a role) or PLAN_LIMIT_REACHED (change plan). The detail
// string is prose and is free to change between releases.
type APIError struct {
	StatusCode int
	Status     string
	Body       string

	// Code is the machine-readable identity of the condition, e.g. "NOT_FOUND".
	Code string
	// Type is the same identity as a URI, resolving to the published catalog.
	Type string
	// Title is the short human-readable summary of the condition.
	Title string
	// Detail explains this particular occurrence.
	Detail string
	// Instance is the path of the request that produced the error.
	Instance string
	// RequestID correlates this response with the X-Request-Id header.
	RequestID string
	// Errors carries field-level detail when the request failed validation.
	Errors []FieldError
}

func (e *APIError) Error() string {
	if msg := extractMessage(e.Body); msg != "" {
		if e.Code != "" {
			return fmt.Sprintf("pgbeam: %s (%d) [%s]: %s", e.Status, e.StatusCode, e.Code, msg)
		}
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

// HasCode reports whether err is an APIError carrying the given error code.
// This is the check to branch on, because two conditions can share a status.
func HasCode(err error, code string) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == code
	}
	return false
}

// problemDocument is the wire shape of an RFC 9457 problem detail.
type problemDocument struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id"`
	Errors    []FieldError `json:"errors"`
}

// parseProblem fills the problem-document fields on e from its body. A body
// that is not one leaves them zero, so a caller can still read StatusCode.
func (e *APIError) parseProblem() {
	if e.Body == "" {
		return
	}
	var p problemDocument
	if json.Unmarshal([]byte(e.Body), &p) != nil {
		return
	}
	e.Type = p.Type
	e.Title = p.Title
	e.Detail = p.Detail
	e.Instance = p.Instance
	e.Code = p.Code
	e.RequestID = p.RequestID
	e.Errors = p.Errors
}

// extractMessage pulls a human-readable message out of a JSON error body.
//
// The control plane's problem document carries it in "detail". The two older
// shapes are still read because not everything a client talks to is the control
// plane: the edge MCP endpoint answers {"error":{"code","message"}}, and Echo's
// own default is {"message"}. A body in none of those shapes yields nothing
// rather than the raw JSON, which only produced an unreadable blob inside an
// error string.
func extractMessage(body string) string {
	if body == "" {
		return ""
	}

	var problem struct {
		Detail string `json:"detail"`
		Title  string `json:"title"`
		Code   string `json:"code"`
	}
	if json.Unmarshal([]byte(body), &problem) == nil {
		if problem.Detail != "" {
			return problem.Detail
		}
		// A problem document with no detail still has a title, which beats
		// falling back to the status line alone.
		if problem.Title != "" && problem.Code != "" {
			return problem.Title
		}
	}

	var nested struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &nested) == nil && nested.Error.Message != "" {
		return nested.Error.Message
	}

	var flat struct {
		Message string `json:"message"`
	}
	if json.Unmarshal([]byte(body), &flat) == nil && flat.Message != "" {
		return flat.Message
	}

	return ""
}
