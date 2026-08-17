// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package custom_templates

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// customTemplateCreateRepository is the generated handler for operation CustomTemplateCreateRepository.
func customTemplateCreateRepository(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.CustomTemplateCreateRepositoryJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateRepository: parse request body: %w", err)
	}
	resp, err := c.API.CustomTemplateCreateRepositoryWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateRepository: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateRepository: %w", err)
	}
	return redactCustomTemplateCreateRepository(resp.JSON200), nil
}

// customTemplateCreateString is the generated handler for operation CustomTemplateCreateString.
func customTemplateCreateString(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.CustomTemplateCreateStringJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateString: parse request body: %w", err)
	}
	resp, err := c.API.CustomTemplateCreateStringWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateString: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateString: %w", err)
	}
	return redactCustomTemplateCreateString(resp.JSON200), nil
}

// customTemplateDelete is the generated handler for operation CustomTemplateDelete.
func customTemplateDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateDelete: parse input: %w", err)
	}
	resp, err := c.API.CustomTemplateDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// customTemplateFile is the generated handler for operation CustomTemplateFile.
func customTemplateFile(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateFile: parse input: %w", err)
	}
	resp, err := c.API.CustomTemplateFileWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateFile: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateFile: %w", err)
	}
	return resp.JSON200, nil
}

// customTemplateGitFetch is the generated handler for operation CustomTemplateGitFetch.
func customTemplateGitFetch(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateGitFetchInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateGitFetch: parse input: %w", err)
	}
	resp, err := c.API.CustomTemplateGitFetchWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateGitFetch: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateGitFetch: %w", err)
	}
	return resp.JSON200, nil
}

// customTemplateInspect is the generated handler for operation CustomTemplateInspect.
func customTemplateInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateInspect: parse input: %w", err)
	}
	resp, err := c.API.CustomTemplateInspectWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateInspect: %w", err)
	}
	return redactCustomTemplateInspect(resp.JSON200), nil
}

// customTemplateList is the generated handler for operation CustomTemplateList.
func customTemplateList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.CustomTemplateListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("CustomTemplateList: parse query parameters: %w", err)
	}
	resp, err := c.API.CustomTemplateListWithResponse(ctx, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateList: %w", err)
	}
	return redactCustomTemplateList(resp.JSON200), nil
}

// customTemplateUpdate is the generated handler for operation CustomTemplateUpdate.
func customTemplateUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateUpdate: parse input: %w", err)
	}
	var body apigen.CustomTemplateUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("CustomTemplateUpdate: parse request body: %w", err)
	}
	resp, err := c.API.CustomTemplateUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateUpdate: %w", err)
	}
	return redactCustomTemplateUpdate(resp.JSON200), nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.create_repository", Domain: "custom_templates", OperationID: "CustomTemplateCreateRepository",
			Title:       "Create a custom template",
			Description: "Create a custom template.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     customTemplateCreateRepository,
			Input:       customTemplateCreateRepositoryInput{},
		}, narrative("CustomTemplateCreateRepository")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.create_string", Domain: "custom_templates", OperationID: "CustomTemplateCreateString",
			Title:       "Create a custom template",
			Description: "Create a custom template.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     customTemplateCreateString,
			Input:       customTemplateCreateStringInput{},
		}, narrative("CustomTemplateCreateString")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.delete", Domain: "custom_templates", OperationID: "CustomTemplateDelete",
			Title:       "Remove a template",
			Description: "Remove a template.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     customTemplateDelete,
			Input:       customTemplateDeleteInput{},
		}, narrative("CustomTemplateDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.file", Domain: "custom_templates", OperationID: "CustomTemplateFile",
			Title:       "Get Template stack file content.",
			Description: "Retrieve the content of the Stack file for the specified custom template",
			Edition:     edition.CE,
			Handler:     customTemplateFile,
			Input:       customTemplateFileInput{},
		}, narrative("CustomTemplateFile")),
		// Destructive is a hand override, not a generated flag: the verb-derived
		// rule reads PUT as Mutating+Idempotent and stops there, and
		// suspectDangerMismatch's seven keywords match neither this path nor
		// this operationId, so the scaffold run raised nothing for this domain
		// at all. It is destructive all the same, and the criterion is sharper
		// than "the caller cannot see what it loses" — CustomTemplateUpdate
		// silently clears an omitted optional field too, and is deliberately
		// not marked destructive. The distinction is what the request is
		// *capable* of expressing: update's payload has a field for every part
		// of the template it overwrites, so a caller that reads the template
		// first (custom_templates.inspect, custom_templates.file) can state the
		// intended end state in full and lose nothing it did not choose to
		// drop. This request has no such field and cannot acquire one: its
		// entire payload is a template id, the replacement content comes from a
		// third party (the git remote) at a version the caller never named, and
		// Portainer keeps no previous version to restore. No amount of care by
		// the caller makes the loss expressible, which is the property
		// system.upgrade's own hand override marks, and what makes the surfaces
		// render this as [destructive] rather than [mutating] next to an action
		// whose name reads like a query.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.git_fetch", Domain: "custom_templates", OperationID: "CustomTemplateGitFetch",
			Title:       "Fetch the latest config file content based on custom template's git repository configuration",
			Description: "Retrieve details about a template created from git repository method.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     customTemplateGitFetch,
			Input:       customTemplateGitFetchInput{},
		}, narrative("CustomTemplateGitFetch")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.inspect", Domain: "custom_templates", OperationID: "CustomTemplateInspect",
			Title:       "Inspect a custom template",
			Description: "Retrieve details about a template.",
			Edition:     edition.CE,
			Handler:     customTemplateInspect,
			Input:       customTemplateInspectInput{},
		}, narrative("CustomTemplateInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.list", Domain: "custom_templates", OperationID: "CustomTemplateList",
			Title:       "List available custom templates",
			Description: "List available custom templates.",
			Edition:     edition.CE,
			Handler:     customTemplateList,
			Input:       customTemplateListInput{},
		}, narrative("CustomTemplateList")),
		// Not Destructive, and that is a ruling rather than the default being
		// left alone: this is a whole-object replace, but every field it
		// overwrites the caller supplies in the same request, the template
		// survives at the same id, and registries.update — the identical
		// whole-object PUT already shipped — is not marked destructive either.
		// Flagging every replace destructive would leave the flag unable to
		// separate an edit from a deletion, which is the distinction the
		// surfaces render it for. The hazard that remains, that an omitted
		// optional field is silently cleared, is a narrative's job, and
		// narrative("CustomTemplateUpdate") states it.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "custom_templates.update", Domain: "custom_templates", OperationID: "CustomTemplateUpdate",
			Title:       "Update a template",
			Description: "Update a template.",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     customTemplateUpdate,
			Input:       customTemplateUpdateInput{},
		}, narrative("CustomTemplateUpdate")),
	}
}
