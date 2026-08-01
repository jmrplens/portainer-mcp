package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/tags"
)

// DeleteTag deletes a tag by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteTag(id int64) error {
	params := tags.NewTagDeleteParams().WithID(id)
	_, err := a.swagger.Tags.TagDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}
