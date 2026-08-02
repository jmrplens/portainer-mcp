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
		},
		{
			Name: "registries.ping", Domain: "registries", OperationID: "RegistryPing",
			Title:       "Test a registry connection",
			Description: "Checks that Portainer can reach and authenticate against a registry, without persisting it.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryPing,
		},
		{
			Name: "registries.inspect", Domain: "registries", OperationID: "RegistryInspect",
			Title:       "Inspect a registry",
			Description: "Returns the details of a single registry by identifier.",
			Edition:     edition.CE,
			Handler:     registryInspect,
		},
		{
			Name: "registries.update", Domain: "registries", OperationID: "RegistryUpdate",
			Title:       "Update a registry",
			Description: "Replaces a registry's configuration.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryUpdate,
		},
		{
			Name: "registries.configure", Domain: "registries", OperationID: "RegistryConfigure",
			Title:       "Configure a registry for management",
			Description: "Sets the management credentials and TLS options Portainer uses to browse a registry's repositories and tags.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryConfigure,
		},
		{
			Name: "registries.delete", Domain: "registries", OperationID: "RegistryDelete",
			Title:       "Delete a registry",
			Description: "Permanently removes a registry from Portainer. This cannot be undone.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Handler:     registryDelete,
		},
		{
			Name: "registries.ecr_delete_repository", Domain: "registries", OperationID: "EcrDeleteRepository",
			Title:       "Delete an ECR repository",
			Description: "Permanently deletes a repository from an Amazon ECR registry. Business Edition only. This cannot be undone.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     ecrDeleteRepository,
		},
		{
			Name: "registries.ecr_delete_tags", Domain: "registries", OperationID: "EcrDeleteTags",
			Title:       "Delete ECR image tags",
			Description: "Permanently deletes the given image tags from an Amazon ECR repository. Business Edition only. This cannot be undone.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     ecrDeleteTags,
		},
		{
			Name: "registries.repository_tags_delete", Domain: "registries", OperationID: "RepositoryTagsDelete",
			Title:       "Delete repository image tags",
			Description: "Permanently deletes the given image tags from a repository on a generic registry. Business Edition only. This cannot be undone.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Handler:     repositoryTagsDelete,
		},
	}
}

func registryList(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.RegistryListWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("registries list: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries list: %w", err)
	}
	return resp.JSON200, nil
}

// registryCreateInput is the parameter shape for registries.create.
type registryCreateInput struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Type           int    `json:"type"`
	Authentication bool   `json:"authentication,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TLS            bool   `json:"tls,omitempty"`
	BaseURL        string `json:"baseUrl,omitempty"`
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
	}
	if params.Username != "" {
		body.Username = &params.Username
	}
	if params.Password != "" {
		body.Password = &params.Password
	}
	if params.TLS {
		tls := params.TLS
		body.TLS = &tls
	}
	if params.BaseURL != "" {
		body.BaseURL = &params.BaseURL
	}

	resp, err := c.API.RegistryCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("registries create: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries create: %w", err)
	}
	return resp.JSON200, nil
}

// registryPingInput is the parameter shape for registries.ping.
type registryPingInput struct {
	URL      string `json:"url"`
	Type     int    `json:"type"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	TLS      bool   `json:"tls,omitempty"`
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
		URL:  params.URL,
		Type: apigen.PortainerRegistryType(params.Type),
	}
	if params.Username != "" {
		body.Username = &params.Username
	}
	if params.Password != "" {
		body.Password = &params.Password
	}
	if params.TLS {
		tls := params.TLS
		body.TLS = &tls
	}

	resp, err := c.API.RegistryPingWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("registries ping: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries ping: %w", err)
	}
	return resp.JSON200, nil
}

// registryInspectInput is the parameter shape for registries.inspect.
type registryInspectInput struct {
	ID         int  `json:"id"`
	EndpointID *int `json:"endpointId,omitempty"`
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
	return resp.JSON200, nil
}

