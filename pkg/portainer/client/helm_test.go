// Tests for Helm release management client methods covering all 8 helm operations.
// Run: go test ./pkg/portainer/client/ -run TestHelm -v
package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetHelmRepositories verifies retrieval of Helm repositories for a user.
func TestGetHelmRepositories(t *testing.T) {
	tests := []struct {
		name          string
		userId        int
		mockResult    *apimodels.UsersHelmUserRepositoryResponse
		mockError     error
		expectedError bool
	}{
		{
			name:   "successful retrieval",
			userId: 1,
			mockResult: &apimodels.UsersHelmUserRepositoryResponse{
				GlobalRepository: "https://charts.helm.sh/stable",
				UserRepositories: []*apimodels.PortainerHelmUserRepository{
					{URL: "https://charts.bitnami.com/bitnami"},
				},
			},
		},
		{
			name:          "API error",
			userId:        99,
			mockError:     errors.New("user not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListHelmRepositories", int64(tt.userId)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetHelmRepositories(tt.userId)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result.GlobalRepository)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateHelmRepository verifies creation of a Helm repository for a user.
func TestCreateHelmRepository(t *testing.T) {
	tests := []struct {
		name          string
		userId        int
		url           string
		mockResult    *apimodels.PortainerHelmUserRepository
		mockError     error
		expectedError bool
	}{
		{
			name:       "successful creation",
			userId:     1,
			url:        "https://charts.bitnami.com/bitnami",
			mockResult: &apimodels.PortainerHelmUserRepository{URL: "https://charts.bitnami.com/bitnami"},
		},
		{
			name:          "API error",
			userId:        1,
			url:           "invalid-url",
			mockError:     errors.New("invalid repository URL"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateHelmRepository", int64(tt.userId), tt.url).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.CreateHelmRepository(tt.userId, tt.url)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.url, result.URL)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteHelmRepository verifies deletion of a Helm repository.
func TestDeleteHelmRepository(t *testing.T) {
	tests := []struct {
		name          string
		userId        int
		repoId        int
		mockError     error
		expectedError bool
	}{
		{
			name:   "successful deletion",
			userId: 1,
			repoId: 5,
		},
		{
			name:          "API error",
			userId:        1,
			repoId:        99,
			mockError:     errors.New("repository not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("DeleteHelmRepository", int64(tt.userId), int64(tt.repoId)).Return(tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			err := c.DeleteHelmRepository(tt.userId, tt.repoId)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestSearchHelmCharts verifies searching for Helm charts in a repository.
func TestSearchHelmCharts(t *testing.T) {
	tests := []struct {
		name          string
		repo          string
		chart         string
		mockResult    string
		mockError     error
		expectedError bool
	}{
		{
			name:       "search with chart name",
			repo:       "https://charts.bitnami.com/bitnami",
			chart:      "nginx",
			mockResult: `{"entries":{"nginx":[{"name":"nginx","version":"15.0.0"}]}}`,
		},
		{
			name:       "search without chart name",
			repo:       "https://charts.bitnami.com/bitnami",
			chart:      "",
			mockResult: `{"entries":{}}`,
		},
		{
			name:          "API error",
			repo:          "https://invalid.repo",
			chart:         "test",
			mockError:     errors.New("repository unreachable"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			var chartPtr *string
			if tt.chart != "" {
				chartPtr = &tt.chart
			}
			mockAPI.On("SearchHelmCharts", tt.repo, chartPtr).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.SearchHelmCharts(tt.repo, tt.chart)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResult, result)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestInstallHelmChart verifies installation of a Helm chart.
func TestInstallHelmChart(t *testing.T) {
	tests := []struct {
		name          string
		envId         int
		chart         string
		releaseName   string
		namespace     string
		repo          string
		values        string
		version       string
		mockResult    *apimodels.ReleaseRelease
		mockError     error
		expectedError bool
	}{
		{
			name:        "successful installation",
			envId:       1,
			chart:       "nginx",
			releaseName: "my-nginx",
			namespace:   "default",
			repo:        "https://charts.bitnami.com/bitnami",
			values:      "replicaCount: 2",
			version:     "15.0.0",
			mockResult: &apimodels.ReleaseRelease{
				Name:      "my-nginx",
				Namespace: "default",
				Version:   1,
			},
		},
		{
			name:          "API error",
			envId:         1,
			chart:         "nonexistent",
			releaseName:   "test",
			namespace:     "default",
			repo:          "https://charts.bitnami.com/bitnami",
			mockError:     errors.New("chart not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("InstallHelmChart", int64(tt.envId), mock.AnythingOfType("*models.HelmInstallChartPayload")).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.InstallHelmChart(tt.envId, tt.chart, tt.releaseName, tt.namespace, tt.repo, tt.values, tt.version)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "my-nginx", result.Name)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetHelmReleases verifies retrieval of Helm releases.
func TestGetHelmReleases(t *testing.T) {
	tests := []struct {
		name          string
		envId         int
		namespace     string
		filter        string
		selector      string
		mockResult    []*apimodels.ReleaseReleaseElement
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:      "with all filters",
			envId:     1,
			namespace: "default",
			filter:    "nginx",
			selector:  "app=nginx",
			mockResult: []*apimodels.ReleaseReleaseElement{
				{Name: "my-nginx", Namespace: "default", Status: "deployed"},
			},
			expectedCount: 1,
		},
		{
			name:  "without filters",
			envId: 1,
			mockResult: []*apimodels.ReleaseReleaseElement{
				{Name: "release1", Namespace: "default", Status: "deployed"},
				{Name: "release2", Namespace: "kube-system", Status: "deployed"},
			},
			expectedCount: 2,
		},
		{
			name:          "API error",
			envId:         99,
			mockError:     errors.New("environment not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			var nsPtr, filterPtr, selectorPtr *string
			if tt.namespace != "" {
				nsPtr = &tt.namespace
			}
			if tt.filter != "" {
				filterPtr = &tt.filter
			}
			if tt.selector != "" {
				selectorPtr = &tt.selector
			}
			mockAPI.On("ListHelmReleases", int64(tt.envId), nsPtr, filterPtr, selectorPtr).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetHelmReleases(tt.envId, tt.namespace, tt.filter, tt.selector)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteHelmRelease verifies deletion of a Helm release.
func TestDeleteHelmRelease(t *testing.T) {
	tests := []struct {
		name          string
		envId         int
		release       string
		namespace     string
		mockError     error
		expectedError bool
	}{
		{
			name:      "with namespace",
			envId:     1,
			release:   "my-nginx",
			namespace: "default",
		},
		{
			name:    "without namespace",
			envId:   1,
			release: "my-redis",
		},
		{
			name:          "API error",
			envId:         1,
			release:       "nonexistent",
			mockError:     errors.New("release not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			var nsPtr *string
			if tt.namespace != "" {
				nsPtr = &tt.namespace
			}
			mockAPI.On("DeleteHelmRelease", int64(tt.envId), tt.release, nsPtr).Return(tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			err := c.DeleteHelmRelease(tt.envId, tt.release, tt.namespace)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetHelmReleaseHistory verifies retrieval of Helm release history.
func TestGetHelmReleaseHistory(t *testing.T) {
	tests := []struct {
		name          string
		envId         int
		releaseName   string
		namespace     string
		mockResult    []*apimodels.ReleaseRelease
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:        "with namespace",
			envId:       1,
			releaseName: "my-nginx",
			namespace:   "default",
			mockResult: []*apimodels.ReleaseRelease{
				{Name: "my-nginx", Version: 1},
				{Name: "my-nginx", Version: 2},
			},
			expectedCount: 2,
		},
		{
			name:        "without namespace",
			envId:       1,
			releaseName: "my-redis",
			mockResult: []*apimodels.ReleaseRelease{
				{Name: "my-redis", Version: 1},
			},
			expectedCount: 1,
		},
		{
			name:          "API error",
			envId:         1,
			releaseName:   "nonexistent",
			mockError:     errors.New("release not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			var nsPtr *string
			if tt.namespace != "" {
				nsPtr = &tt.namespace
			}
			mockAPI.On("GetHelmReleaseHistory", int64(tt.envId), tt.releaseName, nsPtr).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetHelmReleaseHistory(tt.envId, tt.releaseName, tt.namespace)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
