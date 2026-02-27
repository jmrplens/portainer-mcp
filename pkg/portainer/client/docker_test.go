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

// TestProxyDockerRequest verifies proxy docker request behavior.
func TestProxyDockerRequest(t *testing.T) {
	tests := []struct {
		name             string
		environmentId    int
		opts             models.DockerProxyRequestOptions
		mockResponse     *http.Response
		mockError        error
		expectedError    bool
		expectedStatus   int
		expectedRespBody string
	}{
		{
			name: "GET request with query parameters",
			opts: models.DockerProxyRequestOptions{
				EnvironmentID: 1,
				Method:        "GET",
				Path:          "/images/json",
				QueryParams:   map[string]string{"all": "true", "filter": "dangling"},
			},
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`[{"Id":"img1"}]`)),
			},
			mockError:        nil,
			expectedError:    false,
			expectedStatus:   http.StatusOK,
			expectedRespBody: `[{"Id":"img1"}]`,
		},
		{
			name: "POST request with custom headers",
			opts: models.DockerProxyRequestOptions{
				EnvironmentID: 2,
				Method:        "POST",
				Path:          "/networks/create",
				Headers:       map[string]string{"X-Custom-Header": "value1", "Authorization": "Bearer token"},
				Body:          bytes.NewBufferString(`{"Name": "my-network"}`),
			},
			mockResponse: &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{"Id": "net1"}`)),
			},
			mockError:        nil,
			expectedError:    false,
			expectedStatus:   http.StatusCreated,
			expectedRespBody: `{"Id": "net1"}`,
		},
		{
			name: "API error",
			opts: models.DockerProxyRequestOptions{
				EnvironmentID: 3,
				Method:        "GET",
				Path:          "/version",
			},
			mockResponse:     nil,
			mockError:        errors.New("failed to proxy request"),
			expectedError:    true,
			expectedStatus:   0,  // Not applicable
			expectedRespBody: "", // Not applicable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			opts := client.ProxyRequestOptions{
				Method:      tt.opts.Method,
				APIPath:     tt.opts.Path,
				QueryParams: tt.opts.QueryParams,
				Headers:     tt.opts.Headers,
				Body:        tt.opts.Body,
			}
			mockAPI.On("ProxyDockerRequest", tt.opts.EnvironmentID, opts).Return(tt.mockResponse, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			resp, err := client.ProxyDockerRequest(tt.opts)
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
				}
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetDockerDashboard verifies retrieval of Docker dashboard data for an environment.
func TestGetDockerDashboard(t *testing.T) {
	tests := []struct {
		name          string
		envID         int
		mockResult    *apimodels.DockerDashboardResponse
		mockError     error
		expectedError bool
	}{
		{
			name:  "successful retrieval",
			envID: 1,
			mockResult: &apimodels.DockerDashboardResponse{
				Networks: 4,
				Volumes:  3,
				Stacks:   2,
				Services: 3,
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
			mockAPI.On("GetDockerDashboard", int64(tt.envID)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetDockerDashboard(tt.envID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 4, result.Networks)
				assert.Equal(t, 2, result.Stacks)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
