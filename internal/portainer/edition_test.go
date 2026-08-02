package portainer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
)

func TestDetectEdition_EEServer_ReportsEditionAndVersion(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerVersion":"2.43.0","ServerEdition":"EE","DatabaseVersion":"2.43.0"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, version, err := client.DetectEdition(context.Background())
	if err != nil {
		t.Fatalf("DetectEdition() error = %v", err)
	}
	if got != edition.EE {
		t.Errorf("edition = %q, want %q", got, edition.EE)
	}
	if version != "2.43.0" {
		t.Errorf("version = %q, want %q", version, "2.43.0")
	}
}

func TestDetectEdition_CEServer_ReportsCE(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerVersion":"2.44.0","ServerEdition":"CE"}`))
	}))
	t.Cleanup(server.Close)

	client, _ := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	got, _, err := client.DetectEdition(context.Background())
	if err != nil {
		t.Fatalf("DetectEdition() error = %v", err)
	}
	if got != edition.CE {
		t.Errorf("edition = %q, want %q", got, edition.CE)
	}
}

// A server that answers without ServerEdition must not be silently treated as
// EE: that would expose EE-only operations against a CE instance.
func TestDetectEdition_MissingEditionField_DefaultsToCE(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerVersion":"2.44.0"}`))
	}))
	t.Cleanup(server.Close)

	client, _ := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	got, _, err := client.DetectEdition(context.Background())
	if err != nil {
		t.Fatalf("DetectEdition() error = %v", err)
	}
	if got != edition.CE {
		t.Errorf("edition = %q, want CE when the server does not say", got)
	}
}

func TestDetectEdition_ServerError_ReturnsError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client, _ := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if _, _, err := client.DetectEdition(context.Background()); err == nil {
		t.Error("DetectEdition() error = nil, want an error for a 401")
	}
}

// TestDetectEdition_UnauthorizedWithJSONBody_IsClassified pins the error path,
// not merely that an error occurs. Portainer answers a rejected token with a
// JSON body; without ClassifyResponse that body decodes cleanly into an empty
// systemVersion and the function reports a healthy CE server with no error.
func TestDetectEdition_UnauthorizedWithJSONBody_IsClassified(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized","details":"invalid API key"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "bad", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	gotEdition, _, err := client.DetectEdition(context.Background())
	if err == nil {
		t.Fatalf("DetectEdition() error = nil with edition %q, want an error for a rejected token", gotEdition)
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false, want true; err = %v", err)
	}
	if gotEdition != "" {
		t.Errorf("edition = %q, want empty on an error path", gotEdition)
	}
}
