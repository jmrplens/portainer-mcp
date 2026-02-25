package client

import (
	"fmt"
	"net/http"

	"github.com/portainer/client-api-go/v2/client"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
)

// GetDockerDashboard retrieves the Docker dashboard data for a specific environment.
//
// Parameters:
//   - environmentId: The ID of the environment to get dashboard data for
//
// Returns:
//   - A DockerDashboard object with container, image, network, volume, stack, and service counts
//   - An error if the operation fails
func (c *PortainerClient) GetDockerDashboard(environmentId int) (models.DockerDashboard, error) {
	raw, err := c.cli.GetDockerDashboard(int64(environmentId))
	if err != nil {
		return models.DockerDashboard{}, fmt.Errorf("failed to get docker dashboard: %w", err)
	}

	return models.ConvertDockerDashboardResponse(raw), nil
}

// ProxyDockerRequest proxies a Docker API request to a specific Portainer environment.
//
// Parameters:
//   - opts: Options defining the proxied request (environmentID, method, path, query params, headers, body)
//
// Returns:
//   - *http.Response: The response from the Docker API
//   - error: Any error that occurred during the request
func (c *PortainerClient) ProxyDockerRequest(opts models.DockerProxyRequestOptions) (*http.Response, error) {
	proxyOpts := client.ProxyRequestOptions{
		Method:  opts.Method,
		APIPath: opts.Path,
		Body:    opts.Body,
	}

	if len(opts.QueryParams) > 0 {
		proxyOpts.QueryParams = opts.QueryParams
	}

	if len(opts.Headers) > 0 {
		proxyOpts.Headers = opts.Headers
	}

	return c.cli.ProxyDockerRequest(opts.EnvironmentID, proxyOpts)
}
