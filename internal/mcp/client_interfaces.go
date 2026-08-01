package mcp

import (
	"net/http"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// Domain-specific client interfaces that compose into PortainerClient.
// Each interface corresponds to one or more handler files and represents
// a cohesive set of API operations. Implementations must be safe for
// concurrent use by multiple MCP handler goroutines.

// TagClient manages environment tags.
// Used by: tag.go
type TagClient interface {
	GetEnvironmentTags() ([]models.EnvironmentTag, error)
	CreateEnvironmentTag(name string) (int, error)
	DeleteEnvironmentTag(id int) error
}

// EnvironmentClient manages Portainer environments (endpoints), including
// snapshots and access control.
// Used by: environment.go
type EnvironmentClient interface {
	GetEnvironments() ([]models.Environment, error)
	GetEnvironment(id int) (models.Environment, error)
	DeleteEnvironment(id int) error
	SnapshotEnvironment(id int) error
	SnapshotAllEnvironments() error
	UpdateEnvironmentTags(id int, tagIds []int) error
	UpdateEnvironmentUserAccesses(id int, userAccesses map[int]string) error
	UpdateEnvironmentTeamAccesses(id int, teamAccesses map[int]string) error
}

// EnvironmentGroupClient manages environment groups.
// Used by: group.go
type EnvironmentGroupClient interface {
	GetEnvironmentGroups() ([]models.Group, error)
	CreateEnvironmentGroup(name string, environmentIds []int) (int, error)
	UpdateEnvironmentGroupName(id int, name string) error
	UpdateEnvironmentGroupEnvironments(id int, environmentIds []int) error
	UpdateEnvironmentGroupTags(id int, tagIds []int) error
}

// AccessGroupClient manages access groups (edge groups in Portainer)
// for environment-level permissions.
// Used by: access_group.go
type AccessGroupClient interface {
	GetAccessGroups() ([]models.AccessGroup, error)
	CreateAccessGroup(name string, environmentIds []int) (int, error)
	UpdateAccessGroupName(id int, name string) error
	UpdateAccessGroupUserAccesses(id int, userAccesses map[int]string) error
	UpdateAccessGroupTeamAccesses(id int, teamAccesses map[int]string) error
	AddEnvironmentToAccessGroup(id int, environmentId int) error
	RemoveEnvironmentFromAccessGroup(id int, environmentId int) error
}

// StackClient manages edge stacks and regular (non-edge) compose/swarm stacks.
// Used by: stack.go
type StackClient interface {
	GetStacks() ([]models.Stack, error)
	GetStackFile(id int) (string, error)
	CreateStack(name string, file string, environmentGroupIds []int) (int, error)
	UpdateStack(id int, file string, environmentGroupIds []int) error
	GetRegularStacks() ([]models.RegularStack, error)
	InspectStack(id int) (models.RegularStack, error)
	DeleteStack(id int, opts models.DeleteStackOptions) error
	InspectStackFile(id int) (string, error)
	UpdateStackGit(id int, opts models.UpdateStackGitOptions) (models.RegularStack, error)
	RedeployStackGit(id int, opts models.RedeployStackGitOptions) (models.RegularStack, error)
	StartStack(id int, endpointID int) (models.RegularStack, error)
	StopStack(id int, endpointID int) (models.RegularStack, error)
	MigrateStack(id int, endpointID int, targetEndpointID int, name string) (models.RegularStack, error)
}

// TeamClient manages teams and their membership.
// Used by: team.go
type TeamClient interface {
	CreateTeam(name string) (int, error)
	GetTeam(id int) (models.Team, error)
	GetTeams() ([]models.Team, error)
	DeleteTeam(id int) error
	UpdateTeamName(id int, name string) error
	UpdateTeamMembers(id int, userIds []int) error
}

// UserClient manages user accounts.
// Used by: user.go
type UserClient interface {
	CreateUser(username, password, role string) (int, error)
	GetUser(id int) (models.User, error)
	GetUsers() ([]models.User, error)
	DeleteUser(id int) error
	UpdateUserRole(id int, role string) error
}

// SettingsClient manages Portainer server settings, public settings,
// and SSL configuration.
// Used by: settings.go, ssl.go
type SettingsClient interface {
	GetSettings() (models.PortainerSettings, error)
	UpdateSettings(settingsJSON map[string]any) error
	GetPublicSettings() (models.PublicSettings, error)
	GetSSLSettings() (models.SSLSettings, error)
	UpdateSSLSettings(cert, key string, httpEnabled *bool) error
}

// TemplateClient manages application templates and custom templates.
// Used by: app_template.go, custom_template.go
type TemplateClient interface {
	GetAppTemplates() ([]models.AppTemplate, error)
	GetAppTemplateFile(id int) (string, error)
	GetCustomTemplates() ([]models.CustomTemplate, error)
	GetCustomTemplate(id int) (models.CustomTemplate, error)
	GetCustomTemplateFile(id int) (string, error)
	CreateCustomTemplate(title, description, note, logo, fileContent string, platform, templateType int) (int, error)
	DeleteCustomTemplate(id int) error
}

// DockerClient provides Docker API proxy access and dashboard data.
// Used by: docker.go
type DockerClient interface {
	ProxyDockerRequest(opts models.DockerProxyRequestOptions) (*http.Response, error)
	GetDockerDashboard(environmentId int) (models.DockerDashboard, error)
}

// KubernetesClient provides Kubernetes API proxy, dashboard, namespace,
// and kubeconfig access.
// Used by: kubernetes.go
type KubernetesClient interface {
	ProxyKubernetesRequest(opts models.KubernetesProxyRequestOptions) (*http.Response, error)
	GetKubernetesDashboard(environmentId int) (models.KubernetesDashboard, error)
	GetKubernetesNamespaces(environmentId int) ([]models.KubernetesNamespace, error)
	GetKubernetesConfig(environmentId int) (any, error)
}

// RegistryClient manages container registries.
// Used by: registry.go
type RegistryClient interface {
	GetRegistries() ([]models.Registry, error)
	GetRegistry(id int) (models.Registry, error)
	CreateRegistry(name string, registryType int, url string, authentication bool, username string, password string, baseURL string) (int, error)
	UpdateRegistry(id int, name *string, url *string, authentication *bool, username *string, password *string, baseURL *string) error
	DeleteRegistry(id int) error
}

// BackupClient manages server backups and S3 backup/restore.
// Used by: backup.go
type BackupClient interface {
	GetBackupStatus() (models.BackupStatus, error)
	GetBackupS3Settings() (models.S3BackupSettings, error)
	CreateBackup(password string) error
	BackupToS3(settings models.S3BackupSettings) error
	RestoreFromS3(accessKeyID, bucketName, filename, password, region, s3CompatibleHost, secretAccessKey string) error
}

// WebhookClient manages webhooks for container services.
// Used by: webhook.go
type WebhookClient interface {
	GetWebhooks() ([]models.Webhook, error)
	CreateWebhook(resourceId string, endpointId int, webhookType int) (int, error)
	DeleteWebhook(id int) error
}

// EdgeClient manages edge compute jobs and update schedules.
// Used by: edge_job.go
type EdgeClient interface {
	GetEdgeJobs() ([]models.EdgeJob, error)
	GetEdgeJob(id int) (models.EdgeJob, error)
	GetEdgeJobFile(id int) (string, error)
	CreateEdgeJob(name, cronExpression, fileContent string, endpoints []int, edgeGroups []int, recurring bool) (int, error)
	DeleteEdgeJob(id int) error
	GetEdgeUpdateSchedules() ([]models.EdgeUpdateSchedule, error)
}

// HelmClient manages Helm repositories, chart search, and releases.
// Used by: helm.go
type HelmClient interface {
	GetHelmRepositories(userId int) (models.HelmRepositoryList, error)
	CreateHelmRepository(userId int, url string) (models.HelmRepository, error)
	DeleteHelmRepository(userId int, repositoryId int) error
	SearchHelmCharts(repo string, chart string) (string, error)
	InstallHelmChart(environmentId int, chart, name, namespace, repo, values, version string) (models.HelmReleaseDetails, error)
	GetHelmReleases(environmentId int, namespace, filter, selector string) ([]models.HelmRelease, error)
	DeleteHelmRelease(environmentId int, release, namespace string) error
	GetHelmReleaseHistory(environmentId int, name, namespace string) ([]models.HelmReleaseDetails, error)
}

// AuthClient manages user authentication.
// Used by: auth.go
type AuthClient interface {
	AuthenticateUser(username, password string) (models.AuthResponse, error)
	Logout() error
}

// SystemClient manages system-level queries: version, status, roles, and MOTD.
// Used by: system.go, role.go, motd.go, server.go (version check)
type SystemClient interface {
	GetVersion() (string, error)
	GetSystemStatus() (models.SystemStatus, error)
	GetRoles() ([]models.Role, error)
	GetMOTD() (models.MOTD, error)
}
