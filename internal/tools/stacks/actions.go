// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package stacks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// edgeStackWebhookInvoke is the generated handler for operation EdgeStackWebhookInvoke.
func edgeStackWebhookInvoke(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params edgeStackWebhookInvokeInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EdgeStackWebhookInvoke: parse input: %w", err)
	}
	resp, err := c.API.EdgeStackWebhookInvokeWithResponse(ctx, params.WebhookID)
	if err != nil {
		return nil, fmt.Errorf("EdgeStackWebhookInvoke: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EdgeStackWebhookInvoke: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// stackAssociate is the generated handler for operation StackAssociate.
func stackAssociate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackAssociateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackAssociate: parse input: %w", err)
	}
	var queryParams apigen.StackAssociateParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackAssociate: parse query parameters: %w", err)
	}
	resp, err := c.API.StackAssociateWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackAssociate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackAssociate: %w", err)
	}
	return redactStackAssociate(resp.JSON200), nil
}

// stackConvert is the generated handler for operation StackConvert.
func stackConvert(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackConvertInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackConvert: parse input: %w", err)
	}
	var body apigen.StackConvertJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackConvert: parse request body: %w", err)
	}
	resp, err := c.API.StackConvertWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("StackConvert: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackConvert: %w", err)
	}
	return resp.JSON200, nil
}

// stackCreateDockerStandaloneRepository is the generated handler for operation StackCreateDockerStandaloneRepository.
func stackCreateDockerStandaloneRepository(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateDockerStandaloneRepositoryParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneRepository: parse query parameters: %w", err)
	}
	var body apigen.StackCreateDockerStandaloneRepositoryJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneRepository: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateDockerStandaloneRepositoryWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneRepository: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneRepository: %w", err)
	}
	return redactStackCreateDockerStandaloneRepository(resp.JSON200), nil
}

// stackCreateDockerStandaloneString is the generated handler for operation StackCreateDockerStandaloneString.
func stackCreateDockerStandaloneString(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateDockerStandaloneStringParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneString: parse query parameters: %w", err)
	}
	var body apigen.StackCreateDockerStandaloneStringJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneString: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateDockerStandaloneStringWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneString: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneString: %w", err)
	}
	return redactStackCreateDockerStandaloneString(resp.JSON200), nil
}

// stackCreateDockerSwarmRepository is the generated handler for operation StackCreateDockerSwarmRepository.
func stackCreateDockerSwarmRepository(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateDockerSwarmRepositoryParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmRepository: parse query parameters: %w", err)
	}
	var body apigen.StackCreateDockerSwarmRepositoryJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmRepository: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateDockerSwarmRepositoryWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmRepository: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmRepository: %w", err)
	}
	return redactStackCreateDockerSwarmRepository(resp.JSON200), nil
}

// stackCreateDockerSwarmString is the generated handler for operation StackCreateDockerSwarmString.
func stackCreateDockerSwarmString(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateDockerSwarmStringParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmString: parse query parameters: %w", err)
	}
	var body apigen.StackCreateDockerSwarmStringJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmString: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateDockerSwarmStringWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmString: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmString: %w", err)
	}
	return redactStackCreateDockerSwarmString(resp.JSON200), nil
}

// stackCreateKubernetesFile is the generated handler for operation StackCreateKubernetesFile.
func stackCreateKubernetesFile(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateKubernetesFileParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesFile: parse query parameters: %w", err)
	}
	var body apigen.StackCreateKubernetesFileJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesFile: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateKubernetesFileWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesFile: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesFile: %w", err)
	}
	return resp.JSON200, nil
}

// stackCreateKubernetesGit is the generated handler for operation StackCreateKubernetesGit.
func stackCreateKubernetesGit(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateKubernetesGitParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesGit: parse query parameters: %w", err)
	}
	var body apigen.StackCreateKubernetesGitJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesGit: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateKubernetesGitWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesGit: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesGit: %w", err)
	}
	return resp.JSON200, nil
}

// stackCreateKubernetesURL is the generated handler for operation StackCreateKubernetesUrl.
func stackCreateKubernetesURL(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackCreateKubernetesUrlParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesUrl: parse query parameters: %w", err)
	}
	var body apigen.StackCreateKubernetesUrlJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesUrl: parse request body: %w", err)
	}
	resp, err := c.API.StackCreateKubernetesUrlWithResponse(ctx, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesUrl: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateKubernetesUrl: %w", err)
	}
	return resp.JSON200, nil
}

