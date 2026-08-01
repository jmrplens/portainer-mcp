package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/custom_templates"
	"github.com/portainer/client-api-go/v2/pkg/client/templates"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListCustomTemplates lists all custom templates.
func (a *portainerAPIAdapter) ListCustomTemplates() ([]*apimodels.PortainereeCustomTemplate, error) {
	params := custom_templates.NewCustomTemplateListParams()
	resp, err := a.swagger.CustomTemplates.CustomTemplateList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list custom templates: %w", err)
	}
	return resp.Payload, nil
}

// GetCustomTemplate retrieves a custom template by ID.
func (a *portainerAPIAdapter) GetCustomTemplate(id int64) (*apimodels.PortainereeCustomTemplate, error) {
	params := custom_templates.NewCustomTemplateInspectParams().WithID(id)
	resp, err := a.swagger.CustomTemplates.CustomTemplateInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom template: %w", err)
	}
	return resp.Payload, nil
}

// GetCustomTemplateFile retrieves the file content of a custom template.
func (a *portainerAPIAdapter) GetCustomTemplateFile(id int64) (string, error) {
	params := custom_templates.NewCustomTemplateFileParams().WithID(id)
	resp, err := a.swagger.CustomTemplates.CustomTemplateFile(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get custom template file: %w", err)
	}
	return resp.Payload.FileContent, nil
}

// CreateCustomTemplate creates a new custom template from file content.
func (a *portainerAPIAdapter) CreateCustomTemplate(payload *apimodels.CustomtemplatesCustomTemplateFromFileContentPayload) (*apimodels.PortainereeCustomTemplate, error) {
	params := custom_templates.NewCustomTemplateCreateStringParams().WithBody(payload)
	resp, err := a.swagger.CustomTemplates.CustomTemplateCreateString(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom template: %w", err)
	}
	return resp.Payload, nil
}

// DeleteCustomTemplate deletes a custom template by ID.
func (a *portainerAPIAdapter) DeleteCustomTemplate(id int64) error {
	params := custom_templates.NewCustomTemplateDeleteParams().WithID(id)
	_, err := a.swagger.CustomTemplates.CustomTemplateDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete custom template: %w", err)
	}
	return nil
}

// ListAppTemplates lists all application templates.
func (a *portainerAPIAdapter) ListAppTemplates() ([]*apimodels.PortainerTemplate, error) {
	params := templates.NewTemplateListParams()
	resp, err := a.swagger.Templates.TemplateList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list app templates: %w", err)
	}
	return resp.Payload.Templates, nil
}

// GetAppTemplateFile retrieves the file content of an application template.
func (a *portainerAPIAdapter) GetAppTemplateFile(id int64) (string, error) {
	params := templates.NewTemplateFileParams().WithID(id)
	resp, err := a.swagger.Templates.TemplateFile(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get app template file: %w", err)
	}
	return resp.Payload.FileContent, nil
}
