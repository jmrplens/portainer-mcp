package mcp

import (
	"net/http"

	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/mock"
)

// Mock Implementation Patterns:
//
// This file contains mock implementations of the PortainerClient interface.
// The following patterns are used throughout the mocks:
//
// 1. Methods returning (T, error):
//    - Uses m.Called() to record the method call and get mock behavior
//    - Includes nil check on first return value to avoid type assertion panics
//    - Example:
//      func (m *Mock) Method() (T, error) {
//          args := m.Called()
//          if args.Get(0) == nil {
//              return nil, args.Error(1)
//          }
//          return args.Get(0).(T), args.Error(1)
//      }
//
// 2. Methods returning only error:
//    - Uses m.Called() with any parameters
//    - Returns only the error value
//    - Example:
//      func (m *Mock) Method(param string) error {
//          args := m.Called(param)
//          return args.Error(0)
//      }
//
// Usage in Tests:
//   mock := new(MockPortainerClient)
//   mock.On("MethodName").Return(expectedValue, nil)
//   result, err := mock.MethodName()
//   mock.AssertExpectations(t)

// MockPortainerClient is a mock implementation of the PortainerClient interface
type MockPortainerClient struct {
	mock.Mock
}

// Tag methods

func (m *MockPortainerClient) GetEnvironmentTags() ([]models.EnvironmentTag, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.EnvironmentTag), args.Error(1)
}

func (m *MockPortainerClient) CreateEnvironmentTag(name string) (int, error) {
	args := m.Called(name)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) DeleteEnvironmentTag(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// Environment methods

func (m *MockPortainerClient) GetEnvironments() ([]models.Environment, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Environment), args.Error(1)
}

func (m *MockPortainerClient) GetEnvironment(id int) (models.Environment, error) {
	args := m.Called(id)
	return args.Get(0).(models.Environment), args.Error(1)
}

