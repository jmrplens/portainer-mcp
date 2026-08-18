package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// This file holds the nine hand-written handlers of this domain.
//
// Five are here because cmd/gen_action_inputs refused to generate them and
// named the reason; three distinct reasons, none fixable by authoring the
// domain differently. The remaining four generated perfectly well and were
// wrong about the server — each carries its own doc comment with the
// measurement that established it, and each was found by calling the action
// rather than by any audit. The five refusals:
//
//   - EndpointCreate and EndpointDockerBrowsePut declare a multipart/form-data
//     request body and nothing else, so oapi-codegen emitted only
//     EndpointCreateWithBodyWithResponse / EndpointDockerBrowsePutWithBodyWithResponse
//     — signatures clientMethodFor, which looks the method up by the
//     operation's own name, can never find. The typed client method does
//     exist; it is the *name* the generator expects that does not. Both call
//     it below, through portainer.MultipartForm, exactly as
//     internal/tools/custom_templates and internal/tools/stacks do for their
//     own multipart routes.
//   - EndpointList and EndpointUpdate each carry a specification "number"
//     (rendered *float64) against a generated client field of *float32.
//     checkWireWidth refuses to bind a parameter whose round-trip it cannot
//     prove lossless and asks for a person's decision instead. The decision
//     taken here is registries.configure's: refuse a value that does not
//     survive the narrowing rather than silently truncate it.
//   - SnapshotContainerInspect's containerId is an opaque Docker container ID
//     that the vendored specification mistypes "integer"
//     (docs/api-divergences.md §6.3). The generated client bakes that in —
//     SnapshotContainerInspectWithResponse(ctx, environmentId int,
//     containerId int) — so no correctly-typed action could ever call it. It
//     builds its request directly, the same way internal/tools/docker's three
//     handlers do for the same defect on their own routes.
//
// Everything else follows the generated handlers in actions.go exactly —
// unmarshal the Input, call, toolutil.Check, redact — and deliberately so:
// the shape being identical is what lets a reader check these nine against
// their eighteen generated neighbours. The redaction calls are the part
// that must not drift. redactEndpointList, redactEndpointCreate and
// redactEndpointUpdate are real functions this domain declares (the
// generator refuses to emit any handler for a PortainereeEndpoint-returning
// operation without them), but nothing mechanical forces a *hand-written*
// handler to call one — the generator that would have is exactly the thing
// that refused these operations. handlers_test.go is what discriminates.

// uploadFilename is the name written into the Content-Disposition of every
// file part this domain sends.
//
// Neither multipart route's schema declares a filename and Portainer reads
// neither, but Go's multipart writer decides whether a part is a file at all
// on that header's presence, and portainer.MultipartForm.File refuses an
// empty one rather than let the server report a plainly-sent field missing.
// Same constant, same reasoning, as custom_templates and stacks.
const uploadFilename = "upload.dat"

// toFloat32 narrows a specification "number" to the float32 the generated
// client declares, refusing rather than truncating.
//
// json.Unmarshal would perform this conversion silently on the way into the
// client's own type — it only errors above float32's range — so the values
// this guard rejects are exactly the ones that would otherwise reach
// Portainer rounded, with nothing said. Precision loss on a check-in window
// or a CPU-usage sample is not catastrophic, which is precisely why it would
// never be noticed; refusing keeps the caller's number and the sent number
// the same number, or says why they cannot be.
func toFloat32(label string, v *float64) (*float32, error) {
	if v == nil {
		return nil, nil
	}
	f := *v
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, fmt.Errorf("%s: %v is not a finite number", label, f)
	}
	if f > math.MaxFloat32 || f < -math.MaxFloat32 {
		return nil, fmt.Errorf("%s: %v does not fit in the float32 this route's parameter is sent as", label, f)
	}
	narrowed := float32(f)
	if float64(narrowed) != f {
		return nil, fmt.Errorf("%s: %v cannot be sent without losing precision (it would become %v)", label, f, float64(narrowed))
	}
	return &narrowed, nil
}

