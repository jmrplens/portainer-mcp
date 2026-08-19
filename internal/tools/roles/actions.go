// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package roles

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// roleList is the generated handler for operation RoleList.
func roleList(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.RoleListWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("RoleList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RoleList: %w", err)
	}
	return resp.JSON200, nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "roles.list", Domain: "roles", OperationID: "RoleList",
			Title:       "List roles",
			Description: "List all roles available for use",
			Edition:     edition.CE,
			Handler:     roleList,
		}, narrative("RoleList")),
	}
}
