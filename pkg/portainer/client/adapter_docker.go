package client

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// GetDockerDashboard retrieves the Docker dashboard data for a specific environment.
// Uses raw HTTP GET because the SDK sends POST but newer Portainer versions require GET.
func (a *portainerAPIAdapter) GetDockerDashboard(environmentId int64) (*apimodels.DockerDashboardResponse, error) {
	op := &runtime.ClientOperation{
		ID:                 "DockerDashboard",
		Method:             "GET",
		PathPattern:        fmt.Sprintf("/docker/%d/dashboard", environmentId),
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{a.scheme},
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
