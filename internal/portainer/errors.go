// Package portainer wraps the generated Portainer API client with
// authentication, TLS policy and error classification.
package portainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Sentinel errors for the status codes callers branch on.
var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

// APIError is a non-success response from Portainer.
type APIError struct {
	StatusCode int
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("portainer: %d %s: %s", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("portainer: %d %s", e.StatusCode, e.Message)
}

// Is lets errors.Is match an APIError against the sentinels above.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case http.StatusNotFound:
		return target == ErrNotFound
	case http.StatusUnauthorized:
		return target == ErrUnauthorized
	case http.StatusForbidden:
		return target == ErrForbidden
	}
	return false
}

// portainerErrorBody is the JSON shape Portainer returns for handled errors.
type portainerErrorBody struct {
	Message string `json:"message"`
	Details string `json:"details"`
}

// ClassifyResponse converts a non-success status into an *APIError, or returns
// nil for anything below 400.
//
// Portainer distinguishes two kinds of 404, and the body is what tells them
// apart: a handled "resource missing" answers with JSON carrying message and
// details, while an absent route falls through to Go's mux and answers with
// plain text. Both are preserved so a caller can tell a missing stack from a
// route this Portainer version does not implement.
func ClassifyResponse(statusCode int, body []byte) error {
	if statusCode < 400 {
		return nil
	}
	apiErr := &APIError{StatusCode: statusCode}

	var parsed portainerErrorBody
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Message != "" {
		apiErr.Message = parsed.Message
		apiErr.Details = parsed.Details
		return apiErr
	}

	apiErr.Message = strings.TrimSpace(string(body))
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(statusCode)
	}
	return apiErr
}