// stackDelete is the generated handler for operation StackDelete.
func stackDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackDelete: parse input: %w", err)
	}
	var queryParams apigen.StackDeleteParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackDelete: parse query parameters: %w", err)
	}
	resp, err := c.API.StackDeleteWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// stackDeleteKubernetesByName is the generated handler for operation StackDeleteKubernetesByName.
func stackDeleteKubernetesByName(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackDeleteKubernetesByNameInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackDeleteKubernetesByName: parse input: %w", err)
	}
	var queryParams apigen.StackDeleteKubernetesByNameParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackDeleteKubernetesByName: parse query parameters: %w", err)
	}
	resp, err := c.API.StackDeleteKubernetesByNameWithResponse(ctx, params.Name, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackDeleteKubernetesByName: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackDeleteKubernetesByName: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// stackFileInspect is the generated handler for operation StackFileInspect.
func stackFileInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackFileInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackFileInspect: parse input: %w", err)
	}
	var queryParams apigen.StackFileInspectParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackFileInspect: parse query parameters: %w", err)
	}
	resp, err := c.API.StackFileInspectWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackFileInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackFileInspect: %w", err)
	}
	return resp.JSON200, nil
}

// stackGitRedeploy is the generated handler for operation StackGitRedeploy.
func stackGitRedeploy(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackGitRedeployInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackGitRedeploy: parse input: %w", err)
	}
	var queryParams apigen.StackGitRedeployParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackGitRedeploy: parse query parameters: %w", err)
	}
	var body apigen.StackGitRedeployJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackGitRedeploy: parse request body: %w", err)
	}
	resp, err := c.API.StackGitRedeployWithResponse(ctx, params.ID, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackGitRedeploy: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackGitRedeploy: %w", err)
	}
	return redactStackGitRedeploy(resp.JSON200), nil
}

// stackImagesStatus is the generated handler for operation StackImagesStatus.
func stackImagesStatus(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackImagesStatusInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackImagesStatus: parse input: %w", err)
	}
	var queryParams apigen.StackImagesStatusParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackImagesStatus: parse query parameters: %w", err)
	}
	resp, err := c.API.StackImagesStatusWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackImagesStatus: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackImagesStatus: %w", err)
	}
	return resp.JSON200, nil
}

// stackInspect is the generated handler for operation StackInspect.
func stackInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackInspect: parse input: %w", err)
	}
	resp, err := c.API.StackInspectWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("StackInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackInspect: %w", err)
	}
	return redactStackInspect(resp.JSON200), nil
}

// stackList is the generated handler for operation StackList.
func stackList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StackListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackList: parse query parameters: %w", err)
	}
	resp, err := c.API.StackListWithResponse(ctx, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackList: %w", err)
	}
	return redactStackList(resp.JSON200), nil
}

// stackStart is the generated handler for operation StackStart.
func stackStart(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackStartInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackStart: parse input: %w", err)
	}
	var queryParams apigen.StackStartParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackStart: parse query parameters: %w", err)
	}
	resp, err := c.API.StackStartWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackStart: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackStart: %w", err)
	}
	return redactStackStart(resp.JSON200), nil
}

// stackStop is the generated handler for operation StackStop.
func stackStop(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackStopInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackStop: parse input: %w", err)
	}
	var queryParams apigen.StackStopParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackStop: parse query parameters: %w", err)
	}
	resp, err := c.API.StackStopWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StackStop: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackStop: %w", err)
	}
	return redactStackStop(resp.JSON200), nil
}

// stackUpdate is the generated handler for operation StackUpdate.
func stackUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackUpdate: parse input: %w", err)
	}
	var queryParams apigen.StackUpdateParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackUpdate: parse query parameters: %w", err)
	}
	var body apigen.StackUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackUpdate: parse request body: %w", err)
	}
	resp, err := c.API.StackUpdateWithResponse(ctx, params.ID, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackUpdate: %w", err)
	}
	return redactStackUpdate(resp.JSON200), nil
}

// stackUpdateGit is the generated handler for operation StackUpdateGit.
func stackUpdateGit(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackUpdateGitInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackUpdateGit: parse input: %w", err)
	}
	var queryParams apigen.StackUpdateGitParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackUpdateGit: parse query parameters: %w", err)
	}
	var body apigen.StackUpdateGitJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackUpdateGit: parse request body: %w", err)
	}
	resp, err := c.API.StackUpdateGitWithResponse(ctx, params.ID, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackUpdateGit: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackUpdateGit: %w", err)
	}
	return redactStackUpdateGit(resp.JSON200), nil
}

