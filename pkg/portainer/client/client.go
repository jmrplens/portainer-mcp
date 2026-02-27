// Package client provides the Portainer API client implementation.
// It defines the PortainerAPIClient interface and its HTTP-based implementation
// that communicates with the Portainer server API. The client handles
// authentication, request construction, and response transformation between
// raw API models and the local model types used by the MCP server.
package client

import (
	"net/http"

	"github.com/portainer/client-api-go/v2/client"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// PortainerAPIClient defines the interface for the underlying Portainer API client
type PortainerAPIClient interface {
	ListEdgeGroups() ([]*apimodels.EdgegroupsDecoratedEdgeGroup, error)
	CreateEdgeGroup(name string, environmentIds []int64) (int64, error)
	UpdateEdgeGroup(id int64, name *string, environmentIds *[]int64, tagIds *[]int64) error
	ListEdgeStacks() ([]*apimodels.PortainereeEdgeStack, error)
	ListRegularStacks() ([]*apimodels.PortainereeStack, error)
	CreateEdgeStack(name string, file string, environmentGroupIds []int64) (int64, error)
	UpdateEdgeStack(id int64, file string, environmentGroupIds []int64) error
	GetEdgeStackFile(id int64) (string, error)
	ListEndpointGroups() ([]*apimodels.PortainerEndpointGroup, error)
	CreateEndpointGroup(name string, associatedEndpoints []int64) (int64, error)
	UpdateEndpointGroup(id int64, name *string, userAccesses *map[int64]string, teamAccesses *map[int64]string) error
	AddEnvironmentToEndpointGroup(groupId int64, environmentId int64) error
	RemoveEnvironmentFromEndpointGroup(groupId int64, environmentId int64) error
	ListEndpoints() ([]*apimodels.PortainereeEndpoint, error)
	GetEndpoint(id int64) (*apimodels.PortainereeEndpoint, error)
	UpdateEndpoint(id int64, tagIds *[]int64, userAccesses *map[int64]string, teamAccesses *map[int64]string) error
	DeleteEndpoint(id int64) error
	SnapshotEndpoint(id int64) error
	SnapshotAllEndpoints() error
	GetSettings() (*apimodels.PortainereeSettings, error)
	UpdateSettings(payload *apimodels.SettingsSettingsUpdatePayload) error
	GetPublicSettings() (*apimodels.SettingsPublicSettingsResponse, error)
	GetSSLSettings() (*apimodels.PortainereeSSLSettings, error)
	UpdateSSLSettings(payload *apimodels.SslSslUpdatePayload) error
	ListAppTemplates() ([]*apimodels.PortainerTemplate, error)
	GetAppTemplateFile(id int64) (string, error)
	ListTags() ([]*apimodels.PortainerTag, error)
	CreateTag(name string) (int64, error)
	DeleteTag(id int64) error
	ListTeams() ([]*apimodels.PortainerTeam, error)
	GetTeam(id int64) (*apimodels.PortainerTeam, error)
	ListTeamMemberships() ([]*apimodels.PortainerTeamMembership, error)
	CreateTeam(name string) (int64, error)
	UpdateTeamName(id int, name string) error
	DeleteTeam(id int64) error
	DeleteTeamMembership(id int) error
	CreateTeamMembership(teamId int, userId int) error
	ListUsers() ([]*apimodels.PortainereeUser, error)
	CreateUser(username, password string, role int64) (int64, error)
	GetUser(id int) (*apimodels.PortainereeUser, error)
	DeleteUser(id int64) error
	UpdateUserRole(id int, role int64) error
	GetVersion() (string, error)
	GetSystemStatus() (*apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus, error)
	ListRegistries() ([]*apimodels.PortainereeRegistry, error)
	GetRegistryByID(id int64) (*apimodels.PortainereeRegistry, error)
	CreateRegistry(body *apimodels.RegistriesRegistryCreatePayload) (int64, error)
	UpdateRegistry(id int64, body *apimodels.RegistriesRegistryUpdatePayload) error
	DeleteRegistry(id int64) error
	ProxyDockerRequest(environmentId int, opts client.ProxyRequestOptions) (*http.Response, error)
	ProxyKubernetesRequest(environmentId int, opts client.ProxyRequestOptions) (*http.Response, error)
	ListCustomTemplates() ([]*apimodels.PortainereeCustomTemplate, error)
	GetCustomTemplate(id int64) (*apimodels.PortainereeCustomTemplate, error)
	GetCustomTemplateFile(id int64) (string, error)
	CreateCustomTemplate(payload *apimodels.CustomtemplatesCustomTemplateFromFileContentPayload) (*apimodels.PortainereeCustomTemplate, error)
	DeleteCustomTemplate(id int64) error
	GetBackupStatus() (*apimodels.BackupBackupStatus, error)
	GetBackupSettings() (*apimodels.PortainereeS3BackupSettings, error)
	CreateBackup(password string) error
	BackupToS3(body *apimodels.BackupS3BackupPayload) error
	RestoreFromS3(body *apimodels.BackupRestoreS3Settings) error
	ListRoles() ([]*apimodels.PortainereeRole, error)
	GetMOTD() (map[string]any, error)
	AuthenticateUser(username, password string) (*apimodels.AuthAuthenticateResponse, error)
	Logout() error
	ListWebhooks() ([]*apimodels.PortainerWebhook, error)
	CreateWebhook(resourceId string, endpointId int64, webhookType int64) (int64, error)
	DeleteWebhook(id int64) error
	ListEdgeJobs() ([]*apimodels.PortainerEdgeJob, error)
	GetEdgeJob(id int64) (*apimodels.PortainerEdgeJob, error)
	GetEdgeJobFile(id int64) (string, error)
	CreateEdgeJob(payload *apimodels.EdgejobsEdgeJobCreateFromFileContentPayload) (int64, error)
	DeleteEdgeJob(id int64) error
	ListEdgeUpdateSchedules() ([]*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule, error)
	ListHelmRepositories(userId int64) (*apimodels.UsersHelmUserRepositoryResponse, error)
	CreateHelmRepository(userId int64, url string) (*apimodels.PortainerHelmUserRepository, error)
	DeleteHelmRepository(userId int64, repositoryId int64) error
	SearchHelmCharts(repo string, chart *string) (string, error)
	InstallHelmChart(environmentId int64, payload *apimodels.HelmInstallChartPayload) (*apimodels.ReleaseRelease, error)
	ListHelmReleases(environmentId int64, namespace *string, filter *string, selector *string) ([]*apimodels.ReleaseReleaseElement, error)
	DeleteHelmRelease(environmentId int64, release string, namespace *string) error
	GetHelmReleaseHistory(environmentId int64, name string, namespace *string) ([]*apimodels.ReleaseRelease, error)
	GetDockerDashboard(environmentId int64) (*apimodels.DockerDashboardResponse, error)
	GetKubernetesDashboard(environmentId int64) (*apimodels.KubernetesK8sDashboard, error)
	GetKubernetesNamespaces(environmentId int64) ([]*apimodels.PortainerK8sNamespaceInfo, error)
	GetKubernetesConfig(environmentId int64) (interface{}, error)
	StackInspect(id int64) (*apimodels.PortainereeStack, error)
	StackDelete(id int64, endpointID int64, removeVolumes bool) error
	StackFileInspect(id int64) (string, error)
	StackUpdateGit(id int64, endpointID int64, body *apimodels.StacksStackGitUpdatePayload) (*apimodels.PortainereeStack, error)
	StackGitRedeploy(id int64, endpointID int64, body *apimodels.StacksStackGitRedployPayload) (*apimodels.PortainereeStack, error)
	StackStart(id int64, endpointID int64) (*apimodels.PortainereeStack, error)
	StackStop(id int64, endpointID int64) (*apimodels.PortainereeStack, error)
	StackMigrate(id int64, endpointID int64, body *apimodels.StacksStackMigratePayload) (*apimodels.PortainereeStack, error)
}

// PortainerClient is a wrapper around the Portainer SDK client
// that provides simplified access to Portainer API functionality.
type PortainerClient struct {
	cli PortainerAPIClient
}

// ClientOption defines a function that configures a PortainerClient.
type ClientOption func(*clientOptions)

// clientOptions holds configuration options for the PortainerClient.
type clientOptions struct {
	skipTLSVerify bool
}

// WithSkipTLSVerify configures whether to skip TLS certificate verification.
// Setting this to true is not recommended for production environments.
func WithSkipTLSVerify(skip bool) ClientOption {
	return func(o *clientOptions) {
		o.skipTLSVerify = skip
	}
}

// NewPortainerClient creates a new PortainerClient instance with the provided
// server URL and authentication token.
//
// Parameters:
//   - serverURL: The base URL of the Portainer server
//   - token: The authentication token for API access
//   - opts: Optional configuration options for the client
//
// Returns:
//   - A configured PortainerClient ready for API operations
func NewPortainerClient(serverURL string, token string, opts ...ClientOption) *PortainerClient {
	options := clientOptions{
		skipTLSVerify: false, // Default to secure TLS verification
	}

	for _, opt := range opts {
		opt(&options)
	}

	return &PortainerClient{
		cli: newPortainerAPIAdapter(serverURL, token, options.skipTLSVerify),
	}
}
