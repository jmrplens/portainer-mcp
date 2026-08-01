package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// CustomTemplate represents a simplified custom template for the MCP application.
type CustomTemplate struct {
	ID              int    `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Note            string `json:"note,omitempty"`
	Platform        int    `json:"platform"`
	Type            int    `json:"type"`
	Logo            string `json:"logo,omitempty"`
	CreatedByUserID int    `json:"created_by_user_id"`
}

// ConvertCustomTemplateToLocal converts a raw SDK custom template to a local CustomTemplate model.
//
// Parameters:
//   - raw: The raw SDK custom template
//
// Returns:
//   - A local CustomTemplate model
func ConvertCustomTemplateToLocal(raw *apimodels.PortainereeCustomTemplate) CustomTemplate {
	if raw == nil {
		return CustomTemplate{}
	}

	return CustomTemplate{
		ID:              int(raw.ID),
		Title:           raw.Title,
		Description:     raw.Description,
		Note:            raw.Note,
		Platform:        int(raw.Platform),
		Type:            int(raw.Type),
		Logo:            raw.Logo,
		CreatedByUserID: int(raw.CreatedByUserID),
	}
}