// endpointList lists environments.
//
// The query parameters round-trip into the generated client's own params
// type through encoding/json, exactly as every generated handler's do, with
// one exception decoded separately: edgeCheckInPassedSeconds, whose *float64
// is what refused this operation generation in the first place. Decoding it
// into apigen.EndpointListParams directly would work and would round the
// value, so it is read as the float64 the specification declares and
// narrowed explicitly.
func endpointList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointList: parse input: %w", err)
	}
	var queryParams apigen.EndpointListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("EndpointList: parse query parameters: %w", err)
	}
	narrowed, err := toFloat32("EndpointList: edgeCheckInPassedSeconds", params.EdgeCheckInPassedSeconds)
	if err != nil {
		return nil, err
	}
	queryParams.EdgeCheckInPassedSeconds = narrowed

	resp, err := c.API.EndpointListWithResponse(ctx, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("EndpointList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointList: %w", err)
	}
	return redactEndpointList(resp.JSON200), nil
}

// endpointUpdate updates one environment.
//
// The body round-trips into the generated client's own request type the way
// every generated handler's does. The four Kubernetes snapshot performance
// metrics are checked first, against the Input's own faithful *float64
// declaration, because the round-trip would otherwise narrow each to the
// client's *float32 silently.
func endpointUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointUpdate: parse input: %w", err)
	}
	if err := checkSnapshotMetricsFit(params.Kubernetes); err != nil {
		return nil, fmt.Errorf("EndpointUpdate: %w", err)
	}
	var body apigen.EndpointUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointUpdate: parse request body: %w", err)
	}
	resp, err := c.API.EndpointUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointUpdate: %w", err)
	}
	return redactEndpointUpdate(resp.JSON200), nil
}

// checkSnapshotMetricsFit refuses an endpoints.update whose Kubernetes
// snapshot metrics would be rounded on the way to the client's *float32.
//
// It reports the offending value's own path — snapshots[3].cpuUsage rather
// than "a metric" — because a caller sending a snapshot array has no other
// way to find which element the complaint is about.
func checkSnapshotMetricsFit(k *endpointUpdateInputKubernetes) error {
	if k == nil {
		return nil
	}
	for i := range k.Snapshots {
		m := k.Snapshots[i].PerformanceMetrics
		if m == nil {
			continue
		}
		for _, f := range []struct {
			name  string
			value *float64
		}{
			{"cpuUsage", m.CPUUsage},
			{"diskUsage", m.DiskUsage},
			{"memoryUsage", m.MemoryUsage},
			{"networkUsage", m.NetworkUsage},
		} {
			if _, err := toFloat32(fmt.Sprintf("kubernetes.snapshots[%d].performanceMetrics.%s", i, f.name), f.value); err != nil {
				return err
			}
		}
	}
	return nil
}

// endpointCreate registers a new environment.
func endpointCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointCreateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointCreate: parse input: %w", err)
	}
	body, contentType, err := endpointCreateBody(params)
	if err != nil {
		return nil, fmt.Errorf("EndpointCreate: %w", err)
	}
	resp, err := c.API.EndpointCreateWithBodyWithResponse(ctx, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointCreate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointCreate: %w", err)
	}
	return redactEndpointCreate(resp.JSON200), nil
}

