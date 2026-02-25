package client

import (
	"net/http"

	"github.com/portainer/client-api-go/v2/client"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/mock"
)

// Mock Implementation Patterns:
//
// This file contains mock implementations of the PortainerAPIClient interface.
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
// 3. Methods with primitive return types:
//    - Uses type-specific getters (e.g., Int64, String)
//    - Example:
//      func (m *Mock) Method() (int64, error) {
//          args := m.Called()
//          return args.Get(0).(int64), args.Error(1)
//      }
//
// Usage in Tests:
//   mock := new(MockPortainerAPI)
//   mock.On("MethodName").Return(expectedValue, nil)
//   result, err := mock.MethodName()
//   mock.AssertExpectations(t)

// MockPortainerAPI is a mock of the PortainerAPIClient interface
type MockPortainerAPI struct {
	mock.Mock
}

// ListEdgeGroups mocks the ListEdgeGroups method
func (m *MockPortainerAPI) ListEdgeGroups() ([]*apimodels.EdgegroupsDecoratedEdgeGroup, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.EdgegroupsDecoratedEdgeGroup), args.Error(1)
}

// CreateEdgeGroup mocks the CreateEdgeGroup method
func (m *MockPortainerAPI) CreateEdgeGroup(name string, environmentIds []int64) (int64, error) {
	args := m.Called(name, environmentIds)
	return args.Get(0).(int64), args.Error(1)
}

// UpdateEdgeGroup mocks the UpdateEdgeGroup method
func (m *MockPortainerAPI) UpdateEdgeGroup(id int64, name *string, environmentIds *[]int64, tagIds *[]int64) error {
	args := m.Called(id, name, environmentIds, tagIds)
	return args.Error(0)
}

// ListEdgeStacks mocks the ListEdgeStacks method
func (m *MockPortainerAPI) ListEdgeStacks() ([]*apimodels.PortainereeEdgeStack, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainereeEdgeStack), args.Error(1)
}

// CreateEdgeStack mocks the CreateEdgeStack method
func (m *MockPortainerAPI) CreateEdgeStack(name string, file string, environmentGroupIds []int64) (int64, error) {
	args := m.Called(name, file, environmentGroupIds)
	return args.Get(0).(int64), args.Error(1)
}

// UpdateEdgeStack mocks the UpdateEdgeStack method
func (m *MockPortainerAPI) UpdateEdgeStack(id int64, file string, environmentGroupIds []int64) error {
	args := m.Called(id, file, environmentGroupIds)
	return args.Error(0)
}

// GetEdgeStackFile mocks the GetEdgeStackFile method
func (m *MockPortainerAPI) GetEdgeStackFile(id int64) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

// ListEndpointGroups mocks the ListEndpointGroups method
func (m *MockPortainerAPI) ListEndpointGroups() ([]*apimodels.PortainerEndpointGroup, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerEndpointGroup), args.Error(1)
}

// CreateEndpointGroup mocks the CreateEndpointGroup method
func (m *MockPortainerAPI) CreateEndpointGroup(name string, associatedEndpoints []int64) (int64, error) {
	args := m.Called(name, associatedEndpoints)
	return args.Get(0).(int64), args.Error(1)
}

// UpdateEndpointGroup mocks the UpdateEndpointGroup method
func (m *MockPortainerAPI) UpdateEndpointGroup(id int64, name *string, userAccesses *map[int64]string, teamAccesses *map[int64]string) error {
	args := m.Called(id, name, userAccesses, teamAccesses)
	return args.Error(0)
}

// AddEnvironmentToEndpointGroup mocks the AddEnvironmentToEndpointGroup method
func (m *MockPortainerAPI) AddEnvironmentToEndpointGroup(groupId int64, environmentId int64) error {
	args := m.Called(groupId, environmentId)
	return args.Error(0)
}

// RemoveEnvironmentFromEndpointGroup mocks the RemoveEnvironmentFromEndpointGroup method
func (m *MockPortainerAPI) RemoveEnvironmentFromEndpointGroup(groupId int64, environmentId int64) error {
	args := m.Called(groupId, environmentId)
	return args.Error(0)
}

