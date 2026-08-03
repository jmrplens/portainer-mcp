// Package registries declares the Portainer container registry actions.
//
// Ten actions in Business Edition, seven in Community Edition. The three
// missing from Community Edition — deleting an ECR repository, deleting ECR
// tags, and deleting repository tags on a generic registry — are genuinely
// absent from its specification, not merely hidden, which makes this domain
// the pilot for per-action edition gating within a single domain: the same
// declarations must yield a smaller catalog on CE than on EE.
//
// registries.list, registries.create, registries.ping, registries.inspect,
// registries.update, registries.delete and registries.ecr_delete_repository
// all run on cmd/gen_action_inputs's generated code (actions.gen.go).
//
// The id-guard tests that used to block inspect/update/delete/
// ecr_delete_repository (TestRegistryInspect_InvalidID_...,
// TestRegistryUpdate_MissingFields_... "invalid id", and the delete/ecr
// equivalents — each asserting the handler refuses a non-positive id before
// the request reaches the network) are now satisfied by
// cmd/gen_action_inputs's own generated schema constraint instead: every
// integer path parameter shaped like a Portainer identifier (isIdentifierPathParam,
// fields.go) publishes a JSON Schema "minimum": 1
// (toolutil.MinimumParams), so tools.Execute's central validation refuses a
// non-positive id before any handler runs — see that generator's own doc
// comment for why this is this project's addition, not the specification's.
// Every remaining guard-clause test that asserted a required-field check
// (TestRegistryCreate_MissingFields_..., TestRegistryPing_MissingURL_...,
// TestRegistryEcrDeleteRepository_MissingFields_... "missing repositoryName")
// routes through tools.Execute rather than calling the handler directly —
// the path every real caller actually takes, and where that check now lives.
// registries.inspect and registries.update additionally needed a redaction
// wrapper (redactRegistryInspect, redactRegistryUpdate below) before
// cmd/gen_action_inputs would generate them at all, for the same reason
// registries.list/create did (see task-4b's report).
//
// registries.ecr_delete_tags and registries.repository_tags_delete stay
// hand-written: each has an existing test
// (TestEcrDeleteTags_EmptyTags_ReturnsErrorWithoutCallingAPI and its
// repository_tags_delete equivalent) asserting the handler refuses an empty
// or omitted tags list before the request reaches the network. That is an
// array-length ("minItems") constraint, not an integer range — the fix that
// closed the id-guard gap does not reach it, and jsonschema-go's reflector
// has no struct-tag syntax for "minItems" either (see
// internal/toolutil/schema.go's EnumParams/MinimumParams doc comments on
// what it does and does not express), so there is still no equivalent path
// to route these two tests through.
//
// registries.configure stays hand-written for an unrelated, pre-existing
// reason: Task 2's own width refusal (TLSCACertFile/TLSCertFile/TLSKeyFile
// narrow from this generator's []int to the generated client's []int32).
package registries

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every registry action: generatedSpecs()'s seven entries
// (registries.list/create/ping/inspect/update/delete/ecr_delete_repository)
// plus the three kept hand-written below.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(),
		toolutil.ActionSpec{
			Name: "registries.configure", Domain: "registries", OperationID: "RegistryConfigure",
			Title:       "Configure a registry for management",
			Description: "Sets the management credentials and TLS options Portainer uses to browse a registry's repositories and tags.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryConfigure,
			Input:       registryConfigureInput{},
		},
		toolutil.ActionSpec{
			Name: "registries.ecr_delete_tags", Domain: "registries", OperationID: "EcrDeleteTags",
			Title: "Delete ECR image tags",
			Description: "Permanently deletes the given image tags from an Amazon ECR repository. Business Edition only. This cannot be undone. " +
				"repositoryName here is Portainer's numeric repository identifier, not the repository's name — pass the integer id, not a string.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     ecrDeleteTags,
			Input:       ecrDeleteTagsInput{},
		},
		toolutil.ActionSpec{
			Name: "registries.repository_tags_delete", Domain: "registries", OperationID: "RepositoryTagsDelete",
			Title:       "Delete repository image tags",
			Description: "Permanently deletes the given image tags from a repository on a generic registry. Business Edition only. This cannot be undone.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     repositoryTagsDelete,
			Input:       repositoryTagsDeleteInput{},
		},
	)
}