// endpointCreateBody renders the Input into the multipart body this route
// requires, returning it with the content type that carries the generated
// boundary.
//
// The part names are the vendored schema's own, capitalised
// (Name, EndpointCreationType, URL, TLSCACertFile and the rest) rather than
// the lowerCamelCase the Input publishes to a model. Multipart part names
// are not JSON keys and Portainer matches them literally, so the two
// casings are not interchangeable and this function is where the one
// becomes the other.
//
// Two parts are written unconditionally because the route lists them
// required and toolutil.ActionSpec.ValidateInput has already refused a
// request missing either. Every other part goes through an Optional*
// method, which emits no part at all when the caller left the field unset —
// an empty TLSCACertFile part is a present-but-empty certificate, which is
// a different request from one that omits it.
func endpointCreateBody(params endpointCreateInput) (io.Reader, string, error) {
	form := portainer.NewMultipartForm()

	form.Field("Name", params.Name)
	form.IntField("EndpointCreationType", params.EndpointCreationType)

	form.OptionalField("AzureApplicationID", params.AzureApplicationID)
	form.OptionalField("AzureAuthenticationKey", params.AzureAuthenticationKey)
	form.OptionalField("AzureTenantID", params.AzureTenantID)
	form.OptionalField("ContainerEngine", params.ContainerEngine)
	form.OptionalField("CustomTemplateContent", params.CustomTemplateContent)
	form.OptionalIntField("CustomTemplateID", params.CustomTemplateID)
	form.OptionalBoolField("EdgeAsyncMode", params.EdgeAsyncMode)
	form.OptionalIntField("EdgeCheckinInterval", params.EdgeCheckinInterval)
	form.OptionalIntField("EdgeCommandInterval", params.EdgeCommandInterval)
	form.OptionalIntField("EdgePingInterval", params.EdgePingInterval)
	form.OptionalIntField("EdgeSnapshotInterval", params.EdgeSnapshotInterval)
	form.OptionalField("EdgeTunnelServerAddress", params.EdgeTunnelServerAddress)
	form.OptionalField("Gpus", params.Gpus)
	form.OptionalIntField("GroupID", params.GroupID)
	form.OptionalField("KubeConfig", params.KubeConfig)
	form.OptionalField("PublicURL", params.PublicURL)
	form.OptionalField("StackName", params.StackName)
	form.OptionalBoolField("TLS", params.TLS)
	form.OptionalBoolField("TLSSkipClientVerify", params.TLSSkipClientVerify)
	form.OptionalBoolField("TLSSkipVerify", params.TLSSkipVerify)
	form.OptionalField("TagIds", params.TagIDs)
	form.OptionalField("URL", params.URL)

	// The three format:binary parts, written as file parts rather than
	// ordinary fields: the schema types them uploads, and Portainer's own
	// handler reads them with a multipart file reader.
	optionalFile(form, "TLSCACertFile", params.TLSCACertFile)
	optionalFile(form, "TLSCertFile", params.TLSCertFile)
	optionalFile(form, "TLSKeyFile", params.TLSKeyFile)

	body, contentType, err := form.Build()
	if err != nil {
		return nil, "", fmt.Errorf("build multipart body: %w", err)
	}
	return body, contentType, nil
}

// optionalFile writes a file part only when the caller supplied one.
//
// portainer.MultipartForm has Optional* methods for every ordinary field
// type but not for a file, because no route it served before this one had
// an optional upload. Adding one there would be the wider change; this is
// the local one.
func optionalFile(form *portainer.MultipartForm, name string, content *string) {
	if content == nil {
		return
	}
	form.File(name, uploadFilename, []byte(*content))
}

