package models

import (
	"io"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// DockerContainerStats represents container status counts for a Docker environment.
type DockerContainerStats struct {
	Healthy   int `json:"healthy"`
	Running   int `json:"running"`
	Stopped   int `json:"stopped"`
	Total     int `json:"total"`
	Unhealthy int `json:"unhealthy"`
}

// DockerImagesCounters represents image statistics for a Docker environment.
type DockerImagesCounters struct {
	Size  int64 `json:"size"`
	Total int   `json:"total"`
}

// DockerDashboard represents the Docker dashboard data for a Portainer environment.
type DockerDashboard struct {
	Containers DockerContainerStats `json:"containers"`
	Images     DockerImagesCounters `json:"images"`
	Networks   int                  `json:"networks"`
	Services   int                  `json:"services"`
	Stacks     int                  `json:"stacks"`
	Volumes    int                  `json:"volumes"`
}

// ConvertDockerDashboardResponse converts the raw API DockerDashboardResponse to a local DockerDashboard model.
func ConvertDockerDashboardResponse(raw *apimodels.DockerDashboardResponse) DockerDashboard {
	if raw == nil {
		return DockerDashboard{}
	}

	dashboard := DockerDashboard{
		Networks: int(raw.Networks),
		Services: int(raw.Services),
		Stacks:   int(raw.Stacks),
		Volumes:  int(raw.Volumes),
	}

	if raw.Containers != nil {
		dashboard.Containers = DockerContainerStats{
			Healthy:   int(raw.Containers.Healthy),
			Running:   int(raw.Containers.Running),
			Stopped:   int(raw.Containers.Stopped),
			Total:     int(raw.Containers.Total),
			Unhealthy: int(raw.Containers.Unhealthy),
		}
	}

	if raw.Images != nil {
		dashboard.Images = DockerImagesCounters{
			Size:  raw.Images.Size,
			Total: int(raw.Images.Total),
		}
	}

	return dashboard
}

// DockerProxyRequestOptions represents the options for a Docker API request to a specific Portainer environment.
type DockerProxyRequestOptions struct {
	// EnvironmentID is the ID of the environment to proxy the request to.
	EnvironmentID int
	// Method is the HTTP method to use (GET, POST, PUT, DELETE, etc.).
	Method string
	// Path is the Docker API endpoint path to proxy to (e.g., "/containers/json"). Must include the leading slash.
	Path string
	// QueryParams is a map of query parameters to include in the request URL.
	QueryParams map[string]string
	// Headers is a map of headers to include in the request.
	Headers map[string]string
	// Body is the request body to send (set it to nil for requests that don't have a body).
	Body io.Reader
}