// ListEndpoints mocks the ListEndpoints method
func (m *MockPortainerAPI) ListEndpoints() ([]*apimodels.PortainereeEndpoint, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainereeEndpoint), args.Error(1)
}

// GetEndpoint mocks the GetEndpoint method
func (m *MockPortainerAPI) GetEndpoint(id int64) (*apimodels.PortainereeEndpoint, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeEndpoint), args.Error(1)
}

// UpdateEndpoint mocks the UpdateEndpoint method
func (m *MockPortainerAPI) UpdateEndpoint(id int64, tagIds *[]int64, userAccesses *map[int64]string, teamAccesses *map[int64]string) error {
	args := m.Called(id, tagIds, userAccesses, teamAccesses)
	return args.Error(0)
}

// DeleteEndpoint mocks the DeleteEndpoint method
func (m *MockPortainerAPI) DeleteEndpoint(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// SnapshotEndpoint mocks the SnapshotEndpoint method
func (m *MockPortainerAPI) SnapshotEndpoint(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// SnapshotAllEndpoints mocks the SnapshotAllEndpoints method
func (m *MockPortainerAPI) SnapshotAllEndpoints() error {
	args := m.Called()
	return args.Error(0)
}

// GetSettings mocks the GetSettings method
func (m *MockPortainerAPI) GetSettings() (*apimodels.PortainereeSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeSettings), args.Error(1)
}

func (m *MockPortainerAPI) UpdateSettings(payload *apimodels.SettingsSettingsUpdatePayload) error {
	args := m.Called(payload)
	return args.Error(0)
}

func (m *MockPortainerAPI) GetPublicSettings() (*apimodels.SettingsPublicSettingsResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.SettingsPublicSettingsResponse), args.Error(1)
}

func (m *MockPortainerAPI) GetSSLSettings() (*apimodels.PortainereeSSLSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeSSLSettings), args.Error(1)
}

func (m *MockPortainerAPI) UpdateSSLSettings(payload *apimodels.SslSslUpdatePayload) error {
	args := m.Called(payload)
	return args.Error(0)
}

func (m *MockPortainerAPI) ListAppTemplates() ([]*apimodels.PortainerTemplate, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerTemplate), args.Error(1)
}

func (m *MockPortainerAPI) GetAppTemplateFile(id int64) (string, error) {
	args := m.Called(id)
	return args.Get(0).(string), args.Error(1)
}

// ListTags mocks the ListTags method
func (m *MockPortainerAPI) ListTags() ([]*apimodels.PortainerTag, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerTag), args.Error(1)
}

// CreateTag mocks the CreateTag method
func (m *MockPortainerAPI) CreateTag(name string) (int64, error) {
	args := m.Called(name)
	return args.Get(0).(int64), args.Error(1)
}

// DeleteTag mocks the DeleteTag method
func (m *MockPortainerAPI) DeleteTag(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// ListTeams mocks the ListTeams method
func (m *MockPortainerAPI) ListTeams() ([]*apimodels.PortainerTeam, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerTeam), args.Error(1)
}

// ListTeamMemberships mocks the ListTeamMemberships method
func (m *MockPortainerAPI) ListTeamMemberships() ([]*apimodels.PortainerTeamMembership, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerTeamMembership), args.Error(1)
}

// CreateTeam mocks the CreateTeam method
func (m *MockPortainerAPI) CreateTeam(name string) (int64, error) {
	args := m.Called(name)
	return args.Get(0).(int64), args.Error(1)
}

// GetTeam mocks the GetTeam method
func (m *MockPortainerAPI) GetTeam(id int64) (*apimodels.PortainerTeam, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainerTeam), args.Error(1)
}

