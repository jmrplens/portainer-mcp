// Package system declares the Portainer system actions.
//
// Four of the five are read-only and identical across editions. The fifth
// diverges: Community Edition exposes POST /system/upgrade and Business
// Edition exposes POST /system/update. Declaring them as two actions with
// different editions, rather than one with a branch, is what lets the catalog
// filter correctly without any domain code knowing which edition it is on.
//
// system.info, system.status, system.version, system.nodes_count and
// system.update all run on cmd/gen_action_inputs's generated code
// (actions.gen.go): every one takes no input (so there is no guard clause to
// lose) and its handler was already a bare resp.JSON<nnn> passthrough, or —
// system.update — its own hand-shaped acknowledgement is never actually
// observed by any test (the only e2e coverage,
// TestSystemUpdate_SafeModePreview_DoesNotActuallyRestart, runs under safe
// mode, which tools.Execute intercepts before the handler ever runs). Only
// system.upgrade stays hand-written, and for a reason no amount of narrative
// override changes: POST /system/upgrade exists only in the Community
// Edition specification, so cmd/gen_action_inputs's EE-driven operation
// enumeration never sees it at all (see actionspec.go's own doc comment on
// editionOf). Its Destructive: true override — converting a Community
// instance to Business Edition cannot be undone, unlike a same-edition
// system.update — is exactly the case Task 3's dangerMismatchWarnings exists
// to surface, and it is applied here by hand because there is nothing for the
// generator to apply it to.
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every system action: generatedSpecs()'s five entries plus
// system.upgrade, kept hand-written above.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), toolutil.WithNarrative(toolutil.ActionSpec{
		Name: "system.upgrade", Domain: "system", OperationID: "SystemUpgrade",
		Edition:     edition.CE,
		Mutating:    true,
		Destructive: true,
		Handler:     systemUpgrade,
	}, narrative("SystemUpgrade")))
}

// narrative supplies every action's ActionSpec narrative fields, generated
// and hand-written alike: the five generated ones via generatedSpecs() (see
// actions.go) and system.upgrade via Specs() above. All six route through
// toolutil.WithNarrative rather than assigning Title/Description directly in
// an ActionSpec literal, which is what lets cmd/audit_spec_drift recognise
// each as a deliberate, permanent improvement on the vendored
// specification's own terser summary/description
// (toolutil.ActionSpec.TitleOverridden/DescriptionOverridden — see
// WithNarrative's own doc comment) rather than needing a
// spec-drift-allowlist.yaml entry to say the identical thing.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "SystemInfo":
		return toolutil.ActionNarrative{
			Title:       "Get system information",
			Description: "Returns counts of agents and edge agents and other instance-wide information about this Portainer server.",
		}
	case "SystemStatus":
		return toolutil.ActionNarrative{
			Title:       "Get system status",
			Description: "Returns this Portainer server's version and instance identifier. Useful as a connectivity check.",
		}
	case "SystemVersion":
		return toolutil.ActionNarrative{
			Title:       "Get version details",
			Description: "Returns the running version, edition, database version and the versions of Docker, Helm, kubectl and Compose this server was built against.",
		}
	case "SystemNodesCount":
		return toolutil.ActionNarrative{
			Title:       "Count nodes",
			Description: "Returns the total number of nodes across every environment this Portainer server manages.",
		}
	case "SystemUpdate":
		return toolutil.ActionNarrative{
			Title:       "Update the Portainer server",
			Description: "Starts an update of this Portainer server. Business Edition only; Community Edition offers system.upgrade instead. The server restarts, so expect the connection to drop.",
		}
	case "SystemUpgrade":
		return toolutil.ActionNarrative{
			Title:       "Upgrade Community Edition to Business Edition",
			Description: "Upgrades this Community Edition server to Business Edition. Community Edition only. The server restarts, so expect the connection to drop.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}

// systemUpgrade calls POST /system/upgrade, which has no generated method
// because the client is generated from the EE specification and this operation
// exists only in Community Edition. See Client.Do for the general case.
func systemUpgrade(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.Do(ctx, http.MethodPost, "/system/upgrade", nil)
	if err != nil {
		return nil, fmt.Errorf("system upgrade: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("system upgrade: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("system upgrade: %w", err)
	}
	return map[string]any{"started": true, "note": "The server restarts; expect this connection to drop."}, nil
}