func (m *MockPortainerClient) DeleteEnvironment(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockPortainerClient) SnapshotEnvironment(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockPortainerClient) SnapshotAllEnvironments() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateEnvironmentTags(id int, tagIds []int) error {
	args := m.Called(id, tagIds)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateEnvironmentUserAccesses(id int, userAccesses map[int]string) error {
	args := m.Called(id, userAccesses)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateEnvironmentTeamAccesses(id int, teamAccesses map[int]string) error {
	args := m.Called(id, teamAccesses)
	return args.Error(0)
}

// Environment Group methods

func (m *MockPortainerClient) GetEnvironmentGroups() ([]models.Group, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *MockPortainerClient) CreateEnvironmentGroup(name string, environmentIds []int) (int, error) {
	args := m.Called(name, environmentIds)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) UpdateEnvironmentGroupName(id int, name string) error {
	args := m.Called(id, name)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateEnvironmentGroupEnvironments(id int, environmentIds []int) error {
	args := m.Called(id, environmentIds)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateEnvironmentGroupTags(id int, tagIds []int) error {
	args := m.Called(id, tagIds)
	return args.Error(0)
}

// Access Group methods

func (m *MockPortainerClient) GetAccessGroups() ([]models.AccessGroup, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AccessGroup), args.Error(1)
}

func (m *MockPortainerClient) CreateAccessGroup(name string, environmentIds []int) (int, error) {
	args := m.Called(name, environmentIds)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) UpdateAccessGroupName(id int, name string) error {
	args := m.Called(id, name)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateAccessGroupUserAccesses(id int, userAccesses map[int]string) error {
	args := m.Called(id, userAccesses)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateAccessGroupTeamAccesses(id int, teamAccesses map[int]string) error {
	args := m.Called(id, teamAccesses)
	return args.Error(0)
}

func (m *MockPortainerClient) AddEnvironmentToAccessGroup(id int, environmentId int) error {
	args := m.Called(id, environmentId)
	return args.Error(0)
}

func (m *MockPortainerClient) RemoveEnvironmentFromAccessGroup(id int, environmentId int) error {
	args := m.Called(id, environmentId)
	return args.Error(0)
}

// Stack methods

func (m *MockPortainerClient) GetStacks() ([]models.Stack, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Stack), args.Error(1)
}

func (m *MockPortainerClient) GetStackFile(id int) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerClient) CreateStack(name string, file string, environmentGroupIds []int) (int, error) {
	args := m.Called(name, file, environmentGroupIds)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) UpdateStack(id int, file string, environmentGroupIds []int) error {
	args := m.Called(id, file, environmentGroupIds)
	return args.Error(0)
}

func (m *MockPortainerClient) InspectStack(id int) (models.RegularStack, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return models.RegularStack{}, args.Error(1)
	}
	return args.Get(0).(models.RegularStack), args.Error(1)
}

func (m *MockPortainerClient) DeleteStack(id int, endpointID int, removeVolumes bool) error {
	args := m.Called(id, endpointID, removeVolumes)
	return args.Error(0)
}

func (m *MockPortainerClient) InspectStackFile(id int) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerClient) UpdateStackGit(id int, endpointID int, referenceName string, prune bool) (models.RegularStack, error) {
	args := m.Called(id, endpointID, referenceName, prune)
	if args.Get(0) == nil {
		return models.RegularStack{}, args.Error(1)
	}
	return args.Get(0).(models.RegularStack), args.Error(1)
}

func (m *MockPortainerClient) RedeployStackGit(id int, endpointID int, pullImage bool, prune bool) (models.RegularStack, error) {
	args := m.Called(id, endpointID, pullImage, prune)
	if args.Get(0) == nil {
		return models.RegularStack{}, args.Error(1)
	}
	return args.Get(0).(models.RegularStack), args.Error(1)
}

func (m *MockPortainerClient) StartStack(id int, endpointID int) (models.RegularStack, error) {
	args := m.Called(id, endpointID)
	if args.Get(0) == nil {
		return models.RegularStack{}, args.Error(1)
	}
	return args.Get(0).(models.RegularStack), args.Error(1)
}

func (m *MockPortainerClient) StopStack(id int, endpointID int) (models.RegularStack, error) {
	args := m.Called(id, endpointID)
	if args.Get(0) == nil {
		return models.RegularStack{}, args.Error(1)
	}
	return args.Get(0).(models.RegularStack), args.Error(1)
}

func (m *MockPortainerClient) MigrateStack(id int, endpointID int, targetEndpointID int, name string) (models.RegularStack, error) {
	args := m.Called(id, endpointID, targetEndpointID, name)
	if args.Get(0) == nil {
		return models.RegularStack{}, args.Error(1)
	}
	return args.Get(0).(models.RegularStack), args.Error(1)
}

// Team methods

func (m *MockPortainerClient) CreateTeam(name string) (int, error) {
	args := m.Called(name)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) GetTeam(id int) (models.Team, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return models.Team{}, args.Error(1)
	}
	return args.Get(0).(models.Team), args.Error(1)
}

func (m *MockPortainerClient) DeleteTeam(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockPortainerClient) GetTeams() ([]models.Team, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Team), args.Error(1)
}

func (m *MockPortainerClient) UpdateTeamName(id int, name string) error {
	args := m.Called(id, name)
	return args.Error(0)
}

func (m *MockPortainerClient) UpdateTeamMembers(id int, userIds []int) error {
	args := m.Called(id, userIds)
	return args.Error(0)
}

// User methods

func (m *MockPortainerClient) GetUsers() ([]models.User, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.User), args.Error(1)
}

func (m *MockPortainerClient) UpdateUserRole(id int, role string) error {
	args := m.Called(id, role)
	return args.Error(0)
}

func (m *MockPortainerClient) CreateUser(username, password, role string) (int, error) {
	args := m.Called(username, password, role)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) GetUser(id int) (models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return models.User{}, args.Error(1)
	}
	return args.Get(0).(models.User), args.Error(1)
}

func (m *MockPortainerClient) DeleteUser(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// System methods

func (m *MockPortainerClient) GetSystemStatus() (models.SystemStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.SystemStatus{}, args.Error(1)
	}
	return args.Get(0).(models.SystemStatus), args.Error(1)
}

// Settings methods

func (m *MockPortainerClient) GetSettings() (models.PortainerSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.PortainerSettings{}, args.Error(1)
	}
	return args.Get(0).(models.PortainerSettings), args.Error(1)
}

