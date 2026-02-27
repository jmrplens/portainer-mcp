package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	sdkclient "github.com/portainer/client-api-go/v2/client"
	swaggerclient "github.com/portainer/client-api-go/v2/pkg/client"
	"github.com/portainer/client-api-go/v2/pkg/client/auth"
	"github.com/portainer/client-api-go/v2/pkg/client/backup"
	"github.com/portainer/client-api-go/v2/pkg/client/custom_templates"
	"github.com/portainer/client-api-go/v2/pkg/client/edge_jobs"
	"github.com/portainer/client-api-go/v2/pkg/client/edge_update_schedules"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoints"
	"github.com/portainer/client-api-go/v2/pkg/client/helm"
	"github.com/portainer/client-api-go/v2/pkg/client/kubernetes"
	"github.com/portainer/client-api-go/v2/pkg/client/registries"
	"github.com/portainer/client-api-go/v2/pkg/client/roles"
	"github.com/portainer/client-api-go/v2/pkg/client/settings"
	"github.com/portainer/client-api-go/v2/pkg/client/ssl"
	"github.com/portainer/client-api-go/v2/pkg/client/stacks"
	"github.com/portainer/client-api-go/v2/pkg/client/tags"
	"github.com/portainer/client-api-go/v2/pkg/client/teams"
	"github.com/portainer/client-api-go/v2/pkg/client/templates"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
	"github.com/portainer/client-api-go/v2/pkg/client/webhooks"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests to the Portainer API.
	defaultHTTPTimeout = 30 * time.Second
)

// portainerAPIAdapter wraps the SDK PortainerClient and adds methods
// that are available in the Swagger-generated client but not exposed
// by the SDK's high-level client (e.g., delete operations).
type portainerAPIAdapter struct {
	*sdkclient.PortainerClient
	swagger       *swaggerclient.PortainerClientAPI
	httpTransport *httptransport.Runtime
}

// newHTTPTransport creates a configured http.Transport with TLS settings.
func newHTTPTransport(skipTLSVerify bool) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify,
		},
	}
}

// detectSchemes returns the appropriate HTTP schemes based on the host string.
// If the host starts with "http://", it returns ["http"]. Otherwise, it defaults to ["https"].
func detectSchemes(host string) []string {
	if strings.HasPrefix(strings.ToLower(host), "http://") {
		return []string{"http"}
	}
	return []string{"https"}
}

// newPortainerAPIAdapter creates a new adapter that embeds the SDK high-level
// client and also holds a reference to the low-level Swagger client for
// operations not exposed by the SDK.
func newPortainerAPIAdapter(host, apiKey string, skipTLSVerify bool) *portainerAPIAdapter {
	sdkCli := sdkclient.NewPortainerClient(host, apiKey, sdkclient.WithSkipTLSVerify(skipTLSVerify))

	httpClient := &http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: newHTTPTransport(skipTLSVerify),
	}
	schemes := detectSchemes(host)
	transport := httptransport.NewWithClient(host, "/api", schemes, httpClient)
	apiKeyAuth := runtime.ClientAuthInfoWriterFunc(func(r runtime.ClientRequest, _ strfmt.Registry) error {
		return r.SetHeaderParam("x-api-key", apiKey)
	})
	transport.DefaultAuthentication = apiKeyAuth

	return &portainerAPIAdapter{
		PortainerClient: sdkCli,
		swagger:         swaggerclient.New(transport, nil),
		httpTransport:   transport,
	}
}

// DeleteTag deletes a tag by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteTag(id int64) error {
	params := tags.NewTagDeleteParams().WithID(id)
	_, err := a.swagger.Tags.TagDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// DeleteTeam deletes a team by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteTeam(id int64) error {
	params := teams.NewTeamDeleteParams().WithID(id)
	_, err := a.swagger.Teams.TeamDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}

// DeleteUser deletes a user by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteUser(id int64) error {
	params := users.NewUserDeleteParams().WithID(id)
	_, err := a.swagger.Users.UserDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// DeleteEndpoint deletes an endpoint by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteEndpoint(id int64) error {
	params := endpoints.NewEndpointDeleteParams().WithID(id)
	_, err := a.swagger.Endpoints.EndpointDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete endpoint: %w", err)
	}
	return nil
}

