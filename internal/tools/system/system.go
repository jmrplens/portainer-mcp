// Package system declares the Portainer system actions.
//
// Four of the five are read-only and identical across editions. The fifth
// diverges: Community Edition exposes POST /system/upgrade and Business
// Edition exposes POST /system/update. Declaring them as two actions with
// different editions, rather than one with a branch, is what lets the catalog
// filter correctly without any domain code knowing which edition it is on.
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

// Specs declares every system action.
func Specs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		{
			Name: "system.info", Domain: "system", OperationID: "SystemInfo",
			Title:       "Get system information",
			Description: "Returns counts of agents and edge agents and other instance-wide information about this Portainer server.",
			Edition:     edition.CE,
			Handler:     systemInfo,
		},
		{
			Name: "system.status", Domain: "system", OperationID: "SystemStatus",
			Title:       "Get system status",
			Description: "Returns this Portainer server's version and instance identifier. Useful as a connectivity check.",
			Edition:     edition.CE,
			Handler:     systemStatus,
		},
		{
			Name: "system.version", Domain: "system", OperationID: "SystemVersion",
			Title:       "Get version details",
			Description: "Returns the running version, edition, database version and the versions of Docker, Helm, kubectl and Compose this server was built against.",
			Edition:     edition.CE,
			Handler:     systemVersion,
		},
		{
			Name: "system.nodes_count", Domain: "system", OperationID: "SystemNodesCount",
			Title:       "Count nodes",
			Description: "Returns the total number of nodes across every environment this Portainer server manages.",
			Edition:     edition.CE,
			Handler:     systemNodes,
		},
		{
			Name: "system.update", Domain: "system", OperationID: "SystemUpdate",
			Title:       "Update the Portainer server",
			Description: "Starts an update of this Portainer server. Business Edition only; Community Edition offers system.upgrade instead. The server restarts, so expect the connection to drop.",
			Edition:     edition.EE,
			Mutating:    true,
			Handler:     systemUpdate,
		},
		{
			Name: "system.upgrade", Domain: "system", OperationID: "SystemUpgrade",
			Title:       "Upgrade Community Edition to Business Edition",
			Description: "Upgrades this Community Edition server to Business Edition. Community Edition only. The server restarts, so expect the connection to drop.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Handler:     systemUpgrade,
		},
	}
}

func systemInfo(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.SystemInfoWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("system info: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("system info: %w", err)
	}
	return resp.JSON200, nil
}

func systemStatus(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.SystemStatusWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("system status: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("system status: %w", err)
	}
	return resp.JSON200, nil
}

func systemVersion(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.SystemVersionWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("system version: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("system version: %w", err)
	}
	return resp.JSON200, nil
}

func systemNodes(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.SystemNodesCountWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("system nodes: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("system nodes: %w", err)
	}
	return resp.JSON200, nil
}

func systemUpdate(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.SystemUpdateWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("system update: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("system update: %w", err)
	}
	return map[string]any{"started": true, "note": "The server restarts; expect this connection to drop."}, nil
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
