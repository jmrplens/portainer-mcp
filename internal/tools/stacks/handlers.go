package stacks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// uploadFilename is the name written into the Content-Disposition of the
// multipart "file" part on both of this domain's upload routes.
//
// A constant rather than an Input field, for the reason custom_templates'
// namesake states: neither route's multipart schema declares a filename
// property, so a caller has nothing to say about it and a knob with no
// declared effect is noise a model has to reason about.
//
// A name is nonetheless required, and mechanically so. Go's multipart reader
// — which is what Portainer parses these bodies with — files a part under
// Form.File only when its Content-Disposition carries a filename, so a
// "file" part written without one reaches the server's own lookup as nothing
// at all and the upload fails reporting the field missing.
// portainer.MultipartForm.File refuses an empty filename for exactly that
// reason. What Portainer does with the value afterwards was measured for
// custom_templates' route and not for these two, so this constant claims
// only what it has to: it is a valid name for the content, and ".yml" suits
// both a Compose file and a Swarm stack file.
const uploadFilename = "stack.yml"

// stackCreateDockerStandaloneFile is the hand-written handler for operation
// StackCreateDockerStandaloneFile (POST /stacks/create/standalone/file).
//
// Hand-written because oapi-codegen emitted no
// StackCreateDockerStandaloneFileWithResponse: the route's only declared
// request body is multipart/form-data, which the generator cannot type, so
// it emitted StackCreateDockerStandaloneFileWithBodyWithResponse instead — a
// name cmd/gen_action_inputs's clientMethodFor, which looks the method up by
// the operation's own name, can never find. The refusal is permanent
// regardless of how this domain is authored; see stacks.go's package doc.
//
// It does NOT bypass the generated client, which is what separates these two
// handlers from custom_templates' hand-written list and from docker's three:
// the WithBody variant takes (ctx, params, contentType, body io.Reader), so
// the multipart body built below is handed to the generated call and the
// path, the query encoding and the response decoding stay generated.
//
// Everything else follows the twenty-two generated handlers in actions.go
// exactly — unmarshal the Input, unmarshal the query parameters, call,
// toolutil.Check, redact — and deliberately so: the shape being identical is
// what lets a reader check this one against its neighbours. The redaction
// call is the part that must not drift. redactStackCreateDockerStandaloneFile
// is a real function this domain declares (checkCredentialRedaction refuses
// to generate any handler for a PortainereeStack-returning operation without
// it), but nothing mechanical forces a *hand-written* handler to call it —
// the generator that would have is exactly the thing that refused this
// operation. The discriminating test is in handlers_test.go.
func stackCreateDockerStandaloneFile(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackCreateDockerStandaloneFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneFile: parse input: %w", err)
	}
	var queryParams apigen.StackCreateDockerStandaloneFileParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneFile: parse query parameters: %w", err)
	}
	body, contentType, err := stackCreateDockerStandaloneFileBody(params)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneFile: %w", err)
	}
	resp, err := c.API.StackCreateDockerStandaloneFileWithBodyWithResponse(ctx, &queryParams, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneFile: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateDockerStandaloneFile: %w", err)
	}
	return redactStackCreateDockerStandaloneFile(resp.JSON200), nil
}

// stackCreateDockerStandaloneFileBody renders the Input into the multipart
// body this route requires, returning it with the content type that carries
// the generated boundary.
//
// The part names are the vendored schema's own — "Name" and "Env"
// capitalised, "file" not, which is the document's own inconsistency and not
// a transcription slip here. Multipart part names are matched literally
// rather than case-folded like a header, so "name" or "File" would reach a
// server that then reports the field missing, and this function is where the
// Input's lowerCamelCase JSON names become the wire's.
//
// EndpointID is absent by design: it is a query parameter on this route, not
// a part, and the handler above passes it through
// apigen.StackCreateDockerStandaloneFileParams.
//
// Name is written unconditionally because the route lists it required and
// toolutil.ActionSpec.ValidateInput has already refused a request without
// it. Env and file are optional in the vendored schema and go through the
// paths that emit no part at all when the caller left them unset — an empty
// Env part would be a present field holding a JSON document that fails to
// parse, which is a different request from one that omits it, and an empty
// file part would be an empty stack file rather than no stack file.
func stackCreateDockerStandaloneFileBody(params stackCreateDockerStandaloneFileInput) (io.Reader, string, error) {
	form := portainer.NewMultipartForm()

	form.Field("Name", params.Name)
	form.OptionalField("Env", params.Env)
	if params.File != nil {
		form.File("file", uploadFilename, []byte(*params.File))
	}

	body, contentType, err := form.Build()
	if err != nil {
		return nil, "", fmt.Errorf("build multipart body: %w", err)
	}
	return body, contentType, nil
}