func (m *MockPortainerClient) UpdateSettings(settings map[string]interface{}) error {
	args := m.Called(settings)
	return args.Error(0)
}

func (m *MockPortainerClient) GetPublicSettings() (models.PublicSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.PublicSettings{}, args.Error(1)
	}
	return args.Get(0).(models.PublicSettings), args.Error(1)
}

func (m *MockPortainerClient) GetSSLSettings() (models.SSLSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.SSLSettings{}, args.Error(1)
	}
	return args.Get(0).(models.SSLSettings), args.Error(1)
}

func (m *MockPortainerClient) UpdateSSLSettings(cert, key string, httpEnabled *bool) error {
	args := m.Called(cert, key, httpEnabled)
	return args.Error(0)
}

func (m *MockPortainerClient) GetAppTemplates() ([]models.AppTemplate, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AppTemplate), args.Error(1)
}

func (m *MockPortainerClient) GetAppTemplateFile(id int) (string, error) {
	args := m.Called(id)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockPortainerClient) GetVersion() (string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.Get(0).(string), args.Error(1)
}

// Docker Proxy methods
func (m *MockPortainerClient) ProxyDockerRequest(opts models.DockerProxyRequestOptions) (*http.Response, error) {
	args := m.Called(opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockPortainerClient) GetDockerDashboard(environmentId int) (models.DockerDashboard, error) {
	args := m.Called(environmentId)
	return args.Get(0).(models.DockerDashboard), args.Error(1)
}

// Kubernetes Proxy methods
func (m *MockPortainerClient) ProxyKubernetesRequest(opts models.KubernetesProxyRequestOptions) (*http.Response, error) {
	args := m.Called(opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockPortainerClient) GetKubernetesDashboard(environmentId int) (models.KubernetesDashboard, error) {
	args := m.Called(environmentId)
	return args.Get(0).(models.KubernetesDashboard), args.Error(1)
}

func (m *MockPortainerClient) GetKubernetesNamespaces(environmentId int) ([]models.KubernetesNamespace, error) {
	args := m.Called(environmentId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.KubernetesNamespace), args.Error(1)
}

func (m *MockPortainerClient) GetKubernetesConfig(environmentId int) (interface{}, error) {
	args := m.Called(environmentId)
	return args.Get(0), args.Error(1)
}

// Custom Template methods

func (m *MockPortainerClient) GetCustomTemplates() ([]models.CustomTemplate, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CustomTemplate), args.Error(1)
}

func (m *MockPortainerClient) GetCustomTemplate(id int) (models.CustomTemplate, error) {
	args := m.Called(id)
	return args.Get(0).(models.CustomTemplate), args.Error(1)
}

func (m *MockPortainerClient) GetCustomTemplateFile(id int) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerClient) CreateCustomTemplate(title, description, note, logo, fileContent string, platform, templateType int) (int, error) {
	args := m.Called(title, description, note, logo, fileContent, platform, templateType)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) DeleteCustomTemplate(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// Webhook methods

func (m *MockPortainerClient) GetWebhooks() ([]models.Webhook, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Webhook), args.Error(1)
}

func (m *MockPortainerClient) CreateWebhook(resourceId string, endpointId int, webhookType int) (int, error) {
	args := m.Called(resourceId, endpointId, webhookType)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) DeleteWebhook(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// Registry methods

func (m *MockPortainerClient) GetRegistries() ([]models.Registry, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Registry), args.Error(1)
}

func (m *MockPortainerClient) GetRegistry(id int) (models.Registry, error) {
	args := m.Called(id)
	return args.Get(0).(models.Registry), args.Error(1)
}

func (m *MockPortainerClient) CreateRegistry(name string, registryType int, url string, authentication bool, username string, password string, baseURL string) (int, error) {
	args := m.Called(name, registryType, url, authentication, username, password, baseURL)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) UpdateRegistry(id int, name *string, url *string, authentication *bool, username *string, password *string, baseURL *string) error {
	args := m.Called(id, name, url, authentication, username, password, baseURL)
	return args.Error(0)
}

func (m *MockPortainerClient) DeleteRegistry(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// Backup methods

func (m *MockPortainerClient) GetBackupStatus() (models.BackupStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.BackupStatus{}, args.Error(1)
	}
	return args.Get(0).(models.BackupStatus), args.Error(1)
}

