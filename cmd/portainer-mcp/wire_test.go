package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
