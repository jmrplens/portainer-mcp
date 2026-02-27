package models

import (
	"io"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// KubernetesProxyRequestOptions represents the options for a Kubernetes API request to a specific Portainer environment.
type KubernetesProxyRequestOptions struct {
	// EnvironmentID is the ID of the environment to proxy the request to.
	EnvironmentID int
	// Method is the HTTP method to use (GET, POST, PUT, DELETE, etc.).
	Method string
	// Path is the Kubernetes API endpoint path to proxy to (e.g., "/api/v1/namespaces/default/pods"). Must include the leading slash.
	Path string
	// QueryParams is a map of query parameters to include in the request URL.
	QueryParams map[string]string
	// Headers is a map of headers to include in the request.
	Headers map[string]string
	// Body is the request body to send (set it to nil for requests that don't have a body).
	Body io.Reader
}

// KubernetesDashboard represents a summary of Kubernetes resource counts.
type KubernetesDashboard struct {
	ApplicationsCount int `json:"applicationsCount"`
	ConfigMapsCount   int `json:"configMapsCount"`
	IngressesCount    int `json:"ingressesCount"`
	NamespacesCount   int `json:"namespacesCount"`
	SecretsCount      int `json:"secretsCount"`
	ServicesCount     int `json:"servicesCount"`
	VolumesCount      int `json:"volumesCount"`
}

// ConvertK8sDashboard converts a raw SDK dashboard model to a local model.
func ConvertK8sDashboard(raw *apimodels.KubernetesK8sDashboard) KubernetesDashboard {
	if raw == nil {
		return KubernetesDashboard{}
	}

	return KubernetesDashboard{
		ApplicationsCount: int(raw.ApplicationsCount),
		ConfigMapsCount:   int(raw.ConfigMapsCount),
		IngressesCount:    int(raw.IngressesCount),
		NamespacesCount:   int(raw.NamespacesCount),
		SecretsCount:      int(raw.SecretsCount),
		ServicesCount:     int(raw.ServicesCount),
		VolumesCount:      int(raw.VolumesCount),
	}
}

// KubernetesNamespace represents a Kubernetes namespace.
type KubernetesNamespace struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	CreationDate   string `json:"creationDate"`
	NamespaceOwner string `json:"namespaceOwner"`
	IsDefault      bool   `json:"isDefault"`
	IsSystem       bool   `json:"isSystem"`
}

// ConvertK8sNamespace converts a raw SDK namespace model to a local model.
func ConvertK8sNamespace(raw *apimodels.PortainerK8sNamespaceInfo) KubernetesNamespace {
	if raw == nil {
		return KubernetesNamespace{}
	}

	return KubernetesNamespace{
		ID:             raw.ID,
		Name:           raw.Name,
		CreationDate:   raw.CreationDate,
		NamespaceOwner: raw.NamespaceOwner,
		IsDefault:      raw.IsDefault,
		IsSystem:       raw.IsSystem,
	}
}
