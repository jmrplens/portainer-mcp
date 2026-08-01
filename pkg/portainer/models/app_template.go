package models

import apimodels "github.com/portainer/client-api-go/v2/pkg/models"

// AppTemplate represents an application template in Portainer.
type AppTemplate struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        int      `json:"type"`
	Image       string   `json:"image,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Platform    string   `json:"platform,omitempty"`
	Logo        string   `json:"logo,omitempty"`
	Name        string   `json:"name,omitempty"`
	Note        string   `json:"note,omitempty"`
}

// ConvertToAppTemplate converts a raw SDK PortainerTemplate to the local AppTemplate model.
func ConvertToAppTemplate(raw *apimodels.PortainerTemplate) AppTemplate {
	if raw == nil {
		return AppTemplate{}
	}

	return AppTemplate{
		ID:          int(raw.ID),
		Title:       raw.Title,
		Description: raw.Description,
		Type:        int(raw.Type),
		Image:       raw.Image,
		Categories:  raw.Categories,
		Platform:    raw.Platform,
		Logo:        raw.Logo,
		Name:        raw.Name,
		Note:        raw.Note,
	}
}

// ConvertToAppTemplates converts a slice of raw SDK PortainerTemplate to local AppTemplate models.
func ConvertToAppTemplates(raw []*apimodels.PortainerTemplate) []AppTemplate {
	templates := make([]AppTemplate, 0, len(raw))
	for _, t := range raw {
		templates = append(templates, ConvertToAppTemplate(t))
	}
	return templates
}