// SnapshotEndpoint triggers a snapshot for a single endpoint.
func (a *portainerAPIAdapter) SnapshotEndpoint(id int64) error {
	params := endpoints.NewEndpointSnapshotParams().WithID(id)
	_, err := a.swagger.Endpoints.EndpointSnapshot(params, nil)
	if err != nil {
		return fmt.Errorf("failed to snapshot endpoint: %w", err)
	}
	return nil
}

// SnapshotAllEndpoints triggers a snapshot for all endpoints.
func (a *portainerAPIAdapter) SnapshotAllEndpoints() error {
	params := endpoints.NewEndpointSnapshotsParams()
	_, err := a.swagger.Endpoints.EndpointSnapshots(params, nil)
	if err != nil {
		return fmt.Errorf("failed to snapshot all endpoints: %w", err)
	}
	return nil
}

// ListWebhooks retrieves all webhooks using the low-level Swagger client.
func (a *portainerAPIAdapter) ListWebhooks() ([]*apimodels.PortainerWebhook, error) {
	params := webhooks.NewGetWebhooksParams()
	resp, err := a.swagger.Webhooks.GetWebhooks(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	return resp.Payload, nil
}

// CreateWebhook creates a new webhook using the low-level Swagger client.
func (a *portainerAPIAdapter) CreateWebhook(resourceId string, endpointId int64, webhookType int64) (int64, error) {
	payload := &apimodels.WebhooksWebhookCreatePayload{
		ResourceID:  resourceId,
		EndpointID:  endpointId,
		WebhookType: webhookType,
	}
	params := webhooks.NewPostWebhooksParams().WithBody(payload)
	resp, err := a.swagger.Webhooks.PostWebhooks(params, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create webhook: %w", err)
	}
	return resp.Payload.ID, nil
}

// DeleteWebhook deletes a webhook by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteWebhook(id int64) error {
	params := webhooks.NewDeleteWebhooksIDParams().WithID(id)
	_, err := a.swagger.Webhooks.DeleteWebhooksID(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}

// ListCustomTemplates lists all custom templates.
func (a *portainerAPIAdapter) ListCustomTemplates() ([]*apimodels.PortainereeCustomTemplate, error) {
	params := custom_templates.NewCustomTemplateListParams()
	resp, err := a.swagger.CustomTemplates.CustomTemplateList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list custom templates: %w", err)
	}
	return resp.Payload, nil
}

// GetCustomTemplate retrieves a custom template by ID.
func (a *portainerAPIAdapter) GetCustomTemplate(id int64) (*apimodels.PortainereeCustomTemplate, error) {
	params := custom_templates.NewCustomTemplateInspectParams().WithID(id)
	resp, err := a.swagger.CustomTemplates.CustomTemplateInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom template: %w", err)
	}
	return resp.Payload, nil
}

// GetCustomTemplateFile retrieves the file content of a custom template.
func (a *portainerAPIAdapter) GetCustomTemplateFile(id int64) (string, error) {
	params := custom_templates.NewCustomTemplateFileParams().WithID(id)
	resp, err := a.swagger.CustomTemplates.CustomTemplateFile(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get custom template file: %w", err)
	}
	return resp.Payload.FileContent, nil
}

// CreateCustomTemplate creates a new custom template from file content.
func (a *portainerAPIAdapter) CreateCustomTemplate(payload *apimodels.CustomtemplatesCustomTemplateFromFileContentPayload) (*apimodels.PortainereeCustomTemplate, error) {
	params := custom_templates.NewCustomTemplateCreateStringParams().WithBody(payload)
	resp, err := a.swagger.CustomTemplates.CustomTemplateCreateString(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom template: %w", err)
	}
	return resp.Payload, nil
}

// DeleteCustomTemplate deletes a custom template by ID.
func (a *portainerAPIAdapter) DeleteCustomTemplate(id int64) error {
	params := custom_templates.NewCustomTemplateDeleteParams().WithID(id)
	_, err := a.swagger.CustomTemplates.CustomTemplateDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete custom template: %w", err)
	}
	return nil
}

