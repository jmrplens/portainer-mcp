package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// dockerGetJSON issues GET against a path built from caller-supplied segments
// and returns the decoded JSON body.
//
// This file holds the hand-written handlers for the three operations
// cmd/gen_action_inputs refuses to generate: DockerContainerGpusInspect,
// ContainerImageStatus and ServiceImageStatus.
//
// The vendored specification types their containerId/serviceId path
// parameter "integer", but Portainer never treats either as a number:
// containerId is Docker's own 64-character hex container ID, and serviceId
// is Docker Swarm's own alphanumeric service ID (see docs/api-divergences.md
// §6.3). The generated client bakes the wrong type in on top of that —
// internal/portainer/gen/client.gen.go declares
// DockerContainerGpusInspect(ctx, environmentId int, containerId int, ...) —
// so even a catalog action publishing the correct string type could never
// call it: cmd/gen_action_inputs's own path-argument type check refuses to
// bind a string Input field to an int client parameter, which is exactly why
// these three exist here instead of in actions.go. Each builds its request
// directly through portainer.Client.Do, the same way system.go's
// systemUpgrade does for its own generation-refused operation, and reads the
// response with portainer.ClassifyResponse before touching the body.
//
// Every path segment built from a caller-supplied identifier goes through
// url.PathEscape. portainer.Client.Do already refuses a decoded ".." path
// segment on its own, but that guard does not stop an identifier containing
// a literal "/" from splicing extra segments into the route entirely — a
// container or service ID is caller-supplied, and nothing in the wire format
// guarantees it stays hex or alphanumeric forever. Escaping the segment is
// what keeps a path parameter from being read as more than one path
// component, whether or not it also happens to contain "..".
func dockerGetJSON(ctx context.Context, c *portainer.Client, label, path string) (any, error) {
	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%s: read response: %w", label, err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}

	var out any
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", label, err)
	}
	return out, nil
}

// refreshQuery renders the "refresh" query string ContainerImageStatus and
// ServiceImageStatus both accept, or "" when the caller left it unset —
// url.Values.Encode so the boolean is rendered the same way the generated
// client's own apigen.ContainerImageStatusParams would.
func refreshQuery(refresh *bool) string {
	if refresh == nil {
		return ""
	}
	values := url.Values{}
	values.Set("refresh", fmt.Sprintf("%t", *refresh))
	return "?" + values.Encode()
}

func dockerContainerGpusInspect(ctx context.Context, c *portainer.Client, raw json.RawMessage) (any, error) {
	var in dockerContainerGpusInspectInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("docker container gpus inspect: decode input: %w", err)
	}
	path := fmt.Sprintf("/docker/%d/containers/%s/gpus", in.EnvironmentID, url.PathEscape(in.ContainerID))
	return dockerGetJSON(ctx, c, "docker container gpus inspect", path)
}

func containerImageStatus(ctx context.Context, c *portainer.Client, raw json.RawMessage) (any, error) {
	var in containerImageStatusInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("container image status: decode input: %w", err)
	}
	path := fmt.Sprintf("/docker/%d/containers/%s/image_status", in.EnvironmentID, url.PathEscape(in.ContainerID)) + refreshQuery(in.Refresh)
	return dockerGetJSON(ctx, c, "container image status", path)
}

func serviceImageStatus(ctx context.Context, c *portainer.Client, raw json.RawMessage) (any, error) {
	var in serviceImageStatusInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("service image status: decode input: %w", err)
	}
	path := fmt.Sprintf("/docker/%d/services/%s/image_status", in.EnvironmentID, url.PathEscape(in.ServiceID)) + refreshQuery(in.Refresh)
	return dockerGetJSON(ctx, c, "service image status", path)
}