// registryUpdateInput is the parameter shape for registries.update.
type registryUpdateInput struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Authentication bool   `json:"authentication,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
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
		Name:           params.Name,
		URL:            params.URL,
		Authentication: params.Authentication,
	}
	if params.Username != "" {
		body.Username = &params.Username
	}
	if params.Password != "" {
		body.Password = &params.Password
	}

	resp, err := c.API.RegistryUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("registries update: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries update: %w", err)
	}
	return resp.JSON200, nil
}

// registryConfigureInput is the parameter shape for registries.configure.
type registryConfigureInput struct {
	ID             int    `json:"id"`
	Authentication bool   `json:"authentication,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	Region         string `json:"region,omitempty"`
	TLS            bool   `json:"tls,omitempty"`
	TLSSkipVerify  bool   `json:"tlsSkipVerify,omitempty"`
}

func registryConfigure(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryConfigureInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries configure: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries configure: id must be a positive integer, got %d", params.ID)
	}

	body := apigen.RegistryConfigureJSONRequestBody{Authentication: params.Authentication}
	if params.Username != "" {
		body.Username = &params.Username
	}
	if params.Password != "" {
		body.Password = &params.Password
	}
	if params.Region != "" {
		body.Region = &params.Region
	}
	if params.TLS {
		tls := params.TLS
		body.TLS = &tls
	}
	if params.TLSSkipVerify {
		skip := params.TLSSkipVerify
		body.TLSSkipVerify = &skip
	}

	resp, err := c.API.RegistryConfigureWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("registries configure: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries configure: %w", err)
	}
	return map[string]any{"configured": true, "id": params.ID}, nil
}

// registryDeleteInput is the parameter shape for registries.delete.
type registryDeleteInput struct {
	ID int `json:"id"`
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

// ecrDeleteRepositoryInput is the parameter shape for registries.ecr_delete_repository.
type ecrDeleteRepositoryInput struct {
	ID             int    `json:"id"`
	RepositoryName string `json:"repositoryName"`
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

// ecrDeleteTagsInput is the parameter shape for registries.ecr_delete_tags.
//
// RepositoryName is an integer here, not a string, because the generated
// method's second parameter is typed int — matching the vendored EE
// specification for this operation exactly, however unusual that looks next
// to every other repositoryName in this domain.
type ecrDeleteTagsInput struct {
	ID             int      `json:"id"`
	RepositoryName int      `json:"repositoryName"`
	Tags           []string `json:"tags,omitempty"`
}

func ecrDeleteTags(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params ecrDeleteTagsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("registries ecr_delete_tags: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("registries ecr_delete_tags: id must be a positive integer, got %d", params.ID)
	}

	body := apigen.EcrDeleteTagsJSONRequestBody{}
	if len(params.Tags) > 0 {
		body.Tags = &params.Tags
	}

	resp, err := c.API.EcrDeleteTagsWithResponse(ctx, params.ID, params.RepositoryName, body)
	if err != nil {
		return nil, fmt.Errorf("registries ecr_delete_tags: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries ecr_delete_tags: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID, "repositoryName": params.RepositoryName}, nil
}

// repositoryTagsDeleteInput is the parameter shape for registries.repository_tags_delete.
type repositoryTagsDeleteInput struct {
	ID             int      `json:"id"`
	RepositoryName string   `json:"repositoryName"`
	Tags           []string `json:"tags,omitempty"`
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

	body := apigen.RepositoryTagsDeleteJSONRequestBody{}
	if len(params.Tags) > 0 {
		body.Tags = &params.Tags
	}

	resp, err := c.API.RepositoryTagsDeleteWithResponse(ctx, params.ID, params.RepositoryName, body)
	if err != nil {
		return nil, fmt.Errorf("registries repository_tags_delete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("registries repository_tags_delete: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID, "repositoryName": params.RepositoryName}, nil
}
