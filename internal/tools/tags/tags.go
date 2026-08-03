// Package tags declares the Portainer environment-tag actions.
//
// Three actions: list, create, delete. It is the smallest domain in the
// catalog that includes a genuinely destructive action, which makes it the
// pilot for that classification — tags.delete removes a tag outright, with no
// softer alternative the way some domains offer a stop/disable short of
// deletion.
package tags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every tag action.
func Specs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		{
			Name: "tags.list", Domain: "tags", OperationID: "TagList",
			Title:       "List tags",
			Description: "Returns every environment tag defined on this Portainer server.",
			Edition:     edition.CE,
			Handler:     tagList,
		},
		{
			Name: "tags.create", Domain: "tags", OperationID: "TagCreate",
			Title:       "Create a tag",
			Description: "Creates a new environment tag with the given name.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     tagCreate,
			Input:       tagCreateInput{},
		},
		{
			Name: "tags.delete", Domain: "tags", OperationID: "TagDelete",
			Title:       "Delete a tag",
			Description: "Permanently removes a tag, unassigning it from every environment and environment group that carries it. This cannot be undone.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Handler:     tagDelete,
			Input:       tagDeleteInput{},
		},
	}
}

func tagList(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.TagListWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("tags list: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("tags list: %w", err)
	}
	return resp.JSON200, nil
}

// tagCreateInput is the parameter shape for tags.create.
type tagCreateInput struct {
	// Name is the tag's display name.
	Name string `json:"name"`
}

func tagCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params tagCreateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("tags create: parse input: %w", err)
	}
	if params.Name == "" {
		return nil, fmt.Errorf("tags create: name is required")
	}

	resp, err := c.API.TagCreateWithResponse(ctx, apigen.TagCreateJSONRequestBody{Name: params.Name})
	if err != nil {
		return nil, fmt.Errorf("tags create: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("tags create: %w", err)
	}
	return resp.JSON200, nil
}

// tagDeleteInput is the parameter shape for tags.delete.
type tagDeleteInput struct {
	// ID is the tag identifier to remove.
	ID int `json:"id"`
}

func tagDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params tagDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("tags delete: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("tags delete: id must be a positive integer, got %d", params.ID)
	}

	resp, err := c.API.TagDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("tags delete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("tags delete: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID}, nil
}