// ListRegistries lists all registries.
func (a *portainerAPIAdapter) ListRegistries() ([]*apimodels.PortainereeRegistry, error) {
	params := registries.NewRegistryListParams()
	resp, err := a.swagger.Registries.RegistryList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list registries: %w", err)
	}
	return resp.Payload, nil
}

// GetRegistryByID retrieves a registry by ID.
func (a *portainerAPIAdapter) GetRegistryByID(id int64) (*apimodels.PortainereeRegistry, error) {
	params := registries.NewRegistryInspectParams().WithID(id)
	resp, err := a.swagger.Registries.RegistryInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry: %w", err)
	}
	return resp.Payload, nil
}

// CreateRegistry creates a new registry.
func (a *portainerAPIAdapter) CreateRegistry(body *apimodels.RegistriesRegistryCreatePayload) (int64, error) {
	params := registries.NewRegistryCreateParams().WithBody(body)
	resp, err := a.swagger.Registries.RegistryCreate(params, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create registry: %w", err)
	}
	return resp.Payload.ID, nil
}

// UpdateRegistry updates an existing registry.
func (a *portainerAPIAdapter) UpdateRegistry(id int64, body *apimodels.RegistriesRegistryUpdatePayload) error {
	params := registries.NewRegistryUpdateParams().WithID(id).WithBody(body)
	_, err := a.swagger.Registries.RegistryUpdate(params, nil)
	if err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}
	return nil
}

// DeleteRegistry deletes a registry by ID.
func (a *portainerAPIAdapter) DeleteRegistry(id int64) error {
	params := registries.NewRegistryDeleteParams().WithID(id)
	_, err := a.swagger.Registries.RegistryDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete registry: %w", err)
	}
	return nil
}

// GetBackupStatus retrieves the status of the last backup.
func (a *portainerAPIAdapter) GetBackupStatus() (*apimodels.BackupBackupStatus, error) {
	params := backup.NewBackupStatusFetchParams()
	resp, err := a.swagger.Backup.BackupStatusFetch(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup status: %w", err)
	}
	return resp.Payload, nil
}

// GetBackupSettings retrieves the S3 backup settings.
func (a *portainerAPIAdapter) GetBackupSettings() (*apimodels.PortainereeS3BackupSettings, error) {
	params := backup.NewBackupSettingsFetchParams()
	resp, err := a.swagger.Backup.BackupSettingsFetch(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup settings: %w", err)
	}
	return resp.Payload, nil
}

// CreateBackup triggers a backup with an optional password.
func (a *portainerAPIAdapter) CreateBackup(password string) error {
	body := &apimodels.BackupBackupPayload{
		Password: password,
	}
	params := backup.NewBackupParams().WithBody(body)
	_, err := a.swagger.Backup.Backup(params, nil)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return nil
}

// BackupToS3 triggers a backup to S3.
func (a *portainerAPIAdapter) BackupToS3(body *apimodels.BackupS3BackupPayload) error {
	params := backup.NewBackupToS3Params().WithBody(body)
	_, err := a.swagger.Backup.BackupToS3(params, nil)
	if err != nil {
		return fmt.Errorf("failed to backup to S3: %w", err)
	}
	return nil
}

// RestoreFromS3 triggers a restore from S3.
func (a *portainerAPIAdapter) RestoreFromS3(body *apimodels.BackupRestoreS3Settings) error {
	params := backup.NewRestoreFromS3Params().WithBody(body)
	_, err := a.swagger.Backup.RestoreFromS3(params)
	if err != nil {
		return fmt.Errorf("failed to restore from S3: %w", err)
	}
	return nil
}

// ListRoles lists all roles.
func (a *portainerAPIAdapter) ListRoles() ([]*apimodels.PortainereeRole, error) {
	params := roles.NewRoleListParams()
	resp, err := a.swagger.Roles.RoleList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	return resp.Payload, nil
}

