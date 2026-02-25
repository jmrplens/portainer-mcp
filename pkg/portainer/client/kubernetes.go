package client

import (
	"fmt"
	"net/http"

	"github.com/portainer/client-api-go/v2/client"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
)

// ProxyKubernetesRequest proxies a Kubernetes API request to a specific Portainer environment.
//
// Parameters:
//   - opts: Options defining the proxied request (environmentID, method, path, query params, headers, body)
//
// Returns:
//   - *http.Response: The response from the Kubernetes API
//   - error: Any error that occurred during the request
func (c *PortainerClient) ProxyKubernetesRequest(opts models.KubernetesProxyRequestOptions) (*http.Response, error) {
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

	return c.cli.ProxyKubernetesRequest(opts.EnvironmentID, proxyOpts)
}

// GetKubernetesDashboard retrieves the Kubernetes dashboard summary for a specific environment.
//
// Parameters:
//   - environmentId: The ID of the environment
//
// Returns:
//   - A KubernetesDashboard with resource counts
//   - An error if the operation fails
func (c *PortainerClient) GetKubernetesDashboard(environmentId int) (models.KubernetesDashboard, error) {
	dashboards, err := c.cli.GetKubernetesDashboard(int64(environmentId))
	if err != nil {
		return models.KubernetesDashboard{}, fmt.Errorf("failed to get kubernetes dashboard: %w", err)
	}

	if len(dashboards) == 0 {
		return models.KubernetesDashboard{}, fmt.Errorf("no kubernetes dashboard data returned")
	}

	return models.ConvertK8sDashboard(dashboards[0]), nil
}

// GetKubernetesNamespaces retrieves the Kubernetes namespaces for a specific environment.
//
// Parameters:
//   - environmentId: The ID of the environment
//
// Returns:
//   - A slice of KubernetesNamespace objects
//   - An error if the operation fails
func (c *PortainerClient) GetKubernetesNamespaces(environmentId int) ([]models.KubernetesNamespace, error) {
	rawNamespaces, err := c.cli.GetKubernetesNamespaces(int64(environmentId))
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes namespaces: %w", err)
	}

	namespaces := make([]models.KubernetesNamespace, len(rawNamespaces))
	for i, raw := range rawNamespaces {
		namespaces[i] = models.ConvertK8sNamespace(raw)
	}

	return namespaces, nil
}

// GetKubernetesConfig retrieves the kubeconfig for a specific environment.
//
// Parameters:
//   - environmentId: The ID of the environment
//
// Returns:
//   - The kubeconfig content as an interface{}
//   - An error if the operation fails
func (c *PortainerClient) GetKubernetesConfig(environmentId int) (interface{}, error) {
	config, err := c.cli.GetKubernetesConfig(int64(environmentId))
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	return config, nil
}
