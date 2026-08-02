package portainer

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyResponse_Success_ReturnsNil(t *testing.T) {
	t.Parallel()
	for _, code := range []int{200, 201, 204, 302} {
		if err := ClassifyResponse(code, nil); err != nil {
			t.Errorf("ClassifyResponse(%d) = %v, want nil", code, err)
		}
	}
}

// Portainer answers a missing resource with a JSON body carrying message and
// details; both belong in the error a model will read.
func TestClassifyResponse_NotFoundWithJSONBody_CarriesMessageAndDetails(t *testing.T) {
	t.Parallel()
	body := []byte(`{"message":"Unable to find an environment with the specified identifier inside the database","details":"Object not found inside the database (bucket=endpoints, key=99999)"}`)

	err := ClassifyResponse(404, body)
	if err == nil {
		t.Fatal("ClassifyResponse(404) = nil, want an error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false, want true")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("errors.As(err, *APIError) = false; err is %T", err)
	}
	if !strings.Contains(apiErr.Message, "Unable to find an environment") {
		t.Errorf("Message = %q, want the API message", apiErr.Message)
	}
	if !strings.Contains(apiErr.Details, "bucket=endpoints") {
		t.Errorf("Details = %q, want the API details", apiErr.Details)
	}
}

// An absent route returns Go's plain-text mux 404, not JSON. It must still
// classify, and the raw body is the only diagnostic available.
func TestClassifyResponse_NotFoundWithPlainTextBody_StillClassifies(t *testing.T) {
	t.Parallel()
	err := ClassifyResponse(404, []byte("404 page not found\n"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false, want true")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("errors.As(err, *APIError) = false")
	}
	if !strings.Contains(apiErr.Message, "404 page not found") {
		t.Errorf("Message = %q, want the raw body preserved when it is not JSON", apiErr.Message)
	}
}

func TestClassifyResponse_AuthCodes_MapToSentinels(t *testing.T) {
	t.Parallel()
	if err := ClassifyResponse(401, nil); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("401 did not map to ErrUnauthorized, got %v", err)
	}
	if err := ClassifyResponse(403, nil); !errors.Is(err, ErrForbidden) {
		t.Errorf("403 did not map to ErrForbidden, got %v", err)
	}
}

func TestAPIError_Error_IncludesStatusAndMessage(t *testing.T) {
	t.Parallel()
	err := &APIError{StatusCode: 409, Message: "stack already exists"}
	got := err.Error()
	if !strings.Contains(got, "409") || !strings.Contains(got, "stack already exists") {
		t.Errorf("Error() = %q, want it to name the status and the message", got)
	}
}