// endpointDockerBrowsePut uploads one file into an environment's filesystem.
func endpointDockerBrowsePut(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointDockerBrowsePutInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointDockerBrowsePut: parse input: %w", err)
	}
	form := portainer.NewMultipartForm()
	form.Field("Path", params.Path)
	form.File("file", uploadFilename, []byte(params.File))
	body, contentType, err := form.Build()
	if err != nil {
		return nil, fmt.Errorf("EndpointDockerBrowsePut: build multipart body: %w", err)
	}

	query := apigen.EndpointDockerBrowsePutParams{VolumeID: params.VolumeID}
	resp, err := c.API.EndpointDockerBrowsePutWithBodyWithResponse(ctx, params.ID, &query, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointDockerBrowsePut: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointDockerBrowsePut: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// snapshotContainerInspectMaxBody bounds how much of the response is read.
//
// 1 MiB, following internal/tools/docker/handlers.go: this response is one
// container's stored snapshot entry, which does not grow with the size of
// the estate.
const snapshotContainerInspectMaxBody = 1 << 20

// snapshotContainerInspect reads one container out of an environment's
// stored Docker snapshot.
//
// The path is built by hand because the generated client's containerId
// argument is an int and a Docker container ID is not — see this file's own
// doc comment. environmentId is an int and is formatted as one; containerId
// goes through url.PathEscape for the reason internal/tools/docker's
// handlers record: portainer.Client.Do already refuses a decoded ".."
// segment, but nothing stops a caller-supplied identifier containing a
// literal "/" from splicing extra segments into the route, and nothing in
// the wire format guarantees a container ID stays hexadecimal forever.
func snapshotContainerInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params snapshotContainerInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("SnapshotContainerInspect: parse input: %w", err)
	}
	path := fmt.Sprintf("/docker/%d/snapshot/containers/%s",
		params.EnvironmentID, url.PathEscape(params.ContainerID))

	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("SnapshotContainerInspect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, snapshotContainerInspectMaxBody))
	if err != nil {
		return nil, fmt.Errorf("SnapshotContainerInspect: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("SnapshotContainerInspect: %w", err)
	}
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("SnapshotContainerInspect: decode response: %w", err)
	}
	return out, nil
}

// endpointSettingsUpdate changes one environment's per-environment settings.
//
// Hand-written for a reason none of the other four share, and the one
// correctness problem in this domain that no refusal from the generator
// pointed at: the two editions accept these ten security settings under
// different shapes, and each silently ignores the other's.
//
// Measured 2026-08-18 against a live Portainer 2.44.0 of each edition, on
// PUT /endpoints/1/settings:
//
//	Community, flat top-level fields ............ 200, applied
//	Community, nested "securitySettings" ........ 200, IGNORED
//	Business,  nested "securitySettings" ........ 200, applied
//	Business,  flat top-level fields ............ 200, IGNORED
//	either edition, BOTH shapes at once ......... 200, that edition's own shape wins
//
// The ignored case answers 200 and echoes back the environment with the
// settings unchanged, so a caller asking to forbid privileged containers is
// told the call succeeded while nothing was forbidden. That is the whole
// reason this is not left to the generated handler: this catalog is
// generated from the Business specification alone, so the generated handler
// sends the nested shape, and against a Community server every call of it
// would have been a silent no-op on the fields that are the point of the
// action.
//
// Sending both shapes is what makes one published field correct on either
// server, and the last row above is what makes it unambiguous rather than a
// gamble: where both are present, each edition takes its own and discards
// the other, so the two copies can never disagree in effect.
func endpointSettingsUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointSettingsUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointSettingsUpdate: parse input: %w", err)
	}
	body, err := endpointSettingsUpdateBody(input, params.SecuritySettings)
	if err != nil {
		return nil, fmt.Errorf("EndpointSettingsUpdate: %w", err)
	}
	resp, err := c.API.EndpointSettingsUpdateWithBodyWithResponse(
		ctx, params.ID, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("EndpointSettingsUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointSettingsUpdate: %w", err)
	}
	return redactEndpointSettingsUpdate(resp.JSON200), nil
}

// endpointSettingsUpdateBody renders the request body carrying both shapes.
//
// It works on the caller's own JSON rather than re-marshalling the Input
// struct, so every field this action does not itself reason about — gpus,
// enableGPUManagement, changeWindow, deploymentOptions,
// enableImageNotification — reaches the wire exactly as the generated
// handler would have sent it, and a field added to the Input later needs no
// edit here. Only the ten security settings are copied, and only from the
// nested object the caller supplied: a caller who sent no securitySettings
// gets a body identical to the generated one.
//
// "id" is stripped: it is a path parameter, and the Input publishes it
// alongside the body fields the way every generated Input in this domain
// does. Leaving it in would send a body property the specification does not
// declare.
func endpointSettingsUpdateBody(input json.RawMessage, security *endpointSettingsUpdateInputSecuritySettings) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return nil, fmt.Errorf("parse request body: %w", err)
	}
	delete(fields, "id")

	if security != nil {
		flat, err := json.Marshal(security)
		if err != nil {
			return nil, fmt.Errorf("render Community security settings: %w", err)
		}
		var flatFields map[string]json.RawMessage
		if err := json.Unmarshal(flat, &flatFields); err != nil {
			return nil, fmt.Errorf("render Community security settings: %w", err)
		}
		// The caller's own top-level spelling, if they somehow sent one,
		// wins over the copy: this mirrors, it never overrides.
		for name, value := range flatFields {
			if _, present := fields[name]; !present {
				fields[name] = value
			}
		}
	}

	body, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("render request body: %w", err)
	}
	return body, nil
}