// stackCreateDockerSwarmFile is the hand-written handler for operation
// StackCreateDockerSwarmFile (POST /stacks/create/swarm/file).
//
// The same shape, and hand-written for the same reason, as
// stackCreateDockerStandaloneFile above: only
// StackCreateDockerSwarmFileWithBodyWithResponse exists. It differs in the
// route it calls, in the SwarmID part its body can carry, and in one thing
// the vendored document decides rather than this file — its schema declares
// no required array, so even Name goes through the optional path here.
func stackCreateDockerSwarmFile(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackCreateDockerSwarmFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmFile: parse input: %w", err)
	}
	var queryParams apigen.StackCreateDockerSwarmFileParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmFile: parse query parameters: %w", err)
	}
	body, contentType, err := stackCreateDockerSwarmFileBody(params)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmFile: %w", err)
	}
	resp, err := c.API.StackCreateDockerSwarmFileWithBodyWithResponse(ctx, &queryParams, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmFile: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackCreateDockerSwarmFile: %w", err)
	}
	return redactStackCreateDockerSwarmFile(resp.JSON200), nil
}

// stackCreateDockerSwarmFileBody renders the Input into this route's
// multipart body.
//
// Every part is optional, which is the vendored schema's doing: POST
// /stacks/create/swarm/file declares no required array, so ValidateInput
// refuses nothing and a caller can legitimately produce a body with no Name
// part. Writing Name unconditionally instead — through Field, which emits
// even an empty value — would send Portainer an explicit empty stack name
// for a caller who simply said nothing, which is a different request.
//
// SwarmID is a string part, matching the vendored type and
// stacks.create_docker_swarm_string's own swarmId; only stacks.associate
// declares that concept an integer.
func stackCreateDockerSwarmFileBody(params stackCreateDockerSwarmFileInput) (io.Reader, string, error) {
	form := portainer.NewMultipartForm()

	form.OptionalField("Name", params.Name)
	form.OptionalField("SwarmID", params.SwarmID)
	form.OptionalField("Env", params.Env)
	if params.File != nil {
		form.File("file", uploadFilename, []byte(*params.File))
	}

	body, contentType, err := form.Build()
	if err != nil {
		return nil, "", fmt.Errorf("build multipart body: %w", err)
	}
	return body, contentType, nil
}

// stackMigrate is the hand-written handler for operation StackMigrate (POST
// /stacks/{id}/migrate).
//
// Hand-written for a reason unrelated to the two handlers above: the
// generated client does declare StackMigrateWithResponse, and this handler
// calls it. What cmd/gen_action_inputs refused is one line of the handler it
// would have written.
//
// A generated handler distributes an operation's query parameters by
// unmarshalling the caller's raw input straight into apigen's Params struct
// — `json.Unmarshal(input, &queryParams)`, as the twenty-two in actions.go
// all do. That works because a generated Input's wire names and the Params
// struct's json tags come from the same specification. On this one route
// they do not agree, and the disagreement is silent:
// apigen.StackMigrateParams's only field is tagged
// `json:"endpointId,omitempty"`, and "endpointId" is the name the REQUEST
// BODY's required EndpointID property publishes here (internal/specnaming
// gives the body the plain name and qualifies the query parameter as
// "endpointIdQuery"). An unmarshal of the raw input would therefore read the
// migration TARGET into the pre-1.18 fixup parameter, and drop whatever the
// caller actually put in endpointIdQuery. Nothing fails: the request is
// well-formed, the server answers 200, and the stack migrates using a
// parameter the caller never set. buildHandlerSpec refuses the operation
// naming both names rather than emit that.
//
// So the query parameter is built by hand, from the one Input field that
// means it, and that assignment is the whole of this handler's deviation
// from its generated neighbours. The body is still unmarshalled from the raw
// input exactly as a generated handler would: the body side of this route
// was never the problem, "endpointIdQuery" matches no field of
// StacksStackMigratePayload (Go's case-insensitive fallback folds
// "endpointid", not "endpointidquery"), and keeping that line generated-shaped
// means a body property added to inputs.go later reaches the wire without a
// second edit here.
//
// The split is pinned by TestUnit_MigrateRequest_SendsTheQueryFieldAsTheQueryAndTheBodyFieldAsTheBody,
// which sends two DIFFERENT values and asserts each arrives where it belongs.
// A test that sent one value for both, or asserted only the status, would
// pass against a handler that had them exactly backwards.
//
// redactStackMigrate is called for the same reason the two handlers above
// call theirs: POST /stacks/{id}/migrate answers with a PortainereeStack,
// which reaches GitConfig.Authentication.Password, and nothing mechanical
// forces a hand-written handler to redact — checkCredentialRedaction only
// proves the wrapper is declared. TestUnit_MigrateWithGitCredentialInResponse_ReturnsNoCredential
// is what proves it is called.
func stackMigrate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params stackMigrateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("StackMigrate: parse input: %w", err)
	}
	// By hand, never json.Unmarshal(input, &queryParams): see this function's
	// doc comment. EndpointIDQuery is a pointer and stays nil when the caller
	// said nothing, which is what keeps the parameter off the URL entirely
	// rather than sending endpointId=0.
	queryParams := apigen.StackMigrateParams{EndpointId: params.EndpointIDQuery}
	var body apigen.StackMigrateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("StackMigrate: parse request body: %w", err)
	}
	resp, err := c.API.StackMigrateWithResponse(ctx, params.ID, &queryParams, body)
	if err != nil {
		return nil, fmt.Errorf("StackMigrate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StackMigrate: %w", err)
	}
	return redactStackMigrate(resp.JSON200), nil
}
