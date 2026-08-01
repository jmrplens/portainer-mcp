package models

import (
	"encoding/json"
	"testing"
	"time"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/assert"
)

// --- AppTemplate ---

// TestConvertToAppTemplate verifies the ConvertToAppTemplate model conversion function.
func TestConvertToAppTemplate(t *testing.T) {
	raw := &apimodels.PortainerTemplate{
		ID:          42,
		Title:       "WordPress",
		Description: "A blog platform",
		Type:        2,
		Image:       "wordpress:latest",
		Categories:  []string{"CMS", "Blog"},
		Platform:    "linux",
		Logo:        "https://example.com/logo.png",
		Name:        "wordpress",
		Note:        "Requires MySQL",
	}

	result := ConvertToAppTemplate(raw)

	assert.Equal(t, 42, result.ID)
	assert.Equal(t, "WordPress", result.Title)
	assert.Equal(t, "A blog platform", result.Description)
	assert.Equal(t, 2, result.Type)
	assert.Equal(t, "wordpress:latest", result.Image)
	assert.Equal(t, []string{"CMS", "Blog"}, result.Categories)
	assert.Equal(t, "linux", result.Platform)
	assert.Equal(t, "https://example.com/logo.png", result.Logo)
	assert.Equal(t, "wordpress", result.Name)
	assert.Equal(t, "Requires MySQL", result.Note)
}

// TestConvertToAppTemplates verifies the ConvertToAppTemplates model conversion function.
func TestConvertToAppTemplates(t *testing.T) {
	raw := []*apimodels.PortainerTemplate{
		{ID: 1, Title: "App1"},
		{ID: 2, Title: "App2"},
	}

	result := ConvertToAppTemplates(raw)

	assert.Len(t, result, 2)
	assert.Equal(t, 1, result[0].ID)
	assert.Equal(t, "App2", result[1].Title)
}

// --- Backup ---

// TestConvertToBackupStatus verifies the ConvertToBackupStatus model conversion function.
func TestConvertToBackupStatus(t *testing.T) {
	raw := &apimodels.BackupBackupStatus{
		Failed:       true,
		TimestampUTC: "2024-01-15T10:30:00Z",
	}

	result := ConvertToBackupStatus(raw)

	assert.True(t, result.Failed)
	assert.Equal(t, "2024-01-15T10:30:00Z", result.TimestampUTC)
}

// TestConvertToS3BackupSettings verifies the ConvertToS3BackupSettings model conversion function.
func TestConvertToS3BackupSettings(t *testing.T) {
	raw := &apimodels.PortainereeS3BackupSettings{
		AccessKeyID:      "AKID123",
		BucketName:       "my-bucket",
		CronRule:         "0 0 * * *",
		Password:         "secret",
		Region:           "us-east-1",
		S3CompatibleHost: "https://s3.example.com",
		SecretAccessKey:  "SAK456",
	}

	result := ConvertToS3BackupSettings(raw)

	assert.Equal(t, "AKID123", result.AccessKeyID)
	assert.Equal(t, "my-bucket", result.BucketName)
	assert.Equal(t, "0 0 * * *", result.CronRule)
	assert.Equal(t, "secret", result.Password)
	assert.Equal(t, "us-east-1", result.Region)
	assert.Equal(t, "https://s3.example.com", result.S3CompatibleHost)
	assert.Equal(t, "SAK456", result.SecretAccessKey)
}

// --- Docker Dashboard ---

