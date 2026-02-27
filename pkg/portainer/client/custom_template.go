package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetCustomTemplates retrieves all custom templates from the Portainer server.
//
// Returns:
//   - A slice of CustomTemplate objects
//   - An error if the operation fails
func (c *PortainerClient) GetCustomTemplates() ([]models.CustomTemplate, error) {
	rawTemplates, err := c.cli.ListCustomTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to list custom templates: %w", err)
	}

	templates := make([]models.CustomTemplate, len(rawTemplates))
	for i, raw := range rawTemplates {
		templates[i] = models.ConvertCustomTemplateToLocal(raw)
	}

	return templates, nil
}

// GetCustomTemplate retrieves a specific custom template by ID.
//
// Parameters:
//   - id: The ID of the custom template
//
// Returns:
//   - A CustomTemplate object
//   - An error if the operation fails
func (c *PortainerClient) GetCustomTemplate(id int) (models.CustomTemplate, error) {
	raw, err := c.cli.GetCustomTemplate(int64(id))
	if err != nil {
		return models.CustomTemplate{}, fmt.Errorf("failed to get custom template: %w", err)
	}

	return models.ConvertCustomTemplateToLocal(raw), nil
}

// GetCustomTemplateFile retrieves the file content of a custom template.
//
// Parameters:
//   - id: The ID of the custom template
//
// Returns:
//   - The file content as a string
//   - An error if the operation fails
func (c *PortainerClient) GetCustomTemplateFile(id int) (string, error) {
	content, err := c.cli.GetCustomTemplateFile(int64(id))
	if err != nil {
		return "", fmt.Errorf("failed to get custom template file: %w", err)
	}

	return content, nil
}

// CreateCustomTemplate creates a new custom template on the Portainer server.
//
// Parameters:
//   - title: The title of the custom template
//   - description: The description of the custom template
//   - note: An optional note for the custom template
//   - logo: An optional logo URL for the custom template
//   - fileContent: The file content for the custom template
//   - platform: The platform type (1=linux, 2=windows)
//   - templateType: The template type (1=swarm, 2=compose, 3=kubernetes)
//
// Returns:
//   - The ID of the created custom template
//   - An error if the operation fails
func (c *PortainerClient) CreateCustomTemplate(title, description, note, logo, fileContent string, platform, templateType int) (int, error) {
	tType := int64(templateType)
	payload := &apimodels.CustomtemplatesCustomTemplateFromFileContentPayload{
		Title:       &title,
		Description: &description,
		FileContent: &fileContent,
		Type:        &tType,
		Note:        note,
		Logo:        logo,
		Platform:    int64(platform),
	}

	raw, err := c.cli.CreateCustomTemplate(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to create custom template: %w", err)
	}

	return int(raw.ID), nil
}

// DeleteCustomTemplate deletes a custom template from the Portainer server.
//
// Parameters:
//   - id: The ID of the custom template to delete
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteCustomTemplate(id int) error {
	err := c.cli.DeleteCustomTemplate(int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete custom template: %w", err)
	}

	return nil
}