func (m *MockPortainerClient) GetBackupS3Settings() (models.S3BackupSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.S3BackupSettings{}, args.Error(1)
	}
	return args.Get(0).(models.S3BackupSettings), args.Error(1)
}

func (m *MockPortainerClient) CreateBackup(password string) error {
	args := m.Called(password)
	return args.Error(0)
}

func (m *MockPortainerClient) BackupToS3(settings models.S3BackupSettings) error {
	args := m.Called(settings)
	return args.Error(0)
}

func (m *MockPortainerClient) RestoreFromS3(accessKeyID, bucketName, filename, password, region, s3CompatibleHost, secretAccessKey string) error {
	args := m.Called(accessKeyID, bucketName, filename, password, region, s3CompatibleHost, secretAccessKey)
	return args.Error(0)
}

// Role methods

func (m *MockPortainerClient) GetRoles() ([]models.Role, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Role), args.Error(1)
}

// MOTD methods

func (m *MockPortainerClient) GetMOTD() (models.MOTD, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return models.MOTD{}, args.Error(1)
	}
	return args.Get(0).(models.MOTD), args.Error(1)
}

// Edge Job methods

func (m *MockPortainerClient) GetEdgeJobs() ([]models.EdgeJob, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.EdgeJob), args.Error(1)
}

func (m *MockPortainerClient) GetEdgeJob(id int) (models.EdgeJob, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return models.EdgeJob{}, args.Error(1)
	}
	return args.Get(0).(models.EdgeJob), args.Error(1)
}

func (m *MockPortainerClient) GetEdgeJobFile(id int) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerClient) CreateEdgeJob(name, cronExpression, fileContent string, endpoints []int, edgeGroups []int, recurring bool) (int, error) {
	args := m.Called(name, cronExpression, fileContent, endpoints, edgeGroups, recurring)
	return args.Int(0), args.Error(1)
}

func (m *MockPortainerClient) DeleteEdgeJob(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// Edge Update Schedule methods

func (m *MockPortainerClient) GetEdgeUpdateSchedules() ([]models.EdgeUpdateSchedule, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.EdgeUpdateSchedule), args.Error(1)
}

// Auth methods

func (m *MockPortainerClient) AuthenticateUser(username, password string) (models.AuthResponse, error) {
	args := m.Called(username, password)
	return args.Get(0).(models.AuthResponse), args.Error(1)
}

func (m *MockPortainerClient) Logout() error {
	args := m.Called()
	return args.Error(0)
}

// Helm methods

func (m *MockPortainerClient) GetHelmRepositories(userId int) (models.HelmRepositoryList, error) {
	args := m.Called(userId)
	return args.Get(0).(models.HelmRepositoryList), args.Error(1)
}

func (m *MockPortainerClient) CreateHelmRepository(userId int, url string) (models.HelmRepository, error) {
	args := m.Called(userId, url)
	return args.Get(0).(models.HelmRepository), args.Error(1)
}

func (m *MockPortainerClient) DeleteHelmRepository(userId int, repositoryId int) error {
	args := m.Called(userId, repositoryId)
	return args.Error(0)
}

func (m *MockPortainerClient) SearchHelmCharts(repo string, chart string) (string, error) {
	args := m.Called(repo, chart)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerClient) InstallHelmChart(environmentId int, chart, name, namespace, repo, values, version string) (models.HelmReleaseDetails, error) {
	args := m.Called(environmentId, chart, name, namespace, repo, values, version)
	return args.Get(0).(models.HelmReleaseDetails), args.Error(1)
}

func (m *MockPortainerClient) GetHelmReleases(environmentId int, namespace, filter, selector string) ([]models.HelmRelease, error) {
	args := m.Called(environmentId, namespace, filter, selector)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.HelmRelease), args.Error(1)
}

func (m *MockPortainerClient) DeleteHelmRelease(environmentId int, release, namespace string) error {
	args := m.Called(environmentId, release, namespace)
	return args.Error(0)
}

func (m *MockPortainerClient) GetHelmReleaseHistory(environmentId int, name, namespace string) ([]models.HelmReleaseDetails, error) {
	args := m.Called(environmentId, name, namespace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.HelmReleaseDetails), args.Error(1)
}