// narrative supplies registries.list/create/ping's ActionSpec narrative
// fields to generatedSpecs() (see actions.gen.go): Title/Description only,
// preserving the exact wording this domain hand-authored before the swap to
// generated code, rather than letting it silently degrade to the vendored
// specification's own terser summary/description. Every other operationId in
// this domain returns the zero toolutil.ActionNarrative — nothing else here
// is generated.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "RegistryList":
		return toolutil.ActionNarrative{
			Title:       "List registries",
			Description: "Returns every container registry configured on this Portainer server.",
		}
	case "RegistryCreate":
		return toolutil.ActionNarrative{
			Title:       "Create a registry",
			Description: "Registers a new container registry with Portainer.",
		}
	case "RegistryPing":
		return toolutil.ActionNarrative{
			Title:       "Test a registry connection",
			Description: "Checks that Portainer can reach and authenticate against a registry, without persisting it.",
		}
	case "RegistryInspect":
		return toolutil.ActionNarrative{
			Title:       "Inspect a registry",
			Description: "Returns the details of a single registry by identifier.",
		}
	case "RegistryUpdate":
		return toolutil.ActionNarrative{
			Title:       "Update a registry",
			Description: "Replaces a registry's configuration.",
		}
	case "RegistryDelete":
		return toolutil.ActionNarrative{
			Title:       "Delete a registry",
			Description: "Permanently removes a registry from Portainer. This cannot be undone.",
		}
	case "EcrDeleteRepository":
		return toolutil.ActionNarrative{
			Title:       "Delete an ECR repository",
			Description: "Permanently deletes a repository from an Amazon ECR registry. Business Edition only. This cannot be undone.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}

// toTLSFileBytes converts registryConfigure's TLS certificate file inputs to
// the wire type. Per registries.registryConfigurePayload, TLSCACertFile,
// TLSCertFile and TLSKeyFile are declared as arrays of int32-formatted
// integers, not — despite the "File" in their names — a multipart upload;
// RegistryConfigureJSONRequestBody has no other way to carry them, so this
// is the complete and correct forwarding, not a placeholder pending a real
// upload path. It errors rather than truncating if a value does not fit in
// an int32, since silently corrupting certificate bytes is worse than
// refusing the call.
func toTLSFileBytes(in []int) (*[]int32, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]int32, len(in))
	for i, v := range in {
		if v < math.MinInt32 || v > math.MaxInt32 {
			return nil, fmt.Errorf("byte at index %d (%d) does not fit in int32", i, v)
		}
		out[i] = int32(v)
	}
	return &out, nil
}

// registryList, registryCreate and registryPing are no longer declared here:
// they run on cmd/gen_action_inputs's generated code (actions.gen.go) — see
// this file's package doc. redactRegistryList, redactRegistryCreate,
// redactRegistryInspect and redactRegistryUpdate below are what that
// generated code calls instead of returning
// RegistryListWithResponse's/RegistryCreateWithResponse's/
// RegistryInspectWithResponse's/RegistryUpdateWithResponse's JSON200
// directly. registryUpdate's own Ecr/Github/Quay/RegistryAccesses converters
// (updateEcrToAPI and siblings) are gone for the identical reason
// registryCreate's were: the generated handler round-trips the caller's raw
// input straight into apigen.RegistryUpdateJSONRequestBody through
// encoding/json, so nothing here needs to assign those fields by hand any
// more.

// redact removes credentials from a registry record before it is returned.
//
// PortainereeRegistry and its nested ManagementConfiguration each carry a
// Password field ("Password or SecretAccessKey used to authenticate against
// this registry" per the generated doc comment) and an AccessToken field
// ("Stores temporary access token") — both are credentials, not registry
// metadata, and a tool result is read by a model and lands in transcripts, so
// neither must ever travel that way. Whether Portainer actually populates
// them on a given response is not something this code should have to know,
// and only List is documented by Portainer as pre-scrubbed — that claim is
// Portainer's, not ours to rely on — so every handler that returns a
// registry record redacts unconditionally.
func redact(r *apigen.PortainereeRegistry) *apigen.PortainereeRegistry {
	if r == nil {
		return nil
	}
	scrubbed := *r
	scrubbed.Password = nil
	scrubbed.AccessToken = nil
	if scrubbed.ManagementConfiguration != nil {
		config := *scrubbed.ManagementConfiguration
		config.Password = nil
		config.AccessToken = nil
		// Dropped whole rather than field by field: TLSConfig carries
		// certificate and key paths that disclose the server's filesystem
		// layout, a model has no use for them, and enumerating its fields
		// would invite the same omission again when one is added.
		config.TLSConfig = nil
		scrubbed.ManagementConfiguration = &config
	}
	return &scrubbed
}

// redactList applies redact to every element of a registry list response.
func redactList(rs *[]apigen.PortainereeRegistry) *[]apigen.PortainereeRegistry {
	if rs == nil {
		return nil
	}
	out := make([]apigen.PortainereeRegistry, len(*rs))
	for i := range *rs {
		out[i] = *redact(&(*rs)[i])
	}
	return &out
}

