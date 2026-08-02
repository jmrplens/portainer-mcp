package portainer

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
)

func TestNew_SendsAPIKeyHeader(t *testing.T) {
	t.Parallel()
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"x"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "ptr_secret", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := client.API.SystemStatusWithResponse(context.Background()); err != nil {
		t.Fatalf("SystemStatus: %v", err)
	}
	if gotKey != "ptr_secret" {
		t.Errorf("X-API-Key = %q, want the configured token", gotKey)
	}
}

func TestNew_SkipTLSVerifyFalse_RejectsSelfSignedCertificate(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := client.API.SystemStatusWithResponse(context.Background()); err == nil {
		t.Error("request succeeded against a self-signed certificate; TLS verification is not enforced")
	}
}

func TestNew_SkipTLSVerifyTrue_AcceptsSelfSignedCertificate(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"x"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "t", SkipTLSVerify: true, ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := client.API.SystemStatusWithResponse(context.Background()); err != nil {
		t.Errorf("request failed with SkipTLSVerify set: %v", err)
	}
	_ = tls.VersionTLS12 // keep the import meaningful if the test grows
}

func TestNew_EmptyURL_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := New(&config.Config{Token: "t"}); err == nil {
		t.Error("New() error = nil, want an error for an empty URL")
	}
}