// TestConvertDockerDashboardResponse verifies the ConvertDockerDashboardResponse model conversion function.
func TestConvertDockerDashboardResponse(t *testing.T) {
	tests := []struct {
		name     string
		raw      *apimodels.DockerDashboardResponse
		expected DockerDashboard
	}{
		{
			name: "full response with containers and images",
			raw: &apimodels.DockerDashboardResponse{
				Networks: 3,
				Services: 5,
				Stacks:   2,
				Volumes:  7,
				Containers: &apimodels.DockerContainerStats{
					Healthy:   4,
					Running:   8,
					Stopped:   2,
					Total:     10,
					Unhealthy: 1,
				},
				Images: &apimodels.DockerImagesCounters{
					Size:  1024000,
					Total: 15,
				},
			},
			expected: DockerDashboard{
				Networks: 3,
				Services: 5,
				Stacks:   2,
				Volumes:  7,
				Containers: DockerContainerStats{
					Healthy:   4,
					Running:   8,
					Stopped:   2,
					Total:     10,
					Unhealthy: 1,
				},
				Images: DockerImagesCounters{
					Size:  1024000,
					Total: 15,
				},
			},
		},
		{
			name: "nil containers and images",
			raw: &apimodels.DockerDashboardResponse{
				Networks:   1,
				Containers: nil,
				Images:     nil,
			},
			expected: DockerDashboard{
				Networks:   1,
				Containers: DockerContainerStats{},
				Images:     DockerImagesCounters{},
			},
		},
		{
			name: "only containers populated",
			raw: &apimodels.DockerDashboardResponse{
				Containers: &apimodels.DockerContainerStats{Running: 5},
			},
			expected: DockerDashboard{
				Containers: DockerContainerStats{Running: 5},
				Images:     DockerImagesCounters{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertDockerDashboardResponse(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Edge Job ---

// TestConvertEdgeJobToLocal verifies the ConvertEdgeJobToLocal model conversion function.
func TestConvertEdgeJobToLocal(t *testing.T) {
	raw := &apimodels.PortainerEdgeJob{
		ID:             10,
		Name:           "backup-job",
		CronExpression: "0 2 * * *",
		Recurring:      true,
		Created:        1700000000,
		Version:        3,
		EdgeGroups:     []int64{1, 2, 5},
	}

	result := ConvertEdgeJobToLocal(raw)

	assert.Equal(t, 10, result.ID)
	assert.Equal(t, "backup-job", result.Name)
	assert.Equal(t, "0 2 * * *", result.CronExpression)
	assert.True(t, result.Recurring)
	assert.Equal(t, int64(1700000000), result.Created)
	assert.Equal(t, 3, result.Version)
	assert.Equal(t, []int{1, 2, 5}, result.EdgeGroups)
}

// TestConvertEdgeJobToLocal_EmptyEdgeGroups verifies the ConvertEdgeJobToLocal_EmptyEdgeGroups model conversion function.
func TestConvertEdgeJobToLocal_EmptyEdgeGroups(t *testing.T) {
	raw := &apimodels.PortainerEdgeJob{
		Name:       "simple-job",
		EdgeGroups: nil,
	}

	result := ConvertEdgeJobToLocal(raw)

	assert.Equal(t, "simple-job", result.Name)
	assert.Empty(t, result.EdgeGroups)
}

// TestConvertEdgeUpdateScheduleToLocal verifies the ConvertEdgeUpdateScheduleToLocal model conversion function.
func TestConvertEdgeUpdateScheduleToLocal(t *testing.T) {
	raw := &apimodels.EdgeupdateschedulesDecoratedUpdateSchedule{
		ID:            7,
		Name:          "weekly-update",
		Type:          1,
		ScheduledTime: "2024-02-01T03:00:00Z",
		Status:        2,
		StatusMessage: "Completed",
		Created:       1706000000,
		CreatedBy:     1,
		EdgeGroupIds:  []int64{3, 4, 6},
	}

	result := ConvertEdgeUpdateScheduleToLocal(raw)

	assert.Equal(t, 7, result.ID)
	assert.Equal(t, "weekly-update", result.Name)
	assert.Equal(t, 1, result.Type)
	assert.Equal(t, "2024-02-01T03:00:00Z", result.ScheduledTime)
	assert.Equal(t, 2, result.Status)
	assert.Equal(t, "Completed", result.StatusMessage)
	assert.Equal(t, int64(1706000000), result.Created)
	assert.Equal(t, 1, result.CreatedBy)
	assert.Equal(t, []int{3, 4, 6}, result.EdgeGroupIds)
}

// --- Helm ---

// TestConvertToHelmRepository verifies the ConvertToHelmRepository model conversion function.
func TestConvertToHelmRepository(t *testing.T) {
	raw := &apimodels.PortainerHelmUserRepository{
		ID:     5,
		URL:    "https://charts.example.com",
		UserID: 2,
	}

	result := ConvertToHelmRepository(raw)

	assert.Equal(t, 5, result.ID)
	assert.Equal(t, "https://charts.example.com", result.URL)
	assert.Equal(t, 2, result.UserID)
}

// TestConvertToHelmRepositoryList verifies the ConvertToHelmRepositoryList model conversion function.
func TestConvertToHelmRepositoryList(t *testing.T) {
	raw := &apimodels.UsersHelmUserRepositoryResponse{
		GlobalRepository: "https://charts.bitnami.com/bitnami",
		UserRepositories: []*apimodels.PortainerHelmUserRepository{
			{ID: 1, URL: "https://repo1.example.com", UserID: 1},
			{ID: 2, URL: "https://repo2.example.com", UserID: 1},
		},
	}

	result := ConvertToHelmRepositoryList(raw)

	assert.Equal(t, "https://charts.bitnami.com/bitnami", result.GlobalRepository)
	assert.Len(t, result.UserRepositories, 2)
	assert.Equal(t, 1, result.UserRepositories[0].ID)
	assert.Equal(t, "https://repo2.example.com", result.UserRepositories[1].URL)
}

// TestConvertToHelmRepositoryList_Empty verifies the ConvertToHelmRepositoryList_Empty model conversion function.
func TestConvertToHelmRepositoryList_Empty(t *testing.T) {
	raw := &apimodels.UsersHelmUserRepositoryResponse{
		GlobalRepository: "https://global.repo",
		UserRepositories: nil,
	}

	result := ConvertToHelmRepositoryList(raw)

	assert.Equal(t, "https://global.repo", result.GlobalRepository)
	assert.Empty(t, result.UserRepositories)
}

// TestConvertToHelmRelease verifies the ConvertToHelmRelease model conversion function.
func TestConvertToHelmRelease(t *testing.T) {
	raw := &apimodels.ReleaseReleaseElement{
		Name:       "my-release",
		Namespace:  "default",
		Revision:   "3",
		Status:     "deployed",
		Chart:      "nginx-1.2.3",
		AppVersion: "1.25.0",
		Updated:    "2024-01-15T10:00:00Z",
	}

	result := ConvertToHelmRelease(raw)

	assert.Equal(t, "my-release", result.Name)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, "3", result.Revision)
	assert.Equal(t, "deployed", result.Status)
	assert.Equal(t, "nginx-1.2.3", result.Chart)
	assert.Equal(t, "1.25.0", result.AppVersion)
	assert.Equal(t, "2024-01-15T10:00:00Z", result.Updated)
}

// TestConvertToHelmReleaseDetails verifies the ConvertToHelmReleaseDetails model conversion function.
func TestConvertToHelmReleaseDetails(t *testing.T) {
	tests := []struct {
		name     string
		raw      *apimodels.ReleaseRelease
		expected HelmReleaseDetails
	}{
		{
			name: "with info",
			raw: &apimodels.ReleaseRelease{
				Name:       "my-app",
				Namespace:  "production",
				Version:    5,
				AppVersion: "2.0.0",
				Info:       &apimodels.ReleaseInfo{Status: "deployed"},
			},
			expected: HelmReleaseDetails{
				Name:       "my-app",
				Namespace:  "production",
				Version:    5,
				AppVersion: "2.0.0",
				Status:     "deployed",
			},
		},
		{
			name: "nil info",
			raw: &apimodels.ReleaseRelease{
				Name:    "orphan",
				Version: 1,
				Info:    nil,
			},
			expected: HelmReleaseDetails{
				Name:    "orphan",
				Version: 1,
				Status:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToHelmReleaseDetails(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Kubernetes ---

// TestConvertK8sDashboard verifies the ConvertK8sDashboard model conversion function.
func TestConvertK8sDashboard(t *testing.T) {
	raw := &apimodels.KubernetesK8sDashboard{
		ApplicationsCount: 10,
		ConfigMapsCount:   25,
		IngressesCount:    3,
		NamespacesCount:   5,
		SecretsCount:      12,
		ServicesCount:     8,
		VolumesCount:      4,
	}

	result := ConvertK8sDashboard(raw)

	assert.Equal(t, 10, result.ApplicationsCount)
	assert.Equal(t, 25, result.ConfigMapsCount)
	assert.Equal(t, 3, result.IngressesCount)
	assert.Equal(t, 5, result.NamespacesCount)
	assert.Equal(t, 12, result.SecretsCount)
	assert.Equal(t, 8, result.ServicesCount)
	assert.Equal(t, 4, result.VolumesCount)
}

// TestConvertK8sNamespace verifies the ConvertK8sNamespace model conversion function.
func TestConvertK8sNamespace(t *testing.T) {
	raw := &apimodels.PortainerK8sNamespaceInfo{
		ID:             "ns-123",
		Name:           "production",
		CreationDate:   "2024-01-01T00:00:00Z",
		NamespaceOwner: "admin",
		IsDefault:      false,
		IsSystem:       false,
	}

	result := ConvertK8sNamespace(raw)

	assert.Equal(t, "ns-123", result.ID)
	assert.Equal(t, "production", result.Name)
	assert.Equal(t, "2024-01-01T00:00:00Z", result.CreationDate)
	assert.Equal(t, "admin", result.NamespaceOwner)
	assert.False(t, result.IsDefault)
	assert.False(t, result.IsSystem)
}

// --- MOTD ---

// TestConvertToMOTDFromMap verifies the ConvertToMOTDFromMap model conversion function.
func TestConvertToMOTDFromMap(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]any
		validate func(t *testing.T, result MOTD)
	}{
		{
			name: "full map",
			raw: map[string]any{
				"Title":   "Welcome",
				"Message": "System update scheduled",
				"Style":   "info",
				"Hash":    "abc123",
				"ContentLayout": map[string]any{
					"header": "Important Notice",
					"footer": "Thank you",
				},
			},
			validate: func(t *testing.T, result MOTD) {
				assert.Equal(t, "Welcome", result.Title)
				assert.Equal(t, "System update scheduled", result.Message)
				assert.Equal(t, "info", result.Style)
				assert.NotNil(t, result.Hash)
				assert.Equal(t, "Important Notice", result.ContentLayout["header"])
				assert.Equal(t, "Thank you", result.ContentLayout["footer"])
			},
		},
		{
			name: "empty map",
			raw:  map[string]any{},
			validate: func(t *testing.T, result MOTD) {
				assert.Empty(t, result.Title)
				assert.Empty(t, result.Message)
				assert.Nil(t, result.ContentLayout)
			},
		},
		{
			name: "wrong types in map",
			raw: map[string]any{
				"Title":   123,
				"Message": true,
			},
			validate: func(t *testing.T, result MOTD) {
				assert.Empty(t, result.Title)
				assert.Empty(t, result.Message)
			},
		},
		{
			name: "hash as array",
			raw: map[string]any{
				"Hash": []int64{1, 2, 3},
			},
			validate: func(t *testing.T, result MOTD) {
				var hashVal []int64
				err := json.Unmarshal(result.Hash, &hashVal)
				assert.NoError(t, err)
				assert.Equal(t, []int64{1, 2, 3}, hashVal)
			},
		},
		{
			name: "content layout with non-string values",
			raw: map[string]any{
				"ContentLayout": map[string]any{
					"valid":   "text",
					"invalid": 999,
				},
			},
			validate: func(t *testing.T, result MOTD) {
				assert.Equal(t, "text", result.ContentLayout["valid"])
				_, exists := result.ContentLayout["invalid"]
				assert.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToMOTDFromMap(tt.raw)
			tt.validate(t, result)
		})
	}
}

// --- Registry ---

// TestConvertRawRegistryToRegistry verifies the ConvertRawRegistryToRegistry model conversion function.
func TestConvertRawRegistryToRegistry(t *testing.T) {
	raw := &apimodels.PortainereeRegistry{
		ID:             3,
		Name:           "Docker Hub",
		Type:           6,
		URL:            "docker.io",
		BaseURL:        "https://hub.docker.com",
		Authentication: true,
		Username:       "myuser",
	}

	result := ConvertRawRegistryToRegistry(raw)

	assert.Equal(t, 3, result.ID)
	assert.Equal(t, "Docker Hub", result.Name)
	assert.Equal(t, 6, result.Type)
	assert.Equal(t, "docker.io", result.URL)
	assert.Equal(t, "https://hub.docker.com", result.BaseURL)
	assert.True(t, result.Authentication)
	assert.Equal(t, "myuser", result.Username)
}

// --- Role ---

// TestConvertToRole verifies the ConvertToRole model conversion function.
func TestConvertToRole(t *testing.T) {
	id := int64(1)
	name := "Administrator"
	desc := "Full access"
	priority := int64(1)

	tests := []struct {
		name     string
		raw      *apimodels.PortainereeRole
		expected Role
	}{
		{
			name: "all fields populated",
			raw: &apimodels.PortainereeRole{
				ID:             &id,
				Name:           &name,
				Description:    &desc,
				Priority:       &priority,
				Authorizations: map[string]bool{"admin": true, "read": true},
			},
			expected: Role{
				ID:             1,
				Name:           "Administrator",
				Description:    "Full access",
				Priority:       1,
				Authorizations: map[string]bool{"admin": true, "read": true},
			},
		},
		{
			name: "all pointers nil",
			raw: &apimodels.PortainereeRole{
				ID:          nil,
				Name:        nil,
				Description: nil,
				Priority:    nil,
			},
			expected: Role{},
		},
		{
			name: "partial pointers",
			raw: &apimodels.PortainereeRole{
				ID:   &id,
				Name: &name,
			},
			expected: Role{
				ID:   1,
				Name: "Administrator",
			},
		},
		{
			name: "nil authorizations",
			raw: &apimodels.PortainereeRole{
				ID:             &id,
				Authorizations: nil,
			},
			expected: Role{
				ID: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToRole(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- SSL ---

// TestConvertToSSLSettings verifies the ConvertToSSLSettings model conversion function.
func TestConvertToSSLSettings(t *testing.T) {
	raw := &apimodels.PortainereeSSLSettings{
		CertPath:    "/certs/cert.pem",
		KeyPath:     "/certs/key.pem",
		CaCertPath:  "/certs/ca.pem",
		HTTPEnabled: true,
		SelfSigned:  false,
	}

	result := ConvertToSSLSettings(raw)

	assert.Equal(t, "/certs/cert.pem", result.CertPath)
	assert.Equal(t, "/certs/key.pem", result.KeyPath)
	assert.Equal(t, "/certs/ca.pem", result.CACertPath)
	assert.True(t, result.HTTPEnabled)
	assert.False(t, result.SelfSigned)
}

// --- Stack ---

// TestConvertRegularStack verifies the ConvertRegularStack model conversion function.
func TestConvertRegularStack(t *testing.T) {
	tests := []struct {
		name     string
		raw      *apimodels.PortainereeStack
		validate func(t *testing.T, result RegularStack)
	}{
		{
			name: "with creation date",
			raw: &apimodels.PortainereeStack{
				ID:             5,
				Name:           "web-app",
				Type:           2,
				Status:         1,
				EndpointID:     1,
				EntryPoint:     "docker-compose.yml",
				SwarmID:        "swarm-abc",
				CreatedBy:      "admin",
				CreationDate:   1700000000,
				FilesystemPath: "/data/compose/5",
			},
			validate: func(t *testing.T, result RegularStack) {
				assert.Equal(t, 5, result.ID)
				assert.Equal(t, "web-app", result.Name)
				assert.Equal(t, 2, result.Type)
				assert.Equal(t, 1, result.Status)
				assert.Equal(t, 1, result.EndpointID)
				assert.Equal(t, "docker-compose.yml", result.EntryPoint)
				assert.Equal(t, "swarm-abc", result.SwarmID)
				assert.Equal(t, "admin", result.CreatedBy)
				expected := time.Unix(1700000000, 0).Format(time.RFC3339)
				assert.Equal(t, expected, result.CreatedAt)
				assert.Equal(t, "/data/compose/5", result.FilesystemPath)
			},
		},
		{
			name: "zero creation date",
			raw: &apimodels.PortainereeStack{
				ID:           3,
				Name:         "old-stack",
				CreationDate: 0,
			},
			validate: func(t *testing.T, result RegularStack) {
				assert.Equal(t, 3, result.ID)
				assert.Equal(t, "old-stack", result.Name)
				assert.Empty(t, result.CreatedAt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertRegularStack(tt.raw)
			tt.validate(t, result)
		})
	}
}

// --- System ---

// TestConvertToSystemStatus verifies the ConvertToSystemStatus model conversion function.
func TestConvertToSystemStatus(t *testing.T) {
	raw := &apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus{
		Version:    "2.20.0",
		InstanceID: "instance-abc-123",
	}

	result := ConvertToSystemStatus(raw)

	assert.Equal(t, "2.20.0", result.Version)
	assert.Equal(t, "instance-abc-123", result.InstanceID)
}

// --- Webhook ---

// TestConvertToWebhook verifies the ConvertToWebhook model conversion function.
func TestConvertToWebhook(t *testing.T) {
	raw := &apimodels.PortainerWebhook{
		ID:         10,
		EndpointID: 2,
		RegistryID: 3,
		ResourceID: "container-abc",
		Token:      "wh-token-xyz",
		Type:       1,
	}

	result := ConvertToWebhook(raw)

	assert.Equal(t, 10, result.ID)
	assert.Equal(t, 2, result.EndpointID)
	assert.Equal(t, 3, result.RegistryID)
	assert.Equal(t, "container-abc", result.ResourceID)
	assert.Equal(t, "wh-token-xyz", result.Token)
	assert.Equal(t, 1, result.Type)
}

// --- Public Settings ---

// TestConvertToPublicSettings verifies the ConvertToPublicSettings model conversion function.
func TestConvertToPublicSettings(t *testing.T) {
	raw := &apimodels.SettingsPublicSettingsResponse{
		AuthenticationMethod:      1,
		EnableEdgeComputeFeatures: true,
		EnableTelemetry:           false,
		LogoURL:                   "https://example.com/logo.png",
		OAuthLoginURI:             "https://auth.example.com/login",
		OAuthLogoutURI:            "https://auth.example.com/logout",
		OAuthHideInternalAuth:     true,
		RequiredPasswordLength:    12,
		Features:                  map[string]bool{"feature1": true},
	}

	result := ConvertToPublicSettings(raw)

	assert.Equal(t, AuthenticationMethodInternal, result.AuthenticationMethod)
	assert.True(t, result.EnableEdgeComputeFeatures)
	assert.False(t, result.EnableTelemetry)
	assert.Equal(t, "https://example.com/logo.png", result.LogoURL)
	assert.Equal(t, "https://auth.example.com/login", result.OAuthLoginURI)
	assert.Equal(t, "https://auth.example.com/logout", result.OAuthLogoutURI)
	assert.True(t, result.OAuthHideInternalAuth)
	assert.Equal(t, 12, result.RequiredPasswordLength)
	assert.Equal(t, map[string]bool{"feature1": true}, result.Features)
}
