// Package registries declares the Portainer container registry actions.
//
// Ten actions in Business Edition, seven in Community Edition. The three
// missing from Community Edition — deleting an ECR repository, deleting ECR
// tags, and deleting repository tags on a generic registry — are genuinely
// absent from its specification, not merely hidden, which makes this domain
// the pilot for per-action edition gating within a single domain: the same
// declarations must yield a smaller catalog on CE than on EE.
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

// Specs declares every registry action.
func Specs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		{
			Name: "registries.list", Domain: "registries", OperationID: "RegistryList",
			Title:       "List registries",
			Description: "Returns every container registry configured on this Portainer server.",
			Edition:     edition.CE,
			Handler:     registryList,
		},
		{
			Name: "registries.create", Domain: "registries", OperationID: "RegistryCreate",
			Title:       "Create a registry",
			Description: "Registers a new container registry with Portainer.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryCreate,
			Input:       registryCreateInput{},
		},
		{
			Name: "registries.ping", Domain: "registries", OperationID: "RegistryPing",
			Title:       "Test a registry connection",
			Description: "Checks that Portainer can reach and authenticate against a registry, without persisting it.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryPing,
			Input:       registryPingInput{},
		},
		{
			Name: "registries.inspect", Domain: "registries", OperationID: "RegistryInspect",
			Title:       "Inspect a registry",
			Description: "Returns the details of a single registry by identifier.",
			Edition:     edition.CE,
			Handler:     registryInspect,
			Input:       registryInspectInput{},
		},
		{
			Name: "registries.update", Domain: "registries", OperationID: "RegistryUpdate",
			Title:       "Update a registry",
			Description: "Replaces a registry's configuration.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryUpdate,
			Input:       registryUpdateInput{},
		},
		{
			Name: "registries.configure", Domain: "registries", OperationID: "RegistryConfigure",
			Title:       "Configure a registry for management",
			Description: "Sets the management credentials and TLS options Portainer uses to browse a registry's repositories and tags.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryConfigure,
			Input:       registryConfigureInput{},
		},
		{
			Name: "registries.delete", Domain: "registries", OperationID: "RegistryDelete",
			Title:       "Delete a registry",
			Description: "Permanently removes a registry from Portainer. This cannot be undone.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Handler:     registryDelete,
			Input:       registryDeleteInput{},
		},
		{
			Name: "registries.ecr_delete_repository", Domain: "registries", OperationID: "EcrDeleteRepository",
			Title:       "Delete an ECR repository",
			Description: "Permanently deletes a repository from an Amazon ECR registry. Business Edition only. This cannot be undone.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     ecrDeleteRepository,
			Input:       ecrDeleteRepositoryInput{},
		},
		{
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
		{
			Name: "registries.repository_tags_delete", Domain: "registries", OperationID: "RepositoryTagsDelete",
			Title:       "Delete repository image tags",
			Description: "Permanently deletes the given image tags from a repository on a generic registry. Business Edition only. This cannot be undone.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     repositoryTagsDelete,
			Input:       repositoryTagsDeleteInput{},
		},
	}
}

// The functions below convert the nested objects declared on the generated
// input structs (registryCreateInput, registryUpdateInput,
// registryConfigureInput) into the corresponding apigen wire types. Every
// field named in the task-4 finding — Ecr, Github, Gitlab, Quay,
// RegistryAccesses, and the TLS certificate file fields — has a matching
// parameter on the generated client's request body, so each is forwarded
// whole rather than picked apart: a caller who sends "ecr": {"region": "x"}
// gets exactly that structure on the wire, not a hand-picked subset of it.
//
// registryCreateInputEcr and registryUpdateInputEcr (and their Github/Quay
// counterparts) are field-for-field identical, but the generator emits a
// distinct named type per operation, so each operation gets its own
// converter rather than one shared by structural coincidence.

// createEcrToAPI converts registryCreate's Ecr input to the wire type.
func createEcrToAPI(in *registryCreateInputEcr) *apigen.PortainerEcrData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerEcrData{Region: in.Region}
}