// DeleteTeam mocks the DeleteTeam method
func (m *MockPortainerAPI) DeleteTeam(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// UpdateTeamName mocks the UpdateTeamName method
func (m *MockPortainerAPI) UpdateTeamName(id int, name string) error {
	args := m.Called(id, name)
	return args.Error(0)
}

// DeleteTeamMembership mocks the DeleteTeamMembership method
func (m *MockPortainerAPI) DeleteTeamMembership(id int) error {
	args := m.Called(id)
	return args.Error(0)
}

// CreateTeamMembership mocks the CreateTeamMembership method
func (m *MockPortainerAPI) CreateTeamMembership(teamId int, userId int) error {
	args := m.Called(teamId, userId)
	return args.Error(0)
}

// ListUsers mocks the ListUsers method
func (m *MockPortainerAPI) ListUsers() ([]*apimodels.PortainereeUser, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainereeUser), args.Error(1)
}

// UpdateUserRole mocks the UpdateUserRole method
func (m *MockPortainerAPI) UpdateUserRole(id int, role int64) error {
	args := m.Called(id, role)
	return args.Error(0)
}

// CreateUser mocks the CreateUser method
func (m *MockPortainerAPI) CreateUser(username, password string, role int64) (int64, error) {
	args := m.Called(username, password, role)
	return args.Get(0).(int64), args.Error(1)
}

// GetUser mocks the GetUser method
func (m *MockPortainerAPI) GetUser(id int) (*apimodels.PortainereeUser, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeUser), args.Error(1)
}

// DeleteUser mocks the DeleteUser method
func (m *MockPortainerAPI) DeleteUser(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// GetSystemStatus mocks the GetSystemStatus method
func (m *MockPortainerAPI) GetSystemStatus() (*apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus), args.Error(1)
}

// GetVersion mocks the GetVersion method
func (m *MockPortainerAPI) GetVersion() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// ProxyDockerRequest mocks the ProxyDockerRequest method
func (m *MockPortainerAPI) ProxyDockerRequest(environmentId int, opts client.ProxyRequestOptions) (*http.Response, error) {
	args := m.Called(environmentId, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// ProxyKubernetesRequest mocks the ProxyKubernetesRequest method
func (m *MockPortainerAPI) ProxyKubernetesRequest(environmentId int, opts client.ProxyRequestOptions) (*http.Response, error) {
	args := m.Called(environmentId, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// ListCustomTemplates mocks the ListCustomTemplates method
func (m *MockPortainerAPI) ListCustomTemplates() ([]*apimodels.PortainereeCustomTemplate, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainereeCustomTemplate), args.Error(1)
}

// GetCustomTemplate mocks the GetCustomTemplate method
func (m *MockPortainerAPI) GetCustomTemplate(id int64) (*apimodels.PortainereeCustomTemplate, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeCustomTemplate), args.Error(1)
}

// GetCustomTemplateFile mocks the GetCustomTemplateFile method
func (m *MockPortainerAPI) GetCustomTemplateFile(id int64) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

// CreateCustomTemplate mocks the CreateCustomTemplate method
func (m *MockPortainerAPI) CreateCustomTemplate(payload *apimodels.CustomtemplatesCustomTemplateFromFileContentPayload) (*apimodels.PortainereeCustomTemplate, error) {
	args := m.Called(payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeCustomTemplate), args.Error(1)
}

// DeleteCustomTemplate mocks the DeleteCustomTemplate method
func (m *MockPortainerAPI) DeleteCustomTemplate(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// ListWebhooks mocks the ListWebhooks method
func (m *MockPortainerAPI) ListWebhooks() ([]*apimodels.PortainerWebhook, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerWebhook), args.Error(1)
}

// CreateWebhook mocks the CreateWebhook method
func (m *MockPortainerAPI) CreateWebhook(resourceId string, endpointId int64, webhookType int64) (int64, error) {
	args := m.Called(resourceId, endpointId, webhookType)
	return args.Get(0).(int64), args.Error(1)
}

// DeleteWebhook mocks the DeleteWebhook method
func (m *MockPortainerAPI) DeleteWebhook(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// ListRegistries mocks the ListRegistries method
func (m *MockPortainerAPI) ListRegistries() ([]*apimodels.PortainereeRegistry, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainereeRegistry), args.Error(1)
}

// GetRegistryByID mocks the GetRegistryByID method
func (m *MockPortainerAPI) GetRegistryByID(id int64) (*apimodels.PortainereeRegistry, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeRegistry), args.Error(1)
}

// CreateRegistry mocks the CreateRegistry method
func (m *MockPortainerAPI) CreateRegistry(body *apimodels.RegistriesRegistryCreatePayload) (int64, error) {
	args := m.Called(body)
	return args.Get(0).(int64), args.Error(1)
}

// UpdateRegistry mocks the UpdateRegistry method
func (m *MockPortainerAPI) UpdateRegistry(id int64, body *apimodels.RegistriesRegistryUpdatePayload) error {
	args := m.Called(id, body)
	return args.Error(0)
}

// DeleteRegistry mocks the DeleteRegistry method
func (m *MockPortainerAPI) DeleteRegistry(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// GetBackupStatus mocks the GetBackupStatus method
func (m *MockPortainerAPI) GetBackupStatus() (*apimodels.BackupBackupStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.BackupBackupStatus), args.Error(1)
}

// GetBackupSettings mocks the GetBackupSettings method
func (m *MockPortainerAPI) GetBackupSettings() (*apimodels.PortainereeS3BackupSettings, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeS3BackupSettings), args.Error(1)
}