// snapshotContainersList reads the containers recorded in an environment's
// stored Docker snapshot.
//
// Hand-written because the generated handler cannot work at all, on any
// input. The vendored specification declares this route's 200 response a
// single portainer.DockerContainerSnapshot object, so oapi-codegen typed
// SnapshotContainersListResponse.JSON200 *PortainerDockerContainerSnapshot,
// and the server answers a JSON array: every call through the generated
// handler fails while decoding, with "json: cannot unmarshal array into Go
// value of type portainerapi.PortainerDockerContainerSnapshot".
//
// Measured 2026-08-18 against a live Business Edition 2.44.0: GET
// /api/docker/1/snapshot/containers answers 200 with a top-level JSON array
// whose elements carry Id, Names, Image, Labels and the rest of a container
// snapshot. See docs/api-divergences.md.
//
// This is the one defect in the domain that no refusal from
// cmd/gen_action_inputs pointed at and no audit would have caught: the
// generator checks a response for credential-shaped fields and a parameter
// for wire width, and neither reads whether the declared response type is
// the shape the server sends. Only calling it does. The request is built
// directly, the way snapshotContainerInspect below does, because there is no
// typed client method whose declared response type is correct to call.
func snapshotContainersList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params snapshotContainersListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("SnapshotContainersList: parse input: %w", err)
	}
	path := fmt.Sprintf("/docker/%d/snapshot/containers", params.EnvironmentID)
	if params.EdgeStackID != nil {
		query := url.Values{}
		query.Set("edgeStackId", strconv.Itoa(*params.EdgeStackID))
		path += "?" + query.Encode()
	}

	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("SnapshotContainersList: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 4 MiB rather than snapshotContainerInspect's 1 MiB: this response
	// grows with the number of containers on the environment, where that one
	// describes exactly one.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("SnapshotContainersList: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("SnapshotContainersList: %w", err)
	}
	if len(body) == 0 {
		return []any{}, nil
	}
	var out []any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("SnapshotContainersList: decode response: %w", err)
	}
	return out, nil
}

// endpointAssociationDelete detaches an edge agent from its environment.
//
// Hand-written because the generated handler cannot work on any input, and
// for a reason distinct from every other entry in this file: the vendored
// specification is wrong about the route's HTTP VERB, not about a type.
//
// Both specifications — Community and Business alike — document this
// operation as PUT /endpoints/{id}/association, so oapi-codegen emitted
// EndpointAssociationDeleteWithResponse issuing http.MethodPut. Measured
// 2026-08-18 against a live 2.44.0 of each edition, every verb on that path:
//
//	GET, POST, PUT, PATCH .... 405 Method Not Allowed
//	DELETE ................... served
//
// Only DELETE is registered. Everything except the document agrees: the
// operationId ends "Delete", the action is destructive, and the user
// interface calls it disassociating.
//
// Neither standing audit could have caught this, which is worth stating
// where the next reader will look. cmd/audit_spec_drift compares parameter
// shapes and never issues a request. cmd/audit_spec_reality does issue one,
// but classifies a route as absent only when the server answers Go's literal
// "404 page not found" (see isRouteAbsent in cmd/audit_spec_reality/probe.go
// and that command's own package doc) — and a 405 means the path IS
// registered, just not for that verb, so the probe reads it as served. A
// wrong verb is invisible to both by construction; only calling the action
// end to end finds it, which is what test/e2e/suite/endpoints_test.go does.
func endpointAssociationDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointAssociationDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointAssociationDelete: parse input: %w", err)
	}
	resp, err := c.Do(ctx, http.MethodDelete, fmt.Sprintf("/endpoints/%d/association", params.ID), nil)
	if err != nil {
		return nil, fmt.Errorf("EndpointAssociationDelete: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("EndpointAssociationDelete: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("EndpointAssociationDelete: %w", err)
	}
	if len(body) == 0 {
		return map[string]any{"status": "ok"}, nil
	}
	// The route answers with the environment, which carries the very edge
	// key this call has just invalidated plus the Azure and TLS material
	// every environment response carries. It is decoded into the generated
	// type and passed through the same wrapper the generated handler would
	// have called, rather than returned as free-form JSON: redaction is not
	// optional on this response.
	var environment apigen.PortainereeEndpoint
	if err := json.Unmarshal(body, &environment); err != nil {
		return nil, fmt.Errorf("EndpointAssociationDelete: decode response: %w", err)
	}
	return redactEndpointAssociationDelete(&environment), nil
}