// GetMOTD retrieves the message of the day.
func (a *portainerAPIAdapter) GetMOTD() (map[string]any, error) {
	// Use raw HTTP to avoid SDK Hash type mismatch
	// (SDK defines Hash as []int64, but newer API versions return a string).
	op := &runtime.ClientOperation{
		ID:                 "MOTD",
		Method:             "GET",
		PathPattern:        "/motd",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params: runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, reg strfmt.Registry) error {
			return nil
		}),
		AuthInfo: a.httpTransport.DefaultAuthentication,
		Reader: runtime.ClientResponseReaderFunc(func(resp runtime.ClientResponse, consumer runtime.Consumer) (any, error) {
			var result map[string]any
			if err := consumer.Consume(resp.Body(), &result); err != nil {
				return nil, err
			}
			return result, nil
		}),
	}
	res, err := a.httpTransport.Submit(op)
	if err != nil {
		return nil, fmt.Errorf("failed to get MOTD: %w", err)
	}
	return res.(map[string]any), nil
}

// ListEdgeJobs lists all edge jobs.
func (a *portainerAPIAdapter) ListEdgeJobs() ([]*apimodels.PortainerEdgeJob, error) {
	params := edge_jobs.NewEdgeJobListParams()
	resp, err := a.swagger.EdgeJobs.EdgeJobList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge jobs: %w", err)
	}
	return resp.Payload, nil
}

// GetEdgeJob retrieves an edge job by ID.
func (a *portainerAPIAdapter) GetEdgeJob(id int64) (*apimodels.PortainerEdgeJob, error) {
	params := edge_jobs.NewEdgeJobInspectParams().WithID(id)
	resp, err := a.swagger.EdgeJobs.EdgeJobInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge job: %w", err)
	}
	return resp.Payload, nil
}

// GetEdgeJobFile retrieves the file content of an edge job.
func (a *portainerAPIAdapter) GetEdgeJobFile(id int64) (string, error) {
	params := edge_jobs.NewEdgeJobFileParams().WithID(id)
	resp, err := a.swagger.EdgeJobs.EdgeJobFile(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get edge job file: %w", err)
	}
	return resp.Payload.FileContent, nil
}

// CreateEdgeJob creates a new edge job from file content.
func (a *portainerAPIAdapter) CreateEdgeJob(payload *apimodels.EdgejobsEdgeJobCreateFromFileContentPayload) (int64, error) {
	params := edge_jobs.NewEdgeJobCreateStringParams().WithBody(payload)
	resp, err := a.swagger.EdgeJobs.EdgeJobCreateString(params, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create edge job: %w", err)
	}
	return resp.Payload.ID, nil
}

// DeleteEdgeJob deletes an edge job by ID.
func (a *portainerAPIAdapter) DeleteEdgeJob(id int64) error {
	params := edge_jobs.NewEdgeJobDeleteParams().WithID(id)
	_, err := a.swagger.EdgeJobs.EdgeJobDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete edge job: %w", err)
	}
	return nil
}

// UpdateSettings updates the Portainer settings using the provided payload.
func (a *portainerAPIAdapter) UpdateSettings(payload *apimodels.SettingsSettingsUpdatePayload) error {
	params := settings.NewSettingsUpdateParams().WithBody(payload)
	_, err := a.swagger.Settings.SettingsUpdate(params, nil)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}

// GetPublicSettings retrieves the public settings from the Portainer server.
func (a *portainerAPIAdapter) GetPublicSettings() (*apimodels.SettingsPublicSettingsResponse, error) {
	params := settings.NewSettingsPublicParams()
	resp, err := a.swagger.Settings.SettingsPublic(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get public settings: %w", err)
	}
	return resp.Payload, nil
}

// GetSSLSettings retrieves the SSL settings from the Portainer server.
func (a *portainerAPIAdapter) GetSSLSettings() (*apimodels.PortainereeSSLSettings, error) {
	params := ssl.NewSSLInspectParams()
	resp, err := a.swagger.Ssl.SSLInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSL settings: %w", err)
	}
	return resp.Payload, nil
}

// UpdateSSLSettings updates the SSL settings.
func (a *portainerAPIAdapter) UpdateSSLSettings(payload *apimodels.SslSslUpdatePayload) error {
	params := ssl.NewSSLUpdateParams().WithBody(payload)
	_, err := a.swagger.Ssl.SSLUpdate(params, nil)
	if err != nil {
		return fmt.Errorf("failed to update SSL settings: %w", err)
	}
	return nil
}