// CreateBackup mocks the CreateBackup method
func (m *MockPortainerAPI) CreateBackup(password string) error {
	args := m.Called(password)
	return args.Error(0)
}

// BackupToS3 mocks the BackupToS3 method
func (m *MockPortainerAPI) BackupToS3(body *apimodels.BackupS3BackupPayload) error {
	args := m.Called(body)
	return args.Error(0)
}

// RestoreFromS3 mocks the RestoreFromS3 method
func (m *MockPortainerAPI) RestoreFromS3(body *apimodels.BackupRestoreS3Settings) error {
	args := m.Called(body)
	return args.Error(0)
}

// ListRoles mocks the ListRoles method
func (m *MockPortainerAPI) ListRoles() ([]*apimodels.PortainereeRole, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainereeRole), args.Error(1)
}

// GetMOTD mocks the GetMOTD method
func (m *MockPortainerAPI) GetMOTD() (*apimodels.MotdMotdResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.MotdMotdResponse), args.Error(1)
}

// ListEdgeJobs mocks the ListEdgeJobs method
func (m *MockPortainerAPI) ListEdgeJobs() ([]*apimodels.PortainerEdgeJob, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerEdgeJob), args.Error(1)
}

// GetEdgeJob mocks the GetEdgeJob method
func (m *MockPortainerAPI) GetEdgeJob(id int64) (*apimodels.PortainerEdgeJob, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainerEdgeJob), args.Error(1)
}

// GetEdgeJobFile mocks the GetEdgeJobFile method
func (m *MockPortainerAPI) GetEdgeJobFile(id int64) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

// CreateEdgeJob mocks the CreateEdgeJob method
func (m *MockPortainerAPI) CreateEdgeJob(payload *apimodels.EdgejobsEdgeJobCreateFromFileContentPayload) (int64, error) {
	args := m.Called(payload)
	return int64(args.Int(0)), args.Error(1)
}

// DeleteEdgeJob mocks the DeleteEdgeJob method
func (m *MockPortainerAPI) DeleteEdgeJob(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}

// ListEdgeUpdateSchedules mocks the ListEdgeUpdateSchedules method
func (m *MockPortainerAPI) ListEdgeUpdateSchedules() ([]*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule), args.Error(1)
}

// AuthenticateUser mocks the AuthenticateUser method
func (m *MockPortainerAPI) AuthenticateUser(username, password string) (*apimodels.AuthAuthenticateResponse, error) {
	args := m.Called(username, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.AuthAuthenticateResponse), args.Error(1)
}

// Logout mocks the Logout method
func (m *MockPortainerAPI) Logout() error {
	args := m.Called()
	return args.Error(0)
}

// Helm methods

func (m *MockPortainerAPI) ListHelmRepositories(userId int64) (*apimodels.UsersHelmUserRepositoryResponse, error) {
	args := m.Called(userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.UsersHelmUserRepositoryResponse), args.Error(1)
}

