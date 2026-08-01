package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog/log"

	"github.com/jmrplens/portainer-mcp-enhanced/internal/tooldef"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/client"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/toolgen"
)

const (
	// MinimumToolsVersion is the minimum supported version of the tools.yaml file.
	// This uses the same "v{major}.{minor}" format as tools.yaml version strings.
	MinimumToolsVersion = "v1.0"
	// SupportedPortainerVersion is the version of Portainer that is supported by this tool
	SupportedPortainerVersion = "2.39.1"
	// maxProxyResponseSize is the maximum allowed response body size (10MB) for Docker/K8s proxy calls
	maxProxyResponseSize = 10 * 1024 * 1024
)

// PortainerClient defines the contract between the MCP server and the Portainer API
// client wrapper. It abstracts all Portainer API interactions so that the MCP handlers
// never communicate with the Portainer HTTP API directly.
//
// It composes 18 domain-specific interfaces defined in client_interfaces.go,
// covering environments, stacks, users, teams, Docker/Kubernetes proxies,
// Helm, registries, templates, backups, edge compute, settings, and system.
//
// Implementations must be safe for concurrent use by multiple MCP handler goroutines.
type PortainerClient interface {
	TagClient
	EnvironmentClient
	EnvironmentGroupClient
	AccessGroupClient
	StackClient
	TeamClient
	UserClient
	SettingsClient
	TemplateClient
	DockerClient
	KubernetesClient
	RegistryClient
	BackupClient
	WebhookClient
	EdgeClient
	HelmClient
	AuthClient
	SystemClient
}

// PortainerMCPServer is the main MCP server that bridges AI assistants and the
// Portainer API. It registers tool definitions loaded from a YAML file, routes
// incoming MCP tool-call requests to the appropriate handlers, and communicates
// with Portainer through the [PortainerClient] interface. The server supports
// read-only mode to prevent modifications and listens on stdio for MCP messages.
type PortainerMCPServer struct {
	srv      *server.MCPServer
	cli      PortainerClient
	tools    map[string]mcp.Tool
	readOnly bool
}

// ServerOption is a functional option for configuring a [PortainerMCPServer].
// Pass one or more options to [NewPortainerMCPServer] to customise behaviour.
type ServerOption func(*serverOptions)

// serverOptions contains all configurable options for the server
type serverOptions struct {
	client              PortainerClient
	readOnly            bool
	granularTools       bool
	disableVersionCheck bool
	skipTLSVerify       bool
}

// WithClient sets a custom client for the server.
// This is primarily used for testing to inject mock clients.
func WithClient(client PortainerClient) ServerOption {
	return func(opts *serverOptions) {
		opts.client = client
	}
}

// WithReadOnly sets the server to read-only mode.
// This will prevent the server from registering write tools.
func WithReadOnly(readOnly bool) ServerOption {
	return func(opts *serverOptions) {
		opts.readOnly = readOnly
	}
}

// WithGranularTools enables granular tool mode, registering all ~98 individual
// tools instead of the default ~15 grouped meta-tools.
func WithGranularTools(granular bool) ServerOption {
	return func(opts *serverOptions) {
		opts.granularTools = granular
	}
}

// WithDisableVersionCheck disables the Portainer server version check.
// This allows connecting to unsupported Portainer versions.
func WithDisableVersionCheck(disable bool) ServerOption {
	return func(opts *serverOptions) {
		opts.disableVersionCheck = disable
	}
}

// WithSkipTLSVerify skips TLS certificate verification when connecting to Portainer.
// This should only be used for development/testing with self-signed certificates.
func WithSkipTLSVerify(skip bool) ServerOption {
	return func(opts *serverOptions) {
		opts.skipTLSVerify = skip
	}
}

// NewPortainerMCPServer creates a new Portainer MCP server.
//
// This server provides an implementation of the MCP protocol for Portainer,
// allowing AI assistants to interact with Portainer through a structured API.
//
// Parameters:
//   - serverURL: The base URL of the Portainer server (e.g., "https://portainer.example.com")
//   - token: The API token for authenticating with the Portainer server
//   - toolsPath: Path to the tools.yaml file that defines the available MCP tools
//   - options: Optional functional options for customizing server behavior (e.g., WithClient)
//
// Returns:
//   - A configured PortainerMCPServer instance ready to be started
//   - An error if initialization fails
//
// Possible errors:
//   - Failed to load tools from the specified path
//   - Failed to communicate with the Portainer server
//   - Incompatible Portainer server version
func NewPortainerMCPServer(serverURL, token, toolsPath string, options ...ServerOption) (*PortainerMCPServer, error) {
	opts := &serverOptions{}

	for _, option := range options {
		option(opts)
	}

	var (
		tools map[string]mcp.Tool
		err   error
	)
	if toolsPath != "" {
		tools, err = toolgen.LoadToolsFromYAML(toolsPath, MinimumToolsVersion)
	} else {
		tools, err = toolgen.LoadToolsFromBytes(tooldef.ToolsFile, MinimumToolsVersion)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load tools: %w", err)
	}

	var portainerClient PortainerClient
	if opts.client != nil {
		portainerClient = opts.client
	} else {
		portainerClient = client.NewPortainerClient(serverURL, token, client.WithSkipTLSVerify(opts.skipTLSVerify))
	}

	if !opts.disableVersionCheck {
		version, err := portainerClient.GetVersion()
		if err != nil {
			return nil, fmt.Errorf("failed to get Portainer server version: %w", err)
		}

		if !isCompatibleVersion(version, SupportedPortainerVersion) {
			return nil, fmt.Errorf("unsupported Portainer server version: %s, only version %s.x is supported", version, SupportedPortainerVersion)
		}
	}

	return &PortainerMCPServer{
		srv: server.NewMCPServer(
			"Portainer MCP Server",
			"0.5.1",
			server.WithToolCapabilities(true),
			server.WithLogging(),
		),
		cli:      portainerClient,
		tools:    tools,
		readOnly: opts.readOnly,
	}, nil
}

// Start begins listening for MCP protocol messages on standard input/output.
// It handles SIGINT and SIGTERM for graceful shutdown.
func (s *PortainerMCPServer) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ServeStdio(s.srv)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info().Msg("Received shutdown signal, stopping server")
		return nil
	}
}

// addToolIfExists adds a tool to the server if it exists in the tools map
func (s *PortainerMCPServer) addToolIfExists(toolName string, handler server.ToolHandlerFunc) {
	if tool, exists := s.tools[toolName]; exists {
		s.srv.AddTool(tool, handler)
	} else {
		log.Warn().Str("tool", toolName).Msg("Tool not found, will not be registered for MCP usage")
	}
}

// isCompatibleVersion checks if the actual version is compatible with the supported version.
// It compares only the major.minor components, allowing patch version differences.
func isCompatibleVersion(actual, supported string) bool {
	return majorMinor(actual) == majorMinor(supported)
}

// majorMinor extracts the "major.minor" prefix from a version string.
// For example, "2.39.1" returns "2.39" and "2.39" returns "2.39".
func majorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}