// stacksWebhookInvoke is the generated handler for operation StacksWebhookInvoke.
func stacksWebhookInvoke(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stacksWebhookInvokeInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StacksWebhookInvoke: parse input: %w", err)
	}
	resp, err := c.API.StacksWebhookInvokeWithResponse(ctx, params.WebhookID)
	if err != nil {
		return nil, fmt.Errorf("StacksWebhookInvoke: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StacksWebhookInvoke: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		// Destructive is a hand ruling, not a generated flag, and one the
		// scaffold run did not prompt: suspectDangerMismatch's keyword list
		// matches neither this path nor this operationId, so this domain
		// produced no verb-mismatch warning at all and the verb-derived rule
		// stopped at Mutating. It is the same ruling as stacks.webhook_invoke
		// below and rests on the same reasoning; see that entry's comment.
		// The one difference is the resource: this webhook redeploys an Edge
		// stack to the edge environments it targets, so what is replaced is
		// running on machines the caller cannot reach at all.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.edge_stack_webhook_invoke", Domain: "stacks", OperationID: "EdgeStackWebhookInvoke",
			Title:       "Webhook for triggering edge stack updates from git",
			Description: "Webhook for triggering edge stack updates from git",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     edgeStackWebhookInvoke,
			Input:       edgeStackWebhookInvokeInput{},
		}, narrative("EdgeStackWebhookInvoke")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.associate", Domain: "stacks", OperationID: "StackAssociate",
			Title:       "Associate an orphaned stack to a new environment(endpoint)",
			Description: "Associate an orphaned stack to a new environment(endpoint)",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     stackAssociate,
			Input:       stackAssociateInput{},
		}, narrative("StackAssociate")),
		// Not Destructive, and that is a ruling rather than a default left
		// alone. The word "convert" reads as an irreversible change to the
		// stack — this wave's reconnaissance listed it among the three
		// operations that "convert a stack's type" and are "effectively
		// irreversible" — and the vendored document's own summary
		// ("Convert a Docker Compose stack to Kubernetes manifests or Helm
		// chart") does nothing to dispel that. Read the rest of the
		// document and it converts nothing: the request body is
		// {namespace, targetFormat}, the success response is
		// stacks.stackConvertResponse, whose single property is "files",
		// described as "Converted files for preview", and the description
		// ends "and return the results for preview". It reads one stack and
		// answers with generated file text; it creates no stack, changes no
		// stack and deploys nothing. Marking it destructive would put the
		// strongest warning this catalog has on the one action in the
		// domain a caller can use to find out what a conversion would look
		// like *before* committing to one, which is exactly backwards.
		//
		// Mutating is left at the verb-derived true rather than cleared.
		// Nothing in the document says the server records the conversion,
		// but nothing says it does not either, and this route has not been
		// probed against a live server. Clearing Mutating would admit the
		// action to PORTAINER_READ_ONLY on the strength of one word in a
		// description; leaving it costs a safe-mode preview on a preview.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.convert", Domain: "stacks", OperationID: "StackConvert",
			Title:       "Convert a Docker Compose stack to Kubernetes manifests or Helm chart",
			Description: "Convert a Docker Compose or Docker Swarm stack to Kubernetes manifests or Helm chart and return the results for preview.",
			Edition:     edition.EE,
			Mutating:    true,
			Handler:     stackConvert,
			Input:       stackConvertInput{},
		}, narrative("StackConvert")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_docker_standalone_repository", Domain: "stacks", OperationID: "StackCreateDockerStandaloneRepository",
			Title:       "Deploy a new compose stack from repository",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateDockerStandaloneRepository,
			Input:       stackCreateDockerStandaloneRepositoryInput{},
		}, narrative("StackCreateDockerStandaloneRepository")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_docker_standalone_string", Domain: "stacks", OperationID: "StackCreateDockerStandaloneString",
			Title:       "Deploy a new compose stack from a text",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateDockerStandaloneString,
			Input:       stackCreateDockerStandaloneStringInput{},
		}, narrative("StackCreateDockerStandaloneString")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_docker_swarm_repository", Domain: "stacks", OperationID: "StackCreateDockerSwarmRepository",
			Title:       "Deploy a new swarm stack from a git repository",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateDockerSwarmRepository,
			Input:       stackCreateDockerSwarmRepositoryInput{},
		}, narrative("StackCreateDockerSwarmRepository")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_docker_swarm_string", Domain: "stacks", OperationID: "StackCreateDockerSwarmString",
			Title:       "Deploy a new swarm stack from a text",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateDockerSwarmString,
			Input:       stackCreateDockerSwarmStringInput{},
		}, narrative("StackCreateDockerSwarmString")),
		// Name is "stacks.create_kubernetes_string", not the
		// "stacks.create_kubernetes_file" ActionName mints mechanically from
		// the operationId StackCreateKubernetesFile. The route is POST
		// /stacks/create/kubernetes/**string**, its body is
		// application/json with a stackFileContent property, and it has no
		// multipart form: the operationId is simply wrong about its own
		// route. The mechanical name is not merely uninformative, it is
		// false in a way that collides — this domain has two real
		// file-upload actions coming, stacks.create_docker_standalone_file
		// and stacks.create_docker_swarm_file, which do take a multipart
		// upload, so "create_kubernetes_file" would sit beside them
		// asserting membership in a family it does not belong to. Renamed,
		// it reads as the sibling of stacks.create_docker_standalone_string
		// and stacks.create_docker_swarm_string, which is what it is.
		//
		// The rename is registered in cmd/gen_action_inputs's
		// actionNameOverrides as well as written here, so a regeneration
		// reproduces it rather than silently reverting to the mechanical
		// name; TestUnit_ActionNames_MatchThisDomainsRulings is what notices
		// if either half goes missing.
		//
		// Its neighbour stacks.create_kubernetes_git (POST
		// /stacks/create/kubernetes/repository) is deliberately NOT renamed
		// to create_kubernetes_repository for symmetry with
		// create_docker_standalone_repository. That name is merely
		// inconsistent, not wrong — the route does clone from a git
		// repository — and an override that buys tidiness at the cost of one
		// more place where an action's name and its operationId disagree is
		// not worth making. Only a name that misleads earns one.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_kubernetes_string", Domain: "stacks", OperationID: "StackCreateKubernetesFile",
			Title:       "Deploy a new kubernetes stack from a file",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateKubernetesFile,
			Input:       stackCreateKubernetesFileInput{},
		}, narrative("StackCreateKubernetesFile")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_kubernetes_git", Domain: "stacks", OperationID: "StackCreateKubernetesGit",
			Title:       "Deploy a new kubernetes stack from a git repository",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateKubernetesGit,
			Input:       stackCreateKubernetesGitInput{},
		}, narrative("StackCreateKubernetesGit")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_kubernetes_url", Domain: "stacks", OperationID: "StackCreateKubernetesUrl",
			Title:       "Deploy a new kubernetes stack from a url",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateKubernetesURL,
			Input:       stackCreateKubernetesURLInput{},
		}, narrative("StackCreateKubernetesUrl")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.delete", Domain: "stacks", OperationID: "StackDelete",
			Title:       "Remove a stack",
			Description: "Remove a stack.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     stackDelete,
			Input:       stackDeleteInput{},
		}, narrative("StackDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.delete_kubernetes_by_name", Domain: "stacks", OperationID: "StackDeleteKubernetesByName",
			Title:       "Remove Kubernetes stacks by name",
			Description: "Remove a stack.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     stackDeleteKubernetesByName,
			Input:       stackDeleteKubernetesByNameInput{},
		}, narrative("StackDeleteKubernetesByName")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.file_inspect", Domain: "stacks", OperationID: "StackFileInspect",
			Title:       "Retrieve the content of the Stack file for the specified stack",
			Description: "Get Stack file content.",
			Edition:     edition.CE,
			Handler:     stackFileInspect,
			Input:       stackFileInspectInput{},
		}, narrative("StackFileInspect")),
		// Destructive is a hand ruling. The verb-derived rule reads PUT as
		// Mutating+Idempotent and stops, and suspectDangerMismatch matched
		// nothing in this domain, so nothing prompted it. It is the exact
		// shape custom_templates.git_fetch was ruled destructive for last
		// stage, and the criterion that stage settled on applies unchanged:
		// what matters is not that the caller cannot see what it loses but
		// that the request is incapable of expressing it. stacks.update,
		// next to it, is a whole-file replace and is deliberately not
		// destructive, because its payload has a field for the content it
		// overwrites — a caller that reads the stack first with
		// stacks.file_inspect can state the intended end state in full. This
		// request has no such field and can never acquire one: the
		// replacement comes from the git remote at whatever revision the
		// configured reference happens to point at when the call lands, the
		// payload names no revision, and Portainer keeps no copy of the
		// replaced deployment. repositoryPassword makes it worse still —
		// with repositoryAuthentication it overwrites the stack's stored git
		// credential as a side effect of redeploying — and prune deletes
		// services the new file no longer names.
		//
		// Idempotent is cleared, and that is the same ruling read the other
		// way round rather than a second, independent one. The verb-derived
		// rule gives every PUT in this domain Idempotent, and for the other
		// three that is right; here it contradicts the paragraph above it.
		// "Can be repeated without additional effect" (toolutil.ActionSpec's
		// own words) is exactly what an action cannot claim when the content
		// it deploys is not determined by its own request: two calls a minute
		// apart deploy whatever the configured reference points at each time,
		// and those can differ. The flag is not inert — tools.AnnotationsFor
		// passes it to clients as IdempotentHint, whose whole purpose is to
		// tell a caller an action is safe to retry unattended — so leaving it
		// set would invite automatic retry of the most irreversible
		// non-delete write this domain has.
		//
		// It also has to agree with stacks.webhook_invoke, which the narrative
		// calls this same replacement through a different door. That one is a
		// POST and so carries no idempotency hint at all; two actions this
		// domain itself calls equivalent must not hand callers opposite advice
		// about retrying them.
		//
		// The line this draws for the whole domain is the one the Destructive
		// rulings already draw, read for a different flag: an action is
		// idempotent when repeating it with the same arguments leaves the same
		// state, and what decides that is whether the request determines the
		// state. stacks.update keeps Idempotent on exactly that basis — its
		// stackFileContent and env carry the end state — even though its
		// optional repullImageAndRedeploy lets a caller opt into the same
		// non-determinism. Opt-in through a field the request carries is the
		// caller's own choice to express; unconditional and inexpressible is
		// this route's.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.git_redeploy", Domain: "stacks", OperationID: "StackGitRedeploy",
			Title:       "Redeploy a stack",
			Description: "Pull and redeploy a stack via Git",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Handler:     stackGitRedeploy,
			Input:       stackGitRedeployInput{},
		}, narrative("StackGitRedeploy")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.images_status", Domain: "stacks", OperationID: "StackImagesStatus",
			Title:       "Fetch image status for stack",
			Description: "Fetch image status for stack",
			Edition:     edition.EE,
			Handler:     stackImagesStatus,
			Input:       stackImagesStatusInput{},
		}, narrative("StackImagesStatus")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.inspect", Domain: "stacks", OperationID: "StackInspect",
			Title:       "Inspect a stack",
			Description: "Retrieve details about a stack.",
			Edition:     edition.CE,
			Handler:     stackInspect,
			Input:       stackInspectInput{},
		}, narrative("StackInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.list", Domain: "stacks", OperationID: "StackList",
			Title:       "List stacks",
			Description: "List all stacks based on the current user authorizations.\nWill return all stacks if using an administrator account otherwise it\nwill only return the list of stacks the user have access to.\nLimited stacks will not be returned by this endpoint.",
			Edition:     edition.CE,
			Handler:     stackList,
			Input:       stackListInput{},
		}, narrative("StackList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.start", Domain: "stacks", OperationID: "StackStart",
			Title:       "Starts a stopped Stack",
			Description: "Starts a stopped Stack.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackStart,
			Input:       stackStartInput{},
		}, narrative("StackStart")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.stop", Domain: "stacks", OperationID: "StackStop",
			Title:       "Stop a running Stack",
			Description: "Stop a running Stack.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackStop,
			Input:       stackStopInput{},
		}, narrative("StackStop")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.update", Domain: "stacks", OperationID: "StackUpdate",
			Title:       "Update a stack",
			Description: "Update a stack, only for file based stacks.",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     stackUpdate,
			Input:       stackUpdateInput{},
		}, narrative("StackUpdate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.update_git", Domain: "stacks", OperationID: "StackUpdateGit",
			Title:       "Update a stack's Git configs",
			Description: "Update the Git settings in a stack, e.g., RepositoryReferenceName and AutoUpdate. When SourceID is set, URL/auth/TLS are taken from the referenced Source.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackUpdateGit,
			Input:       stackUpdateGitInput{},
		}, narrative("StackUpdateGit")),
		// Destructive is a hand ruling, and the one in this domain that goes
		// beyond the three operations the wave brief asked to be ruled on.
		// It follows from the stacks.git_redeploy ruling rather than
		// standing on its own: invoking this webhook makes Portainer pull
		// from the stack's repository and redeploy it, which is the same
		// replacement by a different door. Ruling git_redeploy destructive
		// and leaving the webhook that performs it Mutating would leave the
		// flag saying something about which route was called rather than
		// about what happens to the deployment, and a model routed to the
		// webhook would get no warning at all. If anything the case is
		// stronger here: this request carries a UUID and nothing else, so
		// the caller names neither the revision nor even the stack.
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.webhook_invoke", Domain: "stacks", OperationID: "StacksWebhookInvoke",
			Title:       "Webhook for triggering stack updates from git",
			Description: "Webhook for triggering stack updates from git",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     stacksWebhookInvoke,
			Input:       stacksWebhookInvokeInput{},
		}, narrative("StacksWebhookInvoke")),
	}
}