func (m *MockPortainerAPI) CreateHelmRepository(userId int64, url string) (*apimodels.PortainerHelmUserRepository, error) {
	args := m.Called(userId, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainerHelmUserRepository), args.Error(1)
}

func (m *MockPortainerAPI) DeleteHelmRepository(userId int64, repositoryId int64) error {
	args := m.Called(userId, repositoryId)
	return args.Error(0)
}

func (m *MockPortainerAPI) SearchHelmCharts(repo string, chart *string) (string, error) {
	args := m.Called(repo, chart)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerAPI) InstallHelmChart(environmentId int64, payload *apimodels.HelmInstallChartPayload) (*apimodels.ReleaseRelease, error) {
	args := m.Called(environmentId, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.ReleaseRelease), args.Error(1)
}

func (m *MockPortainerAPI) ListHelmReleases(environmentId int64, namespace *string, filter *string, selector *string) ([]*apimodels.ReleaseReleaseElement, error) {
	args := m.Called(environmentId, namespace, filter, selector)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.ReleaseReleaseElement), args.Error(1)
}

func (m *MockPortainerAPI) DeleteHelmRelease(environmentId int64, release string, namespace *string) error {
	args := m.Called(environmentId, release, namespace)
	return args.Error(0)
}

func (m *MockPortainerAPI) GetHelmReleaseHistory(environmentId int64, name string, namespace *string) ([]*apimodels.ReleaseRelease, error) {
	args := m.Called(environmentId, name, namespace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.ReleaseRelease), args.Error(1)
}

func (m *MockPortainerAPI) GetDockerDashboard(environmentId int64) (*apimodels.DockerDashboardResponse, error) {
	args := m.Called(environmentId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.DockerDashboardResponse), args.Error(1)
}

func (m *MockPortainerAPI) GetKubernetesConfig(environmentId int64) (interface{}, error) {
	args := m.Called(environmentId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func (m *MockPortainerAPI) GetKubernetesDashboard(environmentId int64) ([]*apimodels.ModelsK8sDashboard, error) {
	args := m.Called(environmentId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.ModelsK8sDashboard), args.Error(1)
}

func (m *MockPortainerAPI) GetKubernetesNamespaces(environmentId int64) ([]*apimodels.PortainerK8sNamespaceInfo, error) {
	args := m.Called(environmentId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*apimodels.PortainerK8sNamespaceInfo), args.Error(1)
}

func (m *MockPortainerAPI) StackInspect(id int64) (*apimodels.PortainereeStack, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeStack), args.Error(1)
}

func (m *MockPortainerAPI) StackDelete(id int64, endpointID int64, removeVolumes bool) error {
	args := m.Called(id, endpointID, removeVolumes)
	return args.Error(0)
}

func (m *MockPortainerAPI) StackFileInspect(id int64) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func (m *MockPortainerAPI) StackUpdateGit(id int64, endpointID int64, body *apimodels.StacksStackGitUpdatePayload) (*apimodels.PortainereeStack, error) {
	args := m.Called(id, endpointID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeStack), args.Error(1)
}

func (m *MockPortainerAPI) StackGitRedeploy(id int64, endpointID int64, body *apimodels.StacksStackGitRedployPayload) (*apimodels.PortainereeStack, error) {
	args := m.Called(id, endpointID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeStack), args.Error(1)
}

func (m *MockPortainerAPI) StackStart(id int64, endpointID int64) (*apimodels.PortainereeStack, error) {
	args := m.Called(id, endpointID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeStack), args.Error(1)
}

func (m *MockPortainerAPI) StackStop(id int64, endpointID int64) (*apimodels.PortainereeStack, error) {
	args := m.Called(id, endpointID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeStack), args.Error(1)
}

func (m *MockPortainerAPI) StackMigrate(id int64, endpointID int64, body *apimodels.StacksStackMigratePayload) (*apimodels.PortainereeStack, error) {
	args := m.Called(id, endpointID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*apimodels.PortainereeStack), args.Error(1)
}
