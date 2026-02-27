//go:build live

// Package live provides integration tests that run against a real Portainer instance.
// These tests are gated by the "live" build tag and require environment variables:
//
//	PORTAINER_LIVE_URL   - Portainer server address (no protocol, e.g. "192.168.0.40:31015")
//	PORTAINER_LIVE_TOKEN - API token with admin privileges
//
// Run with: go test -v -tags=live -timeout 300s ./tests/live/...
package live

import (
	"context"
	"os"
	"testing"

	"github.com/jmrplens/portainer-mcp-enhanced/internal/mcp"
	"github.com/stretchr/testify/require"
)

const (
	toolsPath = "../../internal/tooldef/tools.yaml"
)

// liveEnv holds references to the MCP server connected to the live Portainer instance
type liveEnv struct {
	ctx    context.Context
	server *mcp.PortainerMCPServer
	url    string
	token  string
}

// newLiveEnv creates a new live test environment from environment variables
func newLiveEnv(t *testing.T) *liveEnv {
	t.Helper()

	url := os.Getenv("PORTAINER_LIVE_URL")
	token := os.Getenv("PORTAINER_LIVE_TOKEN")

	if url == "" || token == "" {
		t.Skip("PORTAINER_LIVE_URL and PORTAINER_LIVE_TOKEN must be set")
	}

	mcpServer, err := mcp.NewPortainerMCPServer(url, token, toolsPath,
		mcp.WithDisableVersionCheck(true),
	)
	require.NoError(t, err, "Failed to create MCP server for live Portainer")

	return &liveEnv{
		ctx:    context.Background(),
		server: mcpServer,
		url:    url,
		token:  token,
	}
}
