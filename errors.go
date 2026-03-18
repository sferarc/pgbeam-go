package pgbeam

import (
	"encoding/json"
	"errors"
	"fmt"
)

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
