// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package endpoints

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// agentVersions is the generated handler for operation AgentVersions.
func agentVersions(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.AgentVersionsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("AgentVersions: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("AgentVersions: %w", err)
	}
	return resp.JSON200, nil
}

// endpointCreateGlobalKey is the generated handler for operation EndpointCreateGlobalKey.
func endpointCreateGlobalKey(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.EndpointCreateGlobalKeyWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("EndpointCreateGlobalKey: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointCreateGlobalKey: %w", err)
	}
	return resp.JSON200, nil
}

// endpointDelete is the generated handler for operation EndpointDelete.
func endpointDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointDelete: parse input: %w", err)
	}
	resp, err := c.API.EndpointDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("EndpointDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointDeleteBatch is the generated handler for operation EndpointDeleteBatch.
func endpointDeleteBatch(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.EndpointDeleteBatchJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointDeleteBatch: parse request body: %w", err)
	}
	resp, err := c.API.EndpointDeleteBatchWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointDeleteBatch: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointDeleteBatch: %w", err)
	}
	return resp.JSON207, nil
}

// endpointDockerhubStatus is the generated handler for operation EndpointDockerhubStatus.
func endpointDockerhubStatus(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointDockerhubStatusInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointDockerhubStatus: parse input: %w", err)
	}
	resp, err := c.API.EndpointDockerhubStatusWithResponse(ctx, params.ID, params.RegistryID)
	if err != nil {
		return nil, fmt.Errorf("EndpointDockerhubStatus: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointDockerhubStatus: %w", err)
	}
	return resp.JSON200, nil
}

// endpointForceUpdateService is the generated handler for operation EndpointForceUpdateService.
func endpointForceUpdateService(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointForceUpdateServiceInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointForceUpdateService: parse input: %w", err)
	}
	var body apigen.EndpointForceUpdateServiceJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointForceUpdateService: parse request body: %w", err)
	}
	resp, err := c.API.EndpointForceUpdateServiceWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointForceUpdateService: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointForceUpdateService: %w", err)
	}
	return resp.JSON200, nil
}

// endpointInspect is the generated handler for operation EndpointInspect.
func endpointInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointInspect: parse input: %w", err)
	}
	var queryParams apigen.EndpointInspectParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("EndpointInspect: parse query parameters: %w", err)
	}
	resp, err := c.API.EndpointInspectWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("EndpointInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointInspect: %w", err)
	}
	return redactEndpointInspect(resp.JSON200), nil
}

// endpointMTLSAgentCertificateError is the generated handler for operation EndpointMTLSAgentCertificateError.
func endpointMTLSAgentCertificateError(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointMTLSAgentCertificateErrorInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointMTLSAgentCertificateError: parse input: %w", err)
	}
	resp, err := c.API.EndpointMTLSAgentCertificateErrorWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("EndpointMTLSAgentCertificateError: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointMTLSAgentCertificateError: %w", err)
	}
	return redactEndpointMTLSAgentCertificateError(resp.JSON200), nil
}

// endpointMTLSCertificate is the generated handler for operation EndpointMTLSCertificate.
func endpointMTLSCertificate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointMTLSCertificateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointMTLSCertificate: parse input: %w", err)
	}
	resp, err := c.API.EndpointMTLSCertificateWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("EndpointMTLSCertificate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointMTLSCertificate: %w", err)
	}
	return redactEndpointMTLSCertificate(resp.JSON200), nil
}

// endpointRegistriesList is the generated handler for operation EndpointRegistriesList.
func endpointRegistriesList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointRegistriesListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointRegistriesList: parse input: %w", err)
	}
	var queryParams apigen.EndpointRegistriesListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("EndpointRegistriesList: parse query parameters: %w", err)
	}
	resp, err := c.API.EndpointRegistriesListWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("EndpointRegistriesList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointRegistriesList: %w", err)
	}
	return redactEndpointRegistriesList(resp.JSON200), nil
}

// endpointRegistryAccess is the generated handler for operation EndpointRegistryAccess.
func endpointRegistryAccess(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointRegistryAccessInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointRegistryAccess: parse input: %w", err)
	}
	var body apigen.EndpointRegistryAccessJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointRegistryAccess: parse request body: %w", err)
	}
	resp, err := c.API.EndpointRegistryAccessWithResponse(ctx, params.ID, params.RegistryID, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointRegistryAccess: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointRegistryAccess: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointSetPolicyStatuses is the generated handler for operation EndpointSetPolicyStatuses.
func endpointSetPolicyStatuses(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointSetPolicyStatusesInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointSetPolicyStatuses: parse input: %w", err)
	}
	var body apigen.EndpointSetPolicyStatusesJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointSetPolicyStatuses: parse request body: %w", err)
	}
	resp, err := c.API.EndpointSetPolicyStatusesWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointSetPolicyStatuses: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointSetPolicyStatuses: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointSnapshot is the generated handler for operation EndpointSnapshot.
