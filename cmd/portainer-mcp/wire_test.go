package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools/dynamic"
	"github.com/jmrplens/portainer-mcp/internal/tools/individual"
	"github.com/jmrplens/portainer-mcp/internal/tools/meta"
)

func versionServer(t *testing.T, ed, version string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerVersion":"` + version + `","ServerEdition":"` + ed + `"}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestBuildCatalog_DetectsEditionFromServer(t *testing.T) {
	t.Parallel()
	srv := versionServer(t, "EE", "2.44.0")
	cfg := &config.Config{URL: srv.URL, Token: "t", ToolSurface: config.SurfaceDynamic}
	client, err := portainer.New(cfg)
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}

	_, gotEdition, gotVersion, err := buildCatalog(context.Background(), cfg, client, slog.Default())
	if err != nil {
		t.Fatalf("buildCatalog: %v", err)
	}
	if gotEdition != edition.EE {
		t.Errorf("edition = %q, want EE", gotEdition)
	}
	if gotVersion != "2.44.0" {
		t.Errorf("version = %q, want 2.44.0", gotVersion)
	}
}

// The P1 carry-forward item: PORTAINER_EDITION must override detection, and
// today it is read by nothing.
func TestBuildCatalog_ConfiguredEdition_OverridesDetection(t *testing.T) {
	t.Parallel()
	srv := versionServer(t, "EE", "2.44.0")
	cfg := &config.Config{URL: srv.URL, Token: "t", ToolSurface: config.SurfaceDynamic, Edition: edition.CE}
	client, err := portainer.New(cfg)
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}

	_, gotEdition, _, err := buildCatalog(context.Background(), cfg, client, slog.Default())
	if err != nil {
		t.Fatalf("buildCatalog: %v", err)
	}
	if gotEdition != edition.CE {
		t.Errorf("edition = %q, want CE: the configured value must override what the server reports", gotEdition)
	}
}

// A server that cannot be reached must not silently produce an empty catalog.
func TestBuildCatalog_UnreachableServer_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{URL: "http://127.0.0.1:1", Token: "t", ToolSurface: config.SurfaceDynamic}
	client, err := portainer.New(cfg)
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	if _, _, _, err := buildCatalog(context.Background(), cfg, client, slog.Default()); err == nil {
		t.Fatal("buildCatalog() = nil, want an error when the server cannot be reached")
	}
}

// A server that accepts the connection and then never answers must not hang
// startup: the client sets no transport timeout by design, so the deadline has
// to come from the context the caller supplies.
func TestBuildCatalog_UnresponsiveServer_FailsWithinTheDeadline(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic}
	client, err := portainer.New(cfg)
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, _, _, err := buildCatalog(ctx, cfg, client, slog.Default()); err == nil {
		t.Fatal("buildCatalog() = nil, want an error when the server never answers")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("buildCatalog took %v; the context deadline did not bound it", elapsed)
	}
}

// TestRun_AppliesTheStartupDeadline guards against the deadline being declared
// and never wired, which is the state this fix corrects.
//
// This is an interim source-level guard, not a behavioral test: it greps
// main.go for the identifier portainer.DefaultCallTimeout, so it passes if
// that identifier merely appears in a comment, and it would fail to catch the
// same deadline being applied through a differently named constant or moved
// to another file. TestBuildCatalog_UnresponsiveServer_FailsWithinTheDeadline
// already proves a context deadline bounds detection at the buildCatalog
// level; a behavioral version of this test would need the startup sequence
// in run extracted so it can be called with a stub clock or a short deadline,
// which is out of scope here and belongs to P3, once run is restructured for
// injection.
func TestRun_AppliesTheStartupDeadline(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(source), "portainer.DefaultCallTimeout") {
		t.Error("run does not apply portainer.DefaultCallTimeout to the startup detection; an unresponsive server would hang startup indefinitely")
	}
}

func TestSurfaceFor_EachSurface_ReturnsItsProjection(t *testing.T) {
	t.Parallel()
	if _, ok := surfaceFor(config.SurfaceDynamic).(dynamic.Surface); !ok {
		t.Error("SurfaceDynamic did not map to the dynamic surface")
	}
	if _, ok := surfaceFor(config.SurfaceMeta).(meta.Surface); !ok {
		t.Error("SurfaceMeta did not map to the meta surface")
	}
	if _, ok := surfaceFor(config.SurfaceIndividual).(individual.Surface); !ok {
		t.Error("SurfaceIndividual did not map to the individual surface")
	}
}
