// Package wiring builds the action catalog and the MCP server it is
// projected onto — the single construction path cmd/portainer-mcp uses to
// build the real binary.
//
// It exists as its own package, rather than living in cmd/portainer-mcp,
// because the e2e harness (test/e2e/suite) needs to build servers the exact
// same way the binary does, and Go refuses to import a package main from
// anywhere else. Without this package the harness would have to duplicate
// the wiring, and a divergence between the two would be invisible to every
// suite built on top of it.
package wiring

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/dynamic"
	"github.com/jmrplens/portainer-mcp/internal/tools/individual"
	"github.com/jmrplens/portainer-mcp/internal/tools/meta"
	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/tools/tags"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// AllSpecs collects every declared action. Later phases append their domains
// here; this is the only place that changes when a domain is added.
func AllSpecs() []toolutil.ActionSpec {
	var specs []toolutil.ActionSpec
	specs = append(specs, system.Specs()...)
	specs = append(specs, tags.Specs()...)
	specs = append(specs, registries.Specs()...)
	return specs
}

// BuildCatalog resolves the target server's edition and version, then builds
// the catalog for it.
//
// The configured edition wins over detection when set: an operator who knows
// their instance can skip a round-trip, and in HTTP mode detection per token is
// wasteful. Detection still runs, because the server version is needed either
// way and only the server can supply it.
func BuildCatalog(ctx context.Context, cfg *config.Config, client *portainer.Client, logger *slog.Logger) (*actioncatalog.Catalog, edition.Edition, string, error) {
	detected, version, err := client.DetectEdition(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve server edition: %w", err)
	}

	resolved := detected
	if cfg.Edition != "" {
		if cfg.Edition != detected {
			logger.Warn("configured edition differs from the server's",
				"configured", cfg.Edition, "detected", detected)
		}
		resolved = cfg.Edition
	}

	catalog, err := actioncatalog.Build(AllSpecs(), actioncatalog.Options{
		Edition:       resolved,
		ServerVersion: version,
		ReadOnly:      cfg.ReadOnly,
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("build action catalog: %w", err)
	}
	return catalog, resolved, version, nil
}

// SurfaceFor maps the configured surface to its projection.
func SurfaceFor(s config.ToolSurface) tools.Surface {
	switch s {
	case config.SurfaceMeta:
		return meta.Surface{}
	case config.SurfaceIndividual:
		return individual.Surface{}
	default:
		return dynamic.Surface{}
	}
}