// ListAppTemplates lists all application templates.
func (a *portainerAPIAdapter) ListAppTemplates() ([]*apimodels.PortainerTemplate, error) {
	params := templates.NewTemplateListParams()
	resp, err := a.swagger.Templates.TemplateList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list app templates: %w", err)
	}
	return resp.Payload.Templates, nil
}

// GetAppTemplateFile retrieves the file content of an application template.
func (a *portainerAPIAdapter) GetAppTemplateFile(id int64) (string, error) {
	params := templates.NewTemplateFileParams().WithID(id)
	resp, err := a.swagger.Templates.TemplateFile(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get app template file: %w", err)
	}
	return resp.Payload.FileContent, nil
}

// ListEdgeUpdateSchedules lists all edge update schedules.
func (a *portainerAPIAdapter) ListEdgeUpdateSchedules() ([]*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule, error) {
	params := edge_update_schedules.NewEdgeUpdateScheduleListParams()
	resp, err := a.swagger.EdgeUpdateSchedules.EdgeUpdateScheduleList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge update schedules: %w", err)
	}
	return resp.Payload, nil
}

// AuthenticateUser authenticates a user using the Swagger client.
func (a *portainerAPIAdapter) AuthenticateUser(username, password string) (*apimodels.AuthAuthenticateResponse, error) {
	params := auth.NewAuthenticateUserParams()
	params.Body = &apimodels.AuthAuthenticatePayload{
		Username: &username,
		Password: &password,
	}
	resp, err := a.swagger.Auth.AuthenticateUser(params)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate user: %w", err)
	}
	return resp.Payload, nil
}

// Logout logs out the current user session.
func (a *portainerAPIAdapter) Logout() error {
	params := auth.NewLogoutParams()
	_, err := a.swagger.Auth.Logout(params, nil)
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	return nil
}

// ListHelmRepositories lists helm repositories for a user.
func (a *portainerAPIAdapter) ListHelmRepositories(userId int64) (*apimodels.UsersHelmUserRepositoryResponse, error) {
	params := helm.NewHelmUserRepositoriesListParams().WithID(userId)
	resp, err := a.swagger.Helm.HelmUserRepositoriesList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list helm repositories: %w", err)
	}
	return resp.Payload, nil
}

// CreateHelmRepository creates a helm repository for a user.
func (a *portainerAPIAdapter) CreateHelmRepository(userId int64, url string) (*apimodels.PortainerHelmUserRepository, error) {
	params := helm.NewHelmUserRepositoryCreateParams().WithID(userId).WithPayload(&apimodels.UsersAddHelmRepoURLPayload{URL: url})
	resp, err := a.swagger.Helm.HelmUserRepositoryCreate(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create helm repository: %w", err)
	}
	return resp.Payload, nil
}

// DeleteHelmRepository deletes a helm repository for a user.
func (a *portainerAPIAdapter) DeleteHelmRepository(userId int64, repositoryId int64) error {
	params := helm.NewHelmUserRepositoryDeleteParams().WithID(userId).WithRepositoryID(repositoryId)
	_, err := a.swagger.Helm.HelmUserRepositoryDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete helm repository: %w", err)
	}
	return nil
}

// SearchHelmCharts searches for helm charts in a repository.
func (a *portainerAPIAdapter) SearchHelmCharts(repo string, chart *string) (string, error) {
	params := helm.NewHelmRepoSearchParams().WithRepo(repo)
	if chart != nil {
		params = params.WithChart(chart)
	}
	resp, err := a.swagger.Helm.HelmRepoSearch(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to search helm charts: %w", err)
	}
	return resp.Payload, nil
}

// InstallHelmChart installs a helm chart on an environment.
func (a *portainerAPIAdapter) InstallHelmChart(environmentId int64, payload *apimodels.HelmInstallChartPayload) (*apimodels.ReleaseRelease, error) {
	params := helm.NewHelmInstallParams().WithID(environmentId).WithPayload(payload)
	resp, err := a.swagger.Helm.HelmInstall(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to install helm chart: %w", err)
	}
	return resp.Payload, nil
}

