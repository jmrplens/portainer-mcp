package portainer

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// TestNew_LongOperation_IsNotCappedByAClientTimeout guards design acceptance
// criterion 6. An http.Client.Timeout would cut this off regardless of the
// context, which is the defect this test exists to prevent from returning.
func TestNew_LongOperation_IsNotCappedByAClientTimeout(t *testing.T) {
	t.Parallel()
	const serverDelay = 300 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(serverDelay):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"x"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// A context deadline comfortably longer than the server delay must be
	// honoured. If a client-level ceiling shorter than serverDelay is ever
	// reintroduced, this fails.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.API.SystemStatusWithResponse(ctx); err != nil {
		t.Errorf("a call slower than any default ceiling failed: %v", err)
	}
}

// TestNew_ContextDeadline_IsHonoured is the other half: the caller's deadline
// must still be able to cut a call short.
func TestNew_ContextDeadline_IsHonoured(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := client.API.SystemStatusWithResponse(ctx); err == nil {
		t.Fatal("call succeeded, want the context deadline to cut it short")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("call took %v, want the 50ms context deadline to apply promptly", elapsed)
	}
}

// TestNew_HTTPClient_HasNoTimeoutCeiling pins design acceptance criterion 6
// directly rather than by inference. http.Client.Timeout is an absolute ceiling
// that a per-call context cannot raise, so any non-zero value here silently
// caps a stack redeploy that pulls multi-gigabyte images — the exact defect
// that made the previous Portainer MCP server unusable for that job.
func TestNew_HTTPClient_HasNoTimeoutCeiling(t *testing.T) {
	t.Parallel()
	client, err := New(&config.Config{URL: "https://portainer.example.com", Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.httpClient.Timeout != 0 {
		t.Errorf("httpClient.Timeout = %v, want 0: a client-level ceiling cannot be raised by a caller's context", client.httpClient.Timeout)
	}
}

func TestNew_TrailingSlashURL_DoesNotProduceADoubleSlash(t *testing.T) {
	t.Parallel()
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"x"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(&config.Config{URL: server.URL + "/", Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := client.API.SystemStatusWithResponse(context.Background()); err != nil {
		t.Fatalf("SystemStatus: %v", err)
	}
	if strings.Contains(gotPath, "//") {
		t.Errorf("request path = %q, want no double slash", gotPath)
	}
}
