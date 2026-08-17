package custom_templates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// uploadFilename is the name written into the Content-Disposition of the
// multipart "File" part.
//
// It is a constant rather than an Input field on purpose. The vendored
// multipart schema for this route declares ten properties and a filename is
// not one of them, so a caller has nothing to say about it and giving a
// model a knob with no declared effect is noise it has to reason about.
//
// A name is nonetheless required, and not as decoration: Go's multipart
// reader — which is what Portainer parses this body with — files a part
// under Form.File only when its Content-Disposition carries a filename, so a
// "File" part written without one reaches the server's own lookup as
// nothing at all and the upload fails reporting the field missing. That is
// the mechanical reason for the value; what Portainer does with the name
// afterwards is unmeasured, because this route was never exercised against a
// live server (its two JSON siblings were). ".yml" suits all three stack
// types this route accepts — a Swarm or Compose stack file and a Kubernetes
// manifest alike.
const uploadFilename = "template.yml"

// customTemplateCreateFile is the hand-written handler for operation
// CustomTemplateCreateFile (POST /custom_templates/create/file).
//
// This file exists for one operation for one reason: oapi-codegen emitted no
// CustomTemplateCreateFileWithResponse. The route's only declared request
// body is multipart/form-data, which the generator cannot type, so it
// emitted CustomTemplateCreateFileWithBodyWithResponse(ctx, contentType,
// body io.Reader) instead — a signature cmd/gen_action_inputs's
// clientMethodFor, which looks the method up by the operation's own name,
// can never find. The refusal is permanent regardless of how this domain is
// authored; see custom_templates.go's package doc.
//
// Everything else follows the generated handlers in actions.go exactly —
// unmarshal the Input, call, toolutil.Check, redact — and deliberately so:
// the shape being identical is what lets a reader check this one against its
// eight neighbours. The redaction call is the part that must not drift.
// redactCustomTemplateCreateFile is a real function this domain declares
// (checkCredentialRedaction refuses to generate any handler for a
// PortainereeCustomTemplate-returning operation without it), but nothing
// mechanical forces a *hand-written* handler to call it — the generator that
// would have is exactly the thing that refused this operation. The
// discriminating test is in handlers_test.go.
func customTemplateCreateFile(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateCreateFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateFile: parse input: %w", err)
	}
	body, contentType, err := customTemplateCreateFileBody(params)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateFile: %w", err)
	}
	resp, err := c.API.CustomTemplateCreateFileWithBodyWithResponse(ctx, contentType, body)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateFile: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("CustomTemplateCreateFile: %w", err)
	}
	return redactCustomTemplateCreateFile(resp.JSON200), nil
}

// customTemplateCreateFileBody renders the Input into the multipart body
// this route requires, returning it with the content type that carries the
// generated boundary.
//
// The part names are the vendored schema's own, capitalised
// (Title, Description, Note, Platform, Type, File, Logo, EdgeSettings,
// EdgeTemplate, Variables) rather than the lowerCamelCase the Input publishes
// to a model. Multipart part names are not JSON keys and Portainer matches
// them literally, so the two casings are not interchangeable and this
// function is where the one becomes the other.
//
// Six parts are written unconditionally because the route lists them
// required and toolutil.ActionSpec.ValidateInput has already refused a
// request missing any of them. The four optional ones go through the
// Optional* methods, which emit no part at all when the caller left the
// field unset — an empty EdgeSettings part would be a present field whose
// JSON fails to parse, which is a different request from one that omits it.
//
// EdgeSettings and Variables are written as the caller's own strings and are
// never marshalled here. This route's schema types both "string" — a
// JSON-encoded document inside a form field — where the two JSON create
// routes take a real nested object and a real array; handing a Go struct to
// the marshaller here would produce a part Portainer then fails to
// unmarshal. See customTemplateCreateFileInput's field comments.
func customTemplateCreateFileBody(params customTemplateCreateFileInput) (io.Reader, string, error) {
	form := portainer.NewMultipartForm()

	form.Field("Title", params.Title)
	form.Field("Description", params.Description)
	form.Field("Note", params.Note)
	form.IntField("Platform", params.Platform)
	form.IntField("Type", params.Type)

	form.OptionalField("Logo", params.Logo)
	form.OptionalField("EdgeSettings", params.EdgeSettings)
	form.OptionalBoolField("EdgeTemplate", params.EdgeTemplate)
	form.OptionalField("Variables", params.Variables)

	form.File("File", uploadFilename, []byte(params.File))

	body, contentType, err := form.Build()
	if err != nil {
		return nil, "", fmt.Errorf("build multipart body: %w", err)
	}
	return body, contentType, nil
}