// updateEcrToAPI converts registryUpdate's Ecr input to the wire type.
func updateEcrToAPI(in *registryUpdateInputEcr) *apigen.PortainerEcrData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerEcrData{Region: in.Region}
}

// createGithubToAPI converts registryCreate's Github input to the wire type.
func createGithubToAPI(in *registryCreateInputGithub) *apigen.PortainerGithubRegistryData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerGithubRegistryData{OrganisationName: in.OrganisationName, UseOrganisation: in.UseOrganisation}
}

// updateGithubToAPI converts registryUpdate's Github input to the wire type.
func updateGithubToAPI(in *registryUpdateInputGithub) *apigen.PortainerGithubRegistryData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerGithubRegistryData{OrganisationName: in.OrganisationName, UseOrganisation: in.UseOrganisation}
}

// createGitlabToAPI converts registryCreate's Gitlab input to the wire type.
// Update has no Gitlab field — neither registryUpdateInput nor
// registries.registryUpdatePayload declares one, so there is nothing to
// forward there and no counterpart function.
func createGitlabToAPI(in *registryCreateInputGitlab) *apigen.PortainerGitlabRegistryData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerGitlabRegistryData{InstanceURL: in.InstanceURL, ProjectId: in.ProjectID, ProjectPath: in.ProjectPath}
}

// createQuayToAPI converts registryCreate's Quay input to the wire type.
func createQuayToAPI(in *registryCreateInputQuay) *apigen.PortainerQuayRegistryData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerQuayRegistryData{OrganisationName: in.OrganisationName, UseOrganisation: in.UseOrganisation}
}

// updateQuayToAPI converts registryUpdate's Quay input to the wire type.
func updateQuayToAPI(in *registryUpdateInputQuay) *apigen.PortainerQuayRegistryData {
	if in == nil {
		return nil
	}
	return &apigen.PortainerQuayRegistryData{OrganisationName: in.OrganisationName, UseOrganisation: in.UseOrganisation}
}

// toRegistryAccesses converts registryUpdate's RegistryAccesses input —
// keyed by registry ID, each value carrying Namespaces plus per-team and
// per-user access policies — into the wire type. It nests through
// PortainerRegistryAccessPolicies -> PortainerTeamAccessPolicies /
// PortainerUserAccessPolicies -> PortainerAccessPolicy, converting one level
// at a time rather than re-marshaling, since the input and wire shapes carry
// the same fields under different (and, for the map value, differently
// pointered) named types.
func toRegistryAccesses(in map[string]registryUpdateInputRegistryAccessesValue) *apigen.PortainerRegistryAccesses {
	if in == nil {
		return nil
	}
	out := make(apigen.PortainerRegistryAccesses, len(in))
	for registryID, access := range in {
		policies := apigen.PortainerRegistryAccessPolicies{}
		if access.Namespaces != nil {
			namespaces := access.Namespaces
			policies.Namespaces = &namespaces
		}
		if access.TeamAccessPolicies != nil {
			team := make(apigen.PortainerTeamAccessPolicies, len(access.TeamAccessPolicies))
			for teamID, policy := range access.TeamAccessPolicies {
				team[teamID] = toAccessPolicy(policy.Namespaces, policy.RoleID)
			}
			policies.TeamAccessPolicies = &team
		}
		if access.UserAccessPolicies != nil {
			user := make(apigen.PortainerUserAccessPolicies, len(access.UserAccessPolicies))
			for userID, policy := range access.UserAccessPolicies {
				user[userID] = toAccessPolicy(policy.Namespaces, policy.RoleID)
			}
			policies.UserAccessPolicies = &user
		}
		out[registryID] = policies
	}
	return &out
}

// toAccessPolicy converts one team- or user-access-policy entry into the
// wire type; registryUpdateInputRegistryAccessesValueTeamAccessPoliciesValue
// and its User counterpart declare identical fields, so both callers pass
// the same two values in rather than needing their own converter.
func toAccessPolicy(namespaces []string, roleID int) apigen.PortainerAccessPolicy {
	policy := apigen.PortainerAccessPolicy{RoleId: roleID}
	if namespaces != nil {
		ns := namespaces
		policy.Namespaces = &ns
	}
	return policy
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

func registryList(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.RegistryListWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("registries list: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries list: %w", err)
	}
	return redactList(resp.JSON200), nil
}

func registryCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryCreateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries create: parse input: %w", err)
	}
	if params.Name == "" {
		return nil, fmt.Errorf("registries create: name is required")
	}
	if params.URL == "" {
		return nil, fmt.Errorf("registries create: url is required")
	}

	body := apigen.RegistryCreateJSONRequestBody{
		Name:           params.Name,
		URL:            params.URL,
		Type:           apigen.PortainerRegistryType(params.Type),
		Authentication: params.Authentication,
		Username:       params.Username,
		Password:       params.Password,
		BaseURL:        params.BaseURL,
		Ecr:            createEcrToAPI(params.Ecr),
		Github:         createGithubToAPI(params.Github),
		Gitlab:         createGitlabToAPI(params.Gitlab),
		Quay:           createQuayToAPI(params.Quay),
	}
	body.TLS = params.TLS

	resp, err := c.API.RegistryCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("registries create: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries create: %w", err)
	}
	return redact(resp.JSON200), nil
}

func registryPing(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryPingInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries ping: parse input: %w", err)
	}
	if params.URL == "" {
		return nil, fmt.Errorf("registries ping: url is required")
	}

	body := apigen.RegistryPingJSONRequestBody{
		URL:      params.URL,
		Type:     apigen.PortainerRegistryType(params.Type),
		Username: params.Username,
		Password: params.Password,
	}
	body.TLS = params.TLS

	resp, err := c.API.RegistryPingWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("registries ping: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries ping: %w", err)
	}
	return resp.JSON200, nil
}

func registryInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries inspect: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries inspect: id must be a positive integer, got %d", params.ID)
	}

	var reqParams *apigen.RegistryInspectParams
	if params.EndpointID != nil {
		reqParams = &apigen.RegistryInspectParams{EndpointId: params.EndpointID}
	}

	resp, err := c.API.RegistryInspectWithResponse(ctx, params.ID, reqParams)
	if err != nil {
		return nil, fmt.Errorf("registries inspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries inspect: %w", err)
	}
	return redact(resp.JSON200), nil
}

func registryUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries update: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries update: id must be a positive integer, got %d", params.ID)
	}
	if params.Name == "" {
		return nil, fmt.Errorf("registries update: name is required")
	}
	if params.URL == "" {
		return nil, fmt.Errorf("registries update: url is required")
	}

	body := apigen.RegistryUpdateJSONRequestBody{
		Name:             params.Name,
		URL:              params.URL,
		Authentication:   params.Authentication,
		Username:         params.Username,
		Password:         params.Password,
		BaseURL:          params.BaseURL,
		Ecr:              updateEcrToAPI(params.Ecr),
		Github:           updateGithubToAPI(params.Github),
		Quay:             updateQuayToAPI(params.Quay),
		RegistryAccesses: toRegistryAccesses(params.RegistryAccesses),
	}

	resp, err := c.API.RegistryUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("registries update: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries update: %w", err)
	}
	return redact(resp.JSON200), nil
}

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

func registryDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries delete: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries delete: id must be a positive integer, got %d", params.ID)
	}

	resp, err := c.API.RegistryDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("registries delete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries delete: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID}, nil
}

func ecrDeleteRepository(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params ecrDeleteRepositoryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries ecr_delete_repository: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries ecr_delete_repository: id must be a positive integer, got %d", params.ID)
	}
	if params.RepositoryName == "" {
		return nil, fmt.Errorf("registries ecr_delete_repository: repositoryName is required")
	}

	resp, err := c.API.EcrDeleteRepositoryWithResponse(ctx, params.ID, params.RepositoryName)
	if err != nil {
		return nil, fmt.Errorf("registries ecr_delete_repository: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries ecr_delete_repository: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID, "repositoryName": params.RepositoryName}, nil
}

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