// ListHelmReleases lists helm releases on an environment.
func (a *portainerAPIAdapter) ListHelmReleases(environmentId int64, namespace *string, filter *string, selector *string) ([]*apimodels.ReleaseReleaseElement, error) {
	params := helm.NewHelmListParams().WithID(environmentId)
	if namespace != nil {
		params = params.WithNamespace(namespace)
	}
	if filter != nil {
		params = params.WithFilter(filter)
	}
	if selector != nil {
		params = params.WithSelector(selector)
	}
	resp, err := a.swagger.Helm.HelmList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list helm releases: %w", err)
	}
	return resp.Payload, nil
}

// DeleteHelmRelease deletes a helm release from an environment.
func (a *portainerAPIAdapter) DeleteHelmRelease(environmentId int64, release string, namespace *string) error {
	params := helm.NewHelmDeleteParams().WithID(environmentId).WithRelease(release)
	if namespace != nil {
		params = params.WithNamespace(namespace)
	}
	_, err := a.swagger.Helm.HelmDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete helm release: %w", err)
	}
	return nil
}

// GetHelmReleaseHistory gets the history of a helm release.
func (a *portainerAPIAdapter) GetHelmReleaseHistory(environmentId int64, name string, namespace *string) ([]*apimodels.ReleaseRelease, error) {
	params := helm.NewHelmGetHistoryParams().WithID(environmentId).WithName(name)
	if namespace != nil {
		params = params.WithNamespace(namespace)
	}
	resp, err := a.swagger.Helm.HelmGetHistory(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get helm release history: %w", err)
	}
	return resp.Payload, nil
}

// GetDockerDashboard retrieves the Docker dashboard data for a specific environment.
// Uses raw HTTP GET because the SDK sends POST but newer Portainer versions require GET.
func (a *portainerAPIAdapter) GetDockerDashboard(environmentId int64) (*apimodels.DockerDashboardResponse, error) {
	op := &runtime.ClientOperation{
		ID:                 "DockerDashboard",
		Method:             "GET",
		PathPattern:        fmt.Sprintf("/docker/%d/dashboard", environmentId),
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params: runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, reg strfmt.Registry) error {
			return nil
		}),
		AuthInfo: a.httpTransport.DefaultAuthentication,
		Reader: runtime.ClientResponseReaderFunc(func(resp runtime.ClientResponse, consumer runtime.Consumer) (any, error) {
			var result apimodels.DockerDashboardResponse
			if err := consumer.Consume(resp.Body(), &result); err != nil {
				return nil, err
			}
			return &result, nil
		}),
	}
	res, err := a.httpTransport.Submit(op)
	if err != nil {
		return nil, fmt.Errorf("failed to get docker dashboard: %w", err)
	}
	return res.(*apimodels.DockerDashboardResponse), nil
}

// GetKubernetesDashboard retrieves the Kubernetes dashboard data for a specific environment.
// Uses raw HTTP GET because the SDK expects an array but the API returns a single object.
func (a *portainerAPIAdapter) GetKubernetesDashboard(environmentId int64) (*apimodels.KubernetesK8sDashboard, error) {
	op := &runtime.ClientOperation{
		ID:                 "KubernetesDashboard",
		Method:             "GET",
		PathPattern:        fmt.Sprintf("/kubernetes/%d/dashboard", environmentId),
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"https"},
		Params: runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, reg strfmt.Registry) error {
			return nil
		}),
		AuthInfo: a.httpTransport.DefaultAuthentication,
		Reader: runtime.ClientResponseReaderFunc(func(resp runtime.ClientResponse, consumer runtime.Consumer) (any, error) {
			var result apimodels.KubernetesK8sDashboard
			if err := consumer.Consume(resp.Body(), &result); err != nil {
				return nil, err
			}
			return &result, nil
		}),
	}
	res, err := a.httpTransport.Submit(op)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes dashboard: %w", err)
	}
	return res.(*apimodels.KubernetesK8sDashboard), nil
}

