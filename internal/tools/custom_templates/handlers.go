package custom_templates

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
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
// afterwards was measured after this comment was first written: Portainer
// ignores the value entirely (docs/api-divergences.md §2.5), so the constant
// is safe and there is nothing for a caller to control. ".yml" suits all three stack
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
// seven generated neighbours. The redaction call is the part that must not drift.
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
	form.IntField("Platform", params.Platform)
	form.IntField("Type", params.Type)

	form.OptionalField("Note", params.Note)
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

// customTemplateListMaxBody bounds how much of the list response is read.
//
// 4 MiB rather than the 1 MiB internal/tools/docker/handlers.go uses,
// because this response grows with the number of templates on the server
// where docker's grows with nothing. Truncation is not silent either way: a
// body cut mid-array fails to unmarshal and the caller gets a decode error,
// never a short list presented as the whole one.
const customTemplateListMaxBody = 4 << 20

// customTemplateList is the second hand-written handler in this domain, and
// unlike customTemplateCreateFile it exists because the generated one is
// WRONG on the wire rather than absent.
//
// GET /custom_templates declares its required `type` parameter as
// style: form, explode: false, so the generated client renders a
// three-element slice as one comma-joined value —
// runtime.StyleParamWithOptions("form", false, "type", ...) in
// NewCustomTemplateListRequest. Portainer's own handler parses each value
// with strconv.Atoi and answers:
//
//	GET /custom_templates?type=1,2,3
//	400 "Invalid Custom template type: Failed parsing template type:
//	     strconv.Atoi: parsing \"1,2,3\": invalid syntax"
//
// while the repeated form it expects answers 200:
//
//	GET /custom_templates?type=1&type=2&type=3   -> 200
//
// Both measured against a live 2.44.0, Community and Business Edition alike
// (docs/api-divergences.md §6.7). The document is wrong, not either
// implementation, and the cost of publishing the generated call is that the
// most obvious use of a list action — every template, whatever its type —
// is the one call that fails, with no way to avoid it: the parameter is
// required, so there is no "omit it and get everything" escape.
//
// So the query is built here instead. url.Values.Encode renders a repeated
// key per value, which is exactly the encoding the server accepts. Nothing
// about the PUBLISHED parameter shape changes — customTemplateListInput
// still declares type as []int and edge as an optional bool — so this is a
// wire-encoding fix, not a schema divergence, and it needs no
// api/spec-drift-allowlist.yaml entry: cmd/audit_spec_drift compares the
// catalog's published input against the vendored one and sees no difference
// to report.
//
// The redaction wrapper is called for the same reason customTemplateCreateFile
// calls its own: the generator's guard requires redactCustomTemplateList to
// be declared, but nothing mechanical forces a hand-written handler to
// actually call it, and this response carries GitConfig for every git-backed
// template in the list.
// customTemplateListPath is the route customTemplateList builds by hand.
// Named rather than inlined so TestUnit_CustomTemplateListPath_MatchesThe
// VendoredSpecification can compare it against internal/apiversion's
// specification-generated table instead of against another copy of the
// same literal.
const customTemplateListPath = "/custom_templates"

func customTemplateList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params customTemplateListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("CustomTemplateList: parse input: %w", err)
	}

	query := url.Values{}
	for _, stackType := range params.Type {
		query.Add("type", strconv.Itoa(stackType))
	}
	if params.Edge != nil {
		query.Set("edge", strconv.FormatBool(*params.Edge))
	}
	path := customTemplateListPath
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateList: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, customTemplateListMaxBody))
	if err != nil {
		return nil, fmt.Errorf("CustomTemplateList: read response: %w", err)
	}
	if err := portainer.ClassifyResponse(resp.StatusCode, body); err != nil {
		return nil, fmt.Errorf("CustomTemplateList: %w", err)
	}

	// An empty body answers as an empty list rather than as null: the
	// generated handler returned the nil *[]PortainereeCustomTemplate its
	// decoder left behind, and "no templates" reads better to a model as []
	// than as null.
	templates := []apigen.PortainereeCustomTemplate{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &templates); err != nil {
			return nil, fmt.Errorf("CustomTemplateList: decode response: %w", err)
		}
	}
	return redactCustomTemplateList(&templates), nil
}
