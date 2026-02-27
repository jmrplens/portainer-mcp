package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/portainer/client"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
	"github.com/rs/zerolog/log"
)

const (
	// MinimumToolsVersion is the minimum supported version of the tools.yaml file.
	// This uses the same "v{major}.{minor}" format as tools.yaml version strings.
	MinimumToolsVersion = "v1.0"
	// SupportedPortainerVersion is the version of Portainer that is supported by this tool
	SupportedPortainerVersion = "2.31.2"
	// maxProxyResponseSize is the maximum allowed response body size (10MB) for Docker/K8s proxy calls
	maxProxyResponseSize = 10 * 1024 * 1024
)

// PortainerClient defines the contract between the MCP server and the Portainer API
// client wrapper. It abstracts all Portainer API interactions so that the MCP handlers
// never communicate with the Portainer HTTP API directly.
//
// The interface covers the following resource domains:
//   - Environments (endpoints): CRUD, snapshots, access control
//   - Environment groups and access groups: grouping and permission management
//   - Stacks: edge stacks and regular (non-edge) compose/swarm stacks
//   - Users and teams: identity and team membership management
//   - Settings: server, public, and SSL configuration
//   - Templates: application templates and custom templates
//   - Registries: container registry management
//   - Docker and Kubernetes proxies: raw API pass-through to container engines
//   - Tags, roles, webhooks, backups, edge jobs, Helm, auth, and system status
//
// Implementations must be safe for concurrent use by multiple MCP handler goroutines.
type PortainerClient interface {
	// Tag methods
	GetEnvironmentTags() ([]models.EnvironmentTag, error)
	CreateEnvironmentTag(name string) (int, error)
	DeleteEnvironmentTag(id int) error

	// Environment methods
	GetEnvironments() ([]models.Environment, error)
	GetEnvironment(id int) (models.Environment, error)
	DeleteEnvironment(id int) error
	SnapshotEnvironment(id int) error
	SnapshotAllEnvironments() error
	UpdateEnvironmentTags(id int, tagIds []int) error
	UpdateEnvironmentUserAccesses(id int, userAccesses map[int]string) error
	UpdateEnvironmentTeamAccesses(id int, teamAccesses map[int]string) error

	// Environment Group methods
	GetEnvironmentGroups() ([]models.Group, error)
	CreateEnvironmentGroup(name string, environmentIds []int) (int, error)
	UpdateEnvironmentGroupName(id int, name string) error
	UpdateEnvironmentGroupEnvironments(id int, environmentIds []int) error
	UpdateEnvironmentGroupTags(id int, tagIds []int) error

	// Access Group methods
	GetAccessGroups() ([]models.AccessGroup, error)
	CreateAccessGroup(name string, environmentIds []int) (int, error)
	UpdateAccessGroupName(id int, name string) error
	UpdateAccessGroupUserAccesses(id int, userAccesses map[int]string) error
	UpdateAccessGroupTeamAccesses(id int, teamAccesses map[int]string) error
	AddEnvironmentToAccessGroup(id int, environmentId int) error
	RemoveEnvironmentFromAccessGroup(id int, environmentId int) error

	// Stack methods
	GetStacks() ([]models.Stack, error)
	GetStackFile(id int) (string, error)
	CreateStack(name string, file string, environmentGroupIds []int) (int, error)
	UpdateStack(id int, file string, environmentGroupIds []int) error

	// Regular stack methods
	GetRegularStacks() ([]models.RegularStack, error)
	InspectStack(id int) (models.RegularStack, error)
	DeleteStack(id int, endpointID int, removeVolumes bool) error
	InspectStackFile(id int) (string, error)
	UpdateStackGit(id int, endpointID int, referenceName string, prune bool) (models.RegularStack, error)
	RedeployStackGit(id int, endpointID int, pullImage bool, prune bool) (models.RegularStack, error)
	StartStack(id int, endpointID int) (models.RegularStack, error)
	StopStack(id int, endpointID int) (models.RegularStack, error)
	MigrateStack(id int, endpointID int, targetEndpointID int, name string) (models.RegularStack, error)

	// Team methods
	CreateTeam(name string) (int, error)
	GetTeam(id int) (models.Team, error)
	GetTeams() ([]models.Team, error)
	DeleteTeam(id int) error
	UpdateTeamName(id int, name string) error
	UpdateTeamMembers(id int, userIds []int) error

	// User methods
	CreateUser(username, password, role string) (int, error)
	GetUser(id int) (models.User, error)
	GetUsers() ([]models.User, error)
	DeleteUser(id int) error
	UpdateUserRole(id int, role string) error

	// Settings methods
	GetSettings() (models.PortainerSettings, error)
	UpdateSettings(settingsJSON map[string]interface{}) error
	GetPublicSettings() (models.PublicSettings, error)

	// SSL methods
	GetSSLSettings() (models.SSLSettings, error)
	UpdateSSLSettings(cert, key string, httpEnabled *bool) error

	// App Template methods
	GetAppTemplates() ([]models.AppTemplate, error)
	GetAppTemplateFile(id int) (string, error)

	// Version methods
	GetVersion() (string, error)

	// Docker Proxy methods
	ProxyDockerRequest(opts models.DockerProxyRequestOptions) (*http.Response, error)
	GetDockerDashboard(environmentId int) (models.DockerDashboard, error)

	// Kubernetes Proxy methods
	ProxyKubernetesRequest(opts models.KubernetesProxyRequestOptions) (*http.Response, error)

	// Kubernetes Native methods
	GetKubernetesDashboard(environmentId int) (models.KubernetesDashboard, error)
	GetKubernetesNamespaces(environmentId int) ([]models.KubernetesNamespace, error)
	GetKubernetesConfig(environmentId int) (interface{}, error)

	GetWebhooks() ([]models.Webhook, error)
	CreateWebhook(resourceId string, endpointId int, webhookType int) (int, error)
	DeleteWebhook(id int) error

	// System methods
	GetSystemStatus() (models.SystemStatus, error)

	// Custom Template methods
	GetCustomTemplates() ([]models.CustomTemplate, error)
	GetCustomTemplate(id int) (models.CustomTemplate, error)
	GetCustomTemplateFile(id int) (string, error)
	CreateCustomTemplate(title, description, note, logo, fileContent string, platform, templateType int) (int, error)
	DeleteCustomTemplate(id int) error

	// Registry methods
	GetRegistries() ([]models.Registry, error)
	GetRegistry(id int) (models.Registry, error)
	CreateRegistry(name string, registryType int, url string, authentication bool, username string, password string, baseURL string) (int, error)
	UpdateRegistry(id int, name *string, url *string, authentication *bool, username *string, password *string, baseURL *string) error
	DeleteRegistry(id int) error

	// Backup methods
	GetBackupStatus() (models.BackupStatus, error)
	GetBackupS3Settings() (models.S3BackupSettings, error)
	CreateBackup(password string) error
	BackupToS3(settings models.S3BackupSettings) error
	RestoreFromS3(accessKeyID, bucketName, filename, password, region, s3CompatibleHost, secretAccessKey string) error

	// Role methods
	GetRoles() ([]models.Role, error)

	// MOTD methods
	GetMOTD() (models.MOTD, error)

	// Edge Job methods
	GetEdgeJobs() ([]models.EdgeJob, error)
	GetEdgeJob(id int) (models.EdgeJob, error)
	GetEdgeJobFile(id int) (string, error)
	CreateEdgeJob(name, cronExpression, fileContent string, endpoints []int, edgeGroups []int, recurring bool) (int, error)
	DeleteEdgeJob(id int) error

	// Edge Update Schedule methods
	GetEdgeUpdateSchedules() ([]models.EdgeUpdateSchedule, error)

	// Auth methods
	AuthenticateUser(username, password string) (models.AuthResponse, error)
	Logout() error

	// Helm methods
	GetHelmRepositories(userId int) (models.HelmRepositoryList, error)
	CreateHelmRepository(userId int, url string) (models.HelmRepository, error)
	DeleteHelmRepository(userId int, repositoryId int) error
	SearchHelmCharts(repo string, chart string) (string, error)
	InstallHelmChart(environmentId int, chart, name, namespace, repo, values, version string) (models.HelmReleaseDetails, error)
	GetHelmReleases(environmentId int, namespace, filter, selector string) ([]models.HelmRelease, error)
	DeleteHelmRelease(environmentId int, release, namespace string) error
	GetHelmReleaseHistory(environmentId int, name, namespace string) ([]models.HelmReleaseDetails, error)
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

	tools, err := toolgen.LoadToolsFromYAML(toolsPath, MinimumToolsVersion)
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
// For example, "2.31.2" returns "2.31" and "2.31" returns "2.31".
func majorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}
