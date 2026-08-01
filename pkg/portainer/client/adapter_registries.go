package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/registries"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListRegistries lists all registries.
func (a *portainerAPIAdapter) ListRegistries() ([]*apimodels.PortainereeRegistry, error) {
	params := registries.NewRegistryListParams()
	resp, err := a.swagger.Registries.RegistryList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list registries: %w", err)
	}
	return resp.Payload, nil
}

// GetRegistryByID retrieves a registry by ID.
func (a *portainerAPIAdapter) GetRegistryByID(id int64) (*apimodels.PortainereeRegistry, error) {
	params := registries.NewRegistryInspectParams().WithID(id)
	resp, err := a.swagger.Registries.RegistryInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry: %w", err)
	}
	return resp.Payload, nil
}

// CreateRegistry creates a new registry.
func (a *portainerAPIAdapter) CreateRegistry(body *apimodels.RegistriesRegistryCreatePayload) (int64, error) {
	params := registries.NewRegistryCreateParams().WithBody(body)
	resp, err := a.swagger.Registries.RegistryCreate(params, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create registry: %w", err)
	}
	return resp.Payload.ID, nil
}

// UpdateRegistry updates an existing registry.
func (a *portainerAPIAdapter) UpdateRegistry(id int64, body *apimodels.RegistriesRegistryUpdatePayload) error {
	params := registries.NewRegistryUpdateParams().WithID(id).WithBody(body)
	_, err := a.swagger.Registries.RegistryUpdate(params, nil)
	if err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}
	return nil
}

// DeleteRegistry deletes a registry by ID.
func (a *portainerAPIAdapter) DeleteRegistry(id int64) error {
	params := registries.NewRegistryDeleteParams().WithID(id)
	_, err := a.swagger.Registries.RegistryDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete registry: %w", err)
	}
	return nil
}
