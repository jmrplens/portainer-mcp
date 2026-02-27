package client

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/portainer/client-api-go/v2/client"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestProxyKubernetesRequest verifies proxy kubernetes request behavior.
func TestProxyKubernetesRequest(t *testing.T) {
	tests := []struct {
		name             string
		opts             models.KubernetesProxyRequestOptions
		mockResponse     *http.Response
		mockError        error
		expectedError    bool
		expectedStatus   int
		expectedRespBody string
	}{
		{
			name: "GET request with query parameters",
			opts: models.KubernetesProxyRequestOptions{
				EnvironmentID: 1,
				Method:        "GET",
				Path:          "/api/v1/pods",
				QueryParams:   map[string]string{"namespace": "default", "labelSelector": "app=myapp"},
			},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"items": [{"metadata": {"name": "pod1"}}]}`)),
			},
			mockError:        nil,
			expectedError:    false,
			expectedStatus:   http.StatusOK,
			expectedRespBody: `{"items": [{"metadata": {"name": "pod1"}}]}`,
		},
		{
			name: "POST request with custom headers and body",
			opts: models.KubernetesProxyRequestOptions{
				EnvironmentID: 2,
				Method:        "POST",
				Path:          "/api/v1/namespaces/default/services",
				Headers:       map[string]string{"X-Custom-Header": "value1", "Content-Type": "application/json"},
				Body:          bytes.NewBufferString(`{"apiVersion": "v1", "kind": "Service", "metadata": {"name": "my-service"}}`),
			},
			mockResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"metadata": {"name": "my-service"}}`)),
			},
			mockError:        nil,
			expectedError:    false,
			expectedStatus:   http.StatusCreated,
			expectedRespBody: `{"metadata": {"name": "my-service"}}`,
		},
		{
			name: "API error",
			opts: models.KubernetesProxyRequestOptions{
				EnvironmentID: 3,
				Method:        "GET",
				Path:          "/version",
			},
			mockResponse:     nil,
			mockError:        errors.New("failed to proxy kubernetes request"),
			expectedError:    true,
			expectedStatus:   0,  // Not applicable
			expectedRespBody: "", // Not applicable
		},
		{
			name: "Request with no params, headers, or body",
			opts: models.KubernetesProxyRequestOptions{
				EnvironmentID: 4,
				Method:        "GET",
				Path:          "/healthz",
			},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
			},
			mockError:        nil,
			expectedError:    false,
			expectedStatus:   http.StatusOK,
			expectedRespBody: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			proxyOpts := client.ProxyRequestOptions{
				Method:      tt.opts.Method,
				APIPath:     tt.opts.Path,
				QueryParams: tt.opts.QueryParams,
				Headers:     tt.opts.Headers,
				Body:        tt.opts.Body,
			}
			mockAPI.On("ProxyKubernetesRequest", tt.opts.EnvironmentID, proxyOpts).Return(tt.mockResponse, tt.mockError)

			portainerClient := &PortainerClient{cli: mockAPI}

			resp, err := portainerClient.ProxyKubernetesRequest(tt.opts)

			if tt.expectedError {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.mockError.Error())
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)

				// Read and verify the response body
				if assert.NotNil(t, resp.Body) { // Ensure body is not nil before reading
					defer resp.Body.Close()
					bodyBytes, readErr := io.ReadAll(resp.Body)
					assert.NoError(t, readErr)
					assert.Equal(t, tt.expectedRespBody, string(bodyBytes))
				} else if tt.expectedRespBody != "" {
					assert.Fail(t, "Expected a response body but got nil")
				}
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetKubernetesDashboard verifies retrieval of Kubernetes dashboard data.
func TestGetKubernetesDashboard(t *testing.T) {
	tests := []struct {
		name          string
		envID         int
		mockResult    *apimodels.KubernetesK8sDashboard
		mockError     error
		expectedError bool
	}{
		{
			name:  "successful retrieval",
			envID: 1,
			mockResult: &apimodels.KubernetesK8sDashboard{
				ApplicationsCount: 5,
				ConfigMapsCount:   3,
				IngressesCount:    2,
				NamespacesCount:   4,
				SecretsCount:      6,
				ServicesCount:     3,
				VolumesCount:      2,
			},
		},
		{
			name:          "API error",
			envID:         99,
			mockError:     errors.New("environment not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetKubernetesDashboard", int64(tt.envID)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetKubernetesDashboard(tt.envID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 5, result.ApplicationsCount)
				assert.Equal(t, 4, result.NamespacesCount)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetKubernetesNamespaces verifies retrieval of Kubernetes namespaces.
func TestGetKubernetesNamespaces(t *testing.T) {
	tests := []struct {
		name          string
		envID         int
		mockResult    []*apimodels.PortainerK8sNamespaceInfo
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name:  "successful retrieval",
			envID: 1,
			mockResult: []*apimodels.PortainerK8sNamespaceInfo{
				{Name: "default", IsDefault: true, IsSystem: false},
				{Name: "kube-system", IsDefault: false, IsSystem: true},
			},
			expectedCount: 2,
		},
		{
			name:          "empty list",
			envID:         1,
			mockResult:    []*apimodels.PortainerK8sNamespaceInfo{},
			expectedCount: 0,
		},
		{
			name:          "API error",
			envID:         99,
			mockError:     errors.New("environment not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetKubernetesNamespaces", int64(tt.envID)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetKubernetesNamespaces(tt.envID)

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

// TestGetKubernetesConfig verifies retrieval of kubeconfig for an environment.
func TestGetKubernetesConfig(t *testing.T) {
	tests := []struct {
		name          string
		envID         int
		mockResult    interface{}
		mockError     error
		expectedError bool
	}{
		{
			name:       "successful retrieval",
			envID:      1,
			mockResult: map[string]interface{}{"apiVersion": "v1", "kind": "Config"},
		},
		{
			name:          "API error",
			envID:         99,
			mockError:     errors.New("environment not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetKubernetesConfig", int64(tt.envID)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetKubernetesConfig(tt.envID)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
