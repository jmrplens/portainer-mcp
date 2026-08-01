package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/endpoints"
)

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
