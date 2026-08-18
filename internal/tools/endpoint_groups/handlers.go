package endpoint_groups

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// This file holds the one hand-written handler of this domain.
//
// endpointGroupInspect is hand-written for a reason none of the six
// generated handlers in actions.go share, and one no domain package can
// author its way out of: GET /endpoint_groups/{id} carries no operationId in
// either vendored document. oapi-codegen derives a generated method from an
// operationId and emits nothing at all for an operation without one, so
// there is no EndpointGroupInspectWithResponse to call — not a method named
// something else (the case internal/tools/endpoints/handlers.go's two
// multipart refusals describe), simply no method. The request is therefore
// built directly through portainer.Client.Do, exactly as that file's
// snapshotContainerInspect does for its own reason.
//
// The name EndpointGroupInspect is not invented here. It comes from
// internal/specnaming's table, which is also what cmd/gen_applicability
// writes into internal/apiversion's operationIDs index for both editions —
// the index internal/tools/actioncatalog resolves this action's Edition
// through. A different name here, however reasonable, would resolve against
// nothing and refuse to build.

// endpointGroupInspectMaxBody bounds how much of the response is read.
//
// 1 MiB, following internal/tools/endpoints/handlers.go's
// snapshotContainerInspect: this response is one environment group's own
// record — Id, Name, Description, its tag ids, a Total and a small TypeInfo
// breakdown, plus Policies on Business Edition — and none of those grow with
// the size of the estate. Measured against a live server, the largest
// response seen was 138 bytes.
const endpointGroupInspectMaxBody = 1 << 20

// endpointGroupInspect reads one environment group.
//
// size is rendered through url.Values.Encode, the way the generated client
// renders EndpointGroupList's identically-named query parameter, rather than
// concatenated by hand; it is omitted entirely when the caller leaves it
// unset, which is the difference between Total reading 0 and Total reading
// the group's real membership count (see this domain's narrative for
// EndpointGroupInspect, measured on both editions).
//
// id is formatted as the int it is, so nothing caller-supplied can splice
// extra segments into the route.
func endpointGroupInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointGroupInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointGroupInspect: parse input: %w", err)
	}

	path := fmt.Sprintf("/endpoint_groups/%d", params.ID)
	if params.Size != nil {
		query := url.Values{}
		query.Set("size", fmt.Sprintf("%t", *params.Size))
		path += "?" + query.Encode()
	}

	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupInspect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, endpointGroupInspectMaxBody))
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupInspect: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("EndpointGroupInspect: %w", err)
	}
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("EndpointGroupInspect: decode response: %w", err)
	}
	return out, nil
}