func endpointSnapshot(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointSnapshotInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointSnapshot: parse input: %w", err)
	}
	resp, err := c.API.EndpointSnapshotWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("EndpointSnapshot: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointSnapshot: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointSnapshots is the generated handler for operation EndpointSnapshots.
func endpointSnapshots(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.EndpointSnapshotsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("EndpointSnapshots: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointSnapshots: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointSummaryCounts is the generated handler for operation EndpointSummaryCounts.
func endpointSummaryCounts(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.EndpointSummaryCountsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("EndpointSummaryCounts: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointSummaryCounts: %w", err)
	}
	return resp.JSON200, nil
}

// endpointUpdateRelations is the generated handler for operation EndpointUpdateRelations.
func endpointUpdateRelations(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.EndpointUpdateRelationsJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointUpdateRelations: parse request body: %w", err)
	}
	resp, err := c.API.EndpointUpdateRelationsWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointUpdateRelations: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointUpdateRelations: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// snapshotInspect is the generated handler for operation SnapshotInspect.
func snapshotInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params snapshotInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("SnapshotInspect: parse input: %w", err)
	}
	resp, err := c.API.SnapshotInspectWithResponse(ctx, params.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("SnapshotInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("SnapshotInspect: %w", err)
	}
	return resp.JSON200, nil
}

// trustEdgeEndpoints is the generated handler for operation TrustEdgeEndpoints.
func trustEdgeEndpoints(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.TrustEdgeEndpointsJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("TrustEdgeEndpoints: parse request body: %w", err)
	}
	resp, err := c.API.TrustEdgeEndpointsWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("TrustEdgeEndpoints: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TrustEdgeEndpoints: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.agent_versions", Domain: "endpoints", OperationID: "AgentVersions",
			Title:       "List agent versions",
			Description: "List all agent versions based on the current user authorizations and query parameters.",
			Edition:     edition.EE,
			Handler:     agentVersions,
		}, narrative("AgentVersions")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.create_global_key", Domain: "endpoints", OperationID: "EndpointCreateGlobalKey",
			Title:       "Create or retrieve the endpoint for an EdgeID",
			Description: "Create or retrieve the endpoint for an EdgeID",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     endpointCreateGlobalKey,
		}, narrative("EndpointCreateGlobalKey")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.delete", Domain: "endpoints", OperationID: "EndpointDelete",
			Title:       "Remove an environment",
			Description: "Remove the environment associated to the specified identifier and optionally clean-up associated resources.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     endpointDelete,
			Input:       endpointDeleteInput{},
		}, narrative("EndpointDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.delete_batch", Domain: "endpoints", OperationID: "EndpointDeleteBatch",
			Title:       "Remove multiple environments",
			Description: "Remove multiple environments and optionally clean-up associated resources.",
			Edition:     edition.CE,
			Mutating:    true,
			// Hand-set, and one of the two this domain needs: the route is a
			// POST, so the verb-only rule derives Destructive: false, and
			// cmd/gen_action_inputs's own dangerMismatchWarnings flags exactly
			// that — the path and operationId both say "delete" while the verb
			// does not. It removes environments and cannot be undone.
			Destructive: true,
			Handler:     endpointDeleteBatch,
			Input:       endpointDeleteBatchInput{},
		}, narrative("EndpointDeleteBatch")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.dockerhub_status", Domain: "endpoints", OperationID: "EndpointDockerhubStatus",
			Title:       "fetch docker pull limits",
			Description: "get docker pull limits for a docker hub registry in the environment",
			Edition:     edition.CE,
			Handler:     endpointDockerhubStatus,
			Input:       endpointDockerhubStatusInput{},
		}, narrative("EndpointDockerhubStatus")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.force_update_service", Domain: "endpoints", OperationID: "EndpointForceUpdateService",
			Title:       "force update a docker service",
			Description: "force update a docker service",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointForceUpdateService,
			Input:       endpointForceUpdateServiceInput{},
		}, narrative("EndpointForceUpdateService")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.inspect", Domain: "endpoints", OperationID: "EndpointInspect",
			Title:       "Inspect an environment(endpoint)",
			Description: "Retrieve details about an environment(endpoint).",
			Edition:     edition.CE,
			Handler:     endpointInspect,
			Input:       endpointInspectInput{},
		}, narrative("EndpointInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.mtls_agent_certificate_error", Domain: "endpoints", OperationID: "EndpointMTLSAgentCertificateError",
			Title:       "Get an agent(endpoint) mTLS certificate",
			Description: "Retrieve the mTLS certificate of an environment(endpoint).",
			Edition:     edition.EE,
			Handler:     endpointMTLSAgentCertificateError,
			Input:       endpointMTLSAgentCertificateErrorInput{},
		}, narrative("EndpointMTLSAgentCertificateError")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.mtls_certificate", Domain: "endpoints", OperationID: "EndpointMTLSCertificate",
			Title:       "Get an environment(endpoint) mTLS certificate",
			Description: "Retrieve the mTLS certificate of an environment(endpoint).",
			Edition:     edition.EE,
			Handler:     endpointMTLSCertificate,
			Input:       endpointMTLSCertificateInput{},
		}, narrative("EndpointMTLSCertificate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.registries_list", Domain: "endpoints", OperationID: "EndpointRegistriesList",
			Title:       "List Registries on environment",
			Description: "List all registries based on the current user authorizations in current environment.",
			Edition:     edition.CE,
			Handler:     endpointRegistriesList,
			Input:       endpointRegistriesListInput{},
		}, narrative("EndpointRegistriesList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.registry_access", Domain: "endpoints", OperationID: "EndpointRegistryAccess",
			Title:       "update registry access for environment",
			Description: "update registry access for environment",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointRegistryAccess,
			Input:       endpointRegistryAccessInput{},
		}, narrative("EndpointRegistryAccess")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.set_policy_statuses", Domain: "endpoints", OperationID: "EndpointSetPolicyStatuses",
			Title:       "Report per-policy status from an edge agent",
			Description: "environment(endpoint) for edge agent to report back per-policy reconciliation statuses",
			Edition:     edition.EE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointSetPolicyStatuses,
			Input:       endpointSetPolicyStatusesInput{},
		}, narrative("EndpointSetPolicyStatuses")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.snapshot", Domain: "endpoints", OperationID: "EndpointSnapshot",
			Title:       "Snapshots an environment(endpoint)",
			Description: "Snapshots an environment(endpoint)",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     endpointSnapshot,
			Input:       endpointSnapshotInput{},
		}, narrative("EndpointSnapshot")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.snapshot_all", Domain: "endpoints", OperationID: "EndpointSnapshots",
			Title:       "Snapshot all environment(endpoint)",
			Description: "Snapshot all environments(endpoints)",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     endpointSnapshots,
		}, narrative("EndpointSnapshots")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.summary_counts", Domain: "endpoints", OperationID: "EndpointSummaryCounts",
			Title:       "Get environment summary counts",
			Description: "Returns counts of environments by status (up, down, outdated) and ungrouped environments (unassigned), plus breakdowns by group, type, and health.",
			Edition:     edition.CE,
			Handler:     endpointSummaryCounts,
		}, narrative("EndpointSummaryCounts")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.update_relations", Domain: "endpoints", OperationID: "EndpointUpdateRelations",
			Title:       "Update relations for a list of environments",
			Description: "Update relations for a list of environments\nEdge groups, tags and environment group can be updated.",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointUpdateRelations,
			Input:       endpointUpdateRelationsInput{},
		}, narrative("EndpointUpdateRelations")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.snapshot_inspect", Domain: "endpoints", OperationID: "SnapshotInspect",
			Title:       "Fetch latest snapshot of environment",
			Description: "Fetch latest snapshot of environment",
			Edition:     edition.EE,
			Handler:     snapshotInspect,
			Input:       snapshotInspectInput{},
		}, narrative("SnapshotInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.trust_edge_endpoints", Domain: "endpoints", OperationID: "TrustEdgeEndpoints",
			Title:       "Associate one or more Edge environments in the waiting room to environments",
			Description: "Associate one or more Edge environments, currently in the waiting room, with Portainer environments. This action effectively grants trust to these environments.",
			Edition:     edition.EE,
			Mutating:    true,
			Handler:     trustEdgeEndpoints,
			Input:       trustEdgeEndpointsInput{},
		}, narrative("TrustEdgeEndpoints")),
	}
}
