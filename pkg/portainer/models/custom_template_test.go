package models

import (
	"testing"

	"github.com/portainer/client-api-go/v2/pkg/models"
)

// TestConvertCustomTemplateToLocal verifies the ConvertCustomTemplateToLocal model conversion function.
func TestConvertCustomTemplateToLocal(t *testing.T) {
	tests := []struct {
		name string
		raw  *models.PortainereeCustomTemplate
		want CustomTemplate
	}{
		{
			name: "full custom template conversion",
			raw: &models.PortainereeCustomTemplate{
				ID:              1,
				Title:           "My Template",
				Description:     "A test template",
				Note:            "Some note",
				Platform:        1,
				Type:            2,
				Logo:            "https://example.com/logo.png",
				CreatedByUserID: 5,
			},
			want: CustomTemplate{
				ID:              1,
				Title:           "My Template",
				Description:     "A test template",
				Note:            "Some note",
				Platform:        1,
				Type:            2,
				Logo:            "https://example.com/logo.png",
				CreatedByUserID: 5,
			},
		},
		{
			name: "custom template with empty optional fields",
			raw: &models.PortainereeCustomTemplate{
				ID:          2,
				Title:       "Minimal Template",
				Description: "Minimal",
				Platform:    1,
				Type:        1,
			},
			want: CustomTemplate{
				ID:          2,
				Title:       "Minimal Template",
				Description: "Minimal",
				Platform:    1,
				Type:        1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertCustomTemplateToLocal(tt.raw)

			if got.ID != tt.want.ID {
				t.Errorf("ID = %d, want %d", got.ID, tt.want.ID)
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Note != tt.want.Note {
				t.Errorf("Note = %q, want %q", got.Note, tt.want.Note)
			}
			if got.Platform != tt.want.Platform {
				t.Errorf("Platform = %d, want %d", got.Platform, tt.want.Platform)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %d, want %d", got.Type, tt.want.Type)
			}
			if got.Logo != tt.want.Logo {
				t.Errorf("Logo = %q, want %q", got.Logo, tt.want.Logo)
			}
			if got.CreatedByUserID != tt.want.CreatedByUserID {
				t.Errorf("CreatedByUserID = %d, want %d", got.CreatedByUserID, tt.want.CreatedByUserID)
			}
		})
	}
}