// redactRegistryList, redactRegistryCreate, redactRegistryInspect and
// redactRegistryUpdate are the redaction wrappers cmd/gen_action_inputs's
// generator requires before it will generate a handler for
// RegistryList/RegistryCreate/RegistryInspect/RegistryUpdate at all: every
// one of their success responses can carry Password/AccessToken (per
// toolutil.IsCredentialShapedName, resolved through the vendored spec's own
// $refs — see credential.go in that package), so the generator refuses to
// emit a bare handler for any of them without a function named exactly this
// way already declared here. Each is a thin rename over the redact/redactList
// this domain already had and already tests
// (TestRegistryList_ResponseWithPassword_IsRedacted,
// TestRegistryCreate_ResponseWithPassword_IsRedacted,
// TestRegistryInspect_ResponseWithPassword_IsRedacted,
// TestRegistryUpdate_ResponseWithPassword_IsRedacted,
// TestRedact_LeavesNoCredentialShapedFieldPopulated): the generator's
// contract only requires a function of this name and shape to exist, not
// that it be a new implementation.
func redactRegistryList(rs *[]apigen.PortainereeRegistry) any { return redactList(rs) }
func redactRegistryCreate(r *apigen.PortainereeRegistry) any  { return redact(r) }
func redactRegistryInspect(r *apigen.PortainereeRegistry) any { return redact(r) }
func redactRegistryUpdate(r *apigen.PortainereeRegistry) any  { return redact(r) }

func registryConfigure(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryConfigureInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries configure: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries configure: id must be a positive integer, got %d", params.ID)
	}

	tlsCACertFile, err := toTLSFileBytes(params.TLSCACertFile)
	if err != nil {
		return nil, fmt.Errorf("registries configure: tlsCACertFile: %w", err)
	}
	tlsCertFile, err := toTLSFileBytes(params.TLSCertFile)
	if err != nil {
		return nil, fmt.Errorf("registries configure: tlsCertFile: %w", err)
	}
	tlsKeyFile, err := toTLSFileBytes(params.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("registries configure: tlsKeyFile: %w", err)
	}

	body := apigen.RegistryConfigureJSONRequestBody{
		Authentication: params.Authentication,
		Username:       params.Username,
		Password:       params.Password,
		Region:         params.Region,
		TLSCACertFile:  tlsCACertFile,
		TLSCertFile:    tlsCertFile,
		TLSKeyFile:     tlsKeyFile,
	}
	body.TLS = params.TLS
	body.TLSSkipVerify = params.TLSSkipVerify

	resp, err := c.API.RegistryConfigureWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("registries configure: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries configure: %w", err)
	}
	return map[string]any{"configured": true, "id": params.ID}, nil
}

// registryDelete and ecrDeleteRepository are no longer declared here: they
// run on cmd/gen_action_inputs's generated code (actions.gen.go) — see this
// file's package doc. Neither response carries a credential-shaped field, so
// unlike inspect/update, neither needed a redaction wrapper to become
// eligible.

func ecrDeleteTags(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params ecrDeleteTagsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries ecr_delete_tags: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries ecr_delete_tags: id must be a positive integer, got %d", params.ID)
	}
	if len(params.Tags) == 0 {
		return nil, fmt.Errorf("registries ecr_delete_tags: tags must list at least one tag")
	}

	body := apigen.EcrDeleteTagsJSONRequestBody{Tags: &params.Tags}

	resp, err := c.API.EcrDeleteTagsWithResponse(ctx, params.ID, params.RepositoryName, body)
	if err != nil {
		return nil, fmt.Errorf("registries ecr_delete_tags: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries ecr_delete_tags: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID, "repositoryName": params.RepositoryName}, nil
}

func repositoryTagsDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params repositoryTagsDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries repository_tags_delete: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries repository_tags_delete: id must be a positive integer, got %d", params.ID)
	}
	if params.RepositoryName == "" {
		return nil, fmt.Errorf("registries repository_tags_delete: repositoryName is required")
	}
	if len(params.Tags) == 0 {
		return nil, fmt.Errorf("registries repository_tags_delete: tags must list at least one tag")
	}

	body := apigen.RepositoryTagsDeleteJSONRequestBody{Tags: &params.Tags}

	resp, err := c.API.RepositoryTagsDeleteWithResponse(ctx, params.ID, params.RepositoryName, body)
	if err != nil {
		return nil, fmt.Errorf("registries repository_tags_delete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries repository_tags_delete: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID, "repositoryName": params.RepositoryName}, nil
}
