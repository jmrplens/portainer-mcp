package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetRegistries retrieves all registries from the Portainer server.
//
// Returns:
//   - A slice of Registry objects
//   - An error if the operation fails
func (c *PortainerClient) GetRegistries() ([]models.Registry, error) {
	rawRegistries, err := c.cli.ListRegistries()
	if err != nil {
		return nil, fmt.Errorf("failed to list registries: %w", err)
	}

	registries := make([]models.Registry, len(rawRegistries))
	for i, raw := range rawRegistries {
		registries[i] = models.ConvertRawRegistryToRegistry(raw)
	}

	return registries, nil
}

// GetRegistry retrieves a single registry by ID from the Portainer server.
//
// Parameters:
//   - id: The ID of the registry to retrieve
//
// Returns:
//   - A Registry object
//   - An error if the operation fails
func (c *PortainerClient) GetRegistry(id int) (models.Registry, error) {
	rawRegistry, err := c.cli.GetRegistryByID(int64(id))
	if err != nil {
		return models.Registry{}, fmt.Errorf("failed to get registry: %w", err)
	}

	return models.ConvertRawRegistryToRegistry(rawRegistry), nil
}

// CreateRegistry creates a new registry on the Portainer server.
//
// Parameters:
//   - name: The name of the registry
//   - registryType: The type of the registry (1=Quay, 2=Azure, 3=Custom, 4=Gitlab, 5=ProGet, 6=DockerHub, 7=ECR)
//   - url: The URL of the registry
//   - authentication: Whether the registry requires authentication
//   - username: The username for authentication
//   - password: The password for authentication
//   - baseURL: The base URL of the registry
//
// Returns:
//   - The ID of the created registry
//   - An error if the operation fails
func (c *PortainerClient) CreateRegistry(name string, registryType int, url string, authentication bool, username string, password string, baseURL string) (int, error) {
	regType := int64(registryType)
	body := &apimodels.RegistriesRegistryCreatePayload{
		Name:           &name,
		Type:           &regType,
		URL:            &url,
		Authentication: &authentication,
		Username:       username,
		Password:       password,
		BaseURL:        baseURL,
	}

	id, err := c.cli.CreateRegistry(body)
	if err != nil {
		return 0, fmt.Errorf("failed to create registry: %w", err)
	}

	return int(id), nil
}

// UpdateRegistry updates an existing registry on the Portainer server.
// It uses a GET-then-PUT pattern, fetching the current registry first
// and merging the provided fields with existing values.
//
// Parameters:
//   - id: The ID of the registry to update
//   - name: The new name (nil to keep existing)
//   - url: The new URL (nil to keep existing)
//   - authentication: The new authentication setting (nil to keep existing)
//   - username: The new username (nil to keep existing)
//   - password: The new password (nil to keep existing)
//   - baseURL: The new base URL (nil to keep existing)
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) UpdateRegistry(id int, name *string, url *string, authentication *bool, username *string, password *string, baseURL *string) error {
	existing, err := c.cli.GetRegistryByID(int64(id))
	if err != nil {
		return fmt.Errorf("failed to get registry for update: %w", err)
	}

	updatedName := existing.Name
	if name != nil {
		updatedName = *name
	}

	updatedURL := existing.URL
	if url != nil {
		updatedURL = *url
	}

	updatedAuth := existing.Authentication
	if authentication != nil {
		updatedAuth = *authentication
	}

	updatedUsername := existing.Username
	if username != nil {
		updatedUsername = *username
	}

	updatedPassword := existing.Password
	if password != nil {
		updatedPassword = *password
	}

	updatedBaseURL := existing.BaseURL
	if baseURL != nil {
		updatedBaseURL = *baseURL
	}

	body := &apimodels.RegistriesRegistryUpdatePayload{
		Name:           &updatedName,
		URL:            &updatedURL,
		Authentication: &updatedAuth,
		Username:       updatedUsername,
		Password:       updatedPassword,
		BaseURL:        updatedBaseURL,
	}

	err = c.cli.UpdateRegistry(int64(id), body)
	if err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	return nil
}

// DeleteRegistry deletes a registry from the Portainer server.
//
// Parameters:
//   - id: The ID of the registry to delete
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteRegistry(id int) error {
	err := c.cli.DeleteRegistry(int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete registry: %w", err)
	}

	return nil
}