// GetKubernetesNamespaces retrieves the Kubernetes namespaces for a specific environment.
func (a *portainerAPIAdapter) GetKubernetesNamespaces(environmentId int64) ([]*apimodels.PortainerK8sNamespaceInfo, error) {
	params := kubernetes.NewGetKubernetesNamespacesParams().WithID(environmentId)
	resp, err := a.swagger.Kubernetes.GetKubernetesNamespaces(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes namespaces: %w", err)
	}
	return resp.Payload, nil
}

// GetKubernetesConfig retrieves the Kubernetes config for a specific environment.
func (a *portainerAPIAdapter) GetKubernetesConfig(environmentId int64) (interface{}, error) {
	params := kubernetes.NewGetKubernetesConfigParams().WithIds([]int64{environmentId})
	resp, err := a.swagger.Kubernetes.GetKubernetesConfig(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}
	return resp.Payload, nil
}

// ListRegularStacks retrieves all regular (non-edge) stacks.
func (a *portainerAPIAdapter) ListRegularStacks() ([]*apimodels.PortainereeStack, error) {
	params := stacks.NewStackListParams()
	resp, respNoContent, err := a.swagger.Stacks.StackList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list regular stacks: %w", err)
	}
	if respNoContent != nil {
		return []*apimodels.PortainereeStack{}, nil
	}
	return resp.Payload, nil
}

// StackInspect retrieves details of a specific stack by ID.
func (a *portainerAPIAdapter) StackInspect(id int64) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackInspectParams().WithID(id)
	resp, err := a.swagger.Stacks.StackInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect stack: %w", err)
	}
	return resp.Payload, nil
}

// StackDelete removes a stack by ID.
func (a *portainerAPIAdapter) StackDelete(id int64, endpointID int64, removeVolumes bool) error {
	params := stacks.NewStackDeleteParams().WithID(id).WithEndpointID(endpointID).WithRemoveVolumes(&removeVolumes)
	_, err := a.swagger.Stacks.StackDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}
	return nil
}

// StackFileInspect retrieves the compose file content for a stack.
func (a *portainerAPIAdapter) StackFileInspect(id int64) (string, error) {
	params := stacks.NewStackFileInspectParams().WithID(id)
	resp, err := a.swagger.Stacks.StackFileInspect(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to inspect stack file: %w", err)
	}
	return resp.Payload.StackFileContent, nil
}

// StackUpdateGit updates the git configuration of a stack.
func (a *portainerAPIAdapter) StackUpdateGit(id int64, endpointID int64, body *apimodels.StacksStackGitUpdatePayload) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackUpdateGitParams().WithID(id).WithEndpointID(&endpointID).WithBody(body)
	resp, err := a.swagger.Stacks.StackUpdateGit(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update stack git: %w", err)
	}
	return resp.Payload, nil
}

// StackGitRedeploy triggers a git-based redeployment of a stack.
func (a *portainerAPIAdapter) StackGitRedeploy(id int64, endpointID int64, body *apimodels.StacksStackGitRedployPayload) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackGitRedeployParams().WithID(id).WithEndpointID(&endpointID).WithBody(body)
	resp, err := a.swagger.Stacks.StackGitRedeploy(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to redeploy stack: %w", err)
	}
	return resp.Payload, nil
}

// StackStart starts a stopped stack.
func (a *portainerAPIAdapter) StackStart(id int64, endpointID int64) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackStartParams().WithID(id).WithEndpointID(endpointID)
	resp, err := a.swagger.Stacks.StackStart(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start stack: %w", err)
	}
	return resp.Payload, nil
}

// StackStop stops a running stack.
func (a *portainerAPIAdapter) StackStop(id int64, endpointID int64) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackStopParams().WithID(id).WithEndpointID(endpointID)
	resp, err := a.swagger.Stacks.StackStop(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to stop stack: %w", err)
	}
	return resp.Payload, nil
}

// StackMigrate migrates a stack to another environment.
func (a *portainerAPIAdapter) StackMigrate(id int64, endpointID int64, body *apimodels.StacksStackMigratePayload) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackMigrateParams().WithID(id).WithEndpointID(&endpointID).WithBody(body)
	resp, err := a.swagger.Stacks.StackMigrate(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate stack: %w", err)
	}
	return resp.Payload, nil
}