// namespacesAccessUpdate grants and revokes access to one Kubernetes
// namespace.
//
// Hand-written for the same reason snapshotContainerInspect is, on the fifth
// instance of the defect docs/api-divergences.md §6.3 catalogues: the
// vendored specification declares the rpn path parameter "integer", and
// Portainer resolves that segment as the Kubernetes namespace's own NAME.
// The generated client bakes the wrong type into its signature —
// NamespacesAccessUpdateWithResponse(ctx, id int, rpn int, ...) — so no
// correctly-typed action could ever call it.
//
// Measured 2026-08-18 against a live Business Edition 2.44.0 with the
// Kubernetes leg up (`make e2e-k8s-up`):
//
//	PUT /endpoints/1/pools/default/access   -> 204
//	PUT /endpoints/1/pools/portainer/access -> 204
//	PUT /endpoints/1/pools/1/access         -> error: namespaces "1" not found
//
// The server names the cause itself in that last line, which is what makes
// this a measurement rather than an inference.
//
// This one was found only because the Kubernetes leg was brought up to give
// the action positive e2e coverage. Neither standing audit could see it —
// audit_spec_drift compares the catalog against the same document that is
// wrong, and audit_spec_reality only asks whether a route exists — and the
// domain's own negative test (the action is refused by a Docker
// environment) passed happily against a broken parameter type, because a
// Docker environment refuses it whatever the rpn is.
func namespacesAccessUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params namespacesAccessUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("NamespacesAccessUpdate: parse input: %w", err)
	}
	body, err := json.Marshal(apigen.NamespacesAccessUpdateJSONRequestBody{
		TeamsToAdd:    toIntSlice(params.TeamsToAdd),
		TeamsToRemove: toIntSlice(params.TeamsToRemove),
		UsersToAdd:    toIntSlice(params.UsersToAdd),
		UsersToRemove: toIntSlice(params.UsersToRemove),
	})
	if err != nil {
		return nil, fmt.Errorf("NamespacesAccessUpdate: render request body: %w", err)
	}

	// url.PathEscape for the reason internal/tools/docker's handlers record:
	// a Kubernetes namespace name cannot contain "/" today, but nothing here
	// guarantees the caller sends a valid one, and a segment that splices
	// extra path components into the route is a different request from the
	// one asked for.
	path := fmt.Sprintf("/endpoints/%d/pools/%s/access", params.ID, url.PathEscape(params.Rpn))
	resp, err := c.Do(ctx, http.MethodPut, path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("NamespacesAccessUpdate: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	answer, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("NamespacesAccessUpdate: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, answer); err != nil {
		return nil, fmt.Errorf("NamespacesAccessUpdate: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// toIntSlice renders an optional list of identifiers the way the generated
// request body declares it: absent when the caller sent nothing, so a call
// that only adds users does not also send four empty arrays.
func toIntSlice(in []int) *[]int {
	if in == nil {
		return nil
	}
	return &in
}
