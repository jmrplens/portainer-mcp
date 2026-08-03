package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
)

// The distribution problem this file exists to solve: an operation's Input
// struct (fields.go) is flat, but the generated client method
// (internal/portainer/gen) is not — it wants some fields as positional path
// arguments, some as a query-parameters struct, and the rest as a request
// body. Measured against the generated client: every one of 429 operations'
// method signatures is purely path arguments, a query struct and/or a JSON
// body, in 12 distinct shapes (see the plan's own count). This file composes
// a handler for any of them generically, from fieldSpec.Origin and
// assembleOperationFields' pathOrder — both already computed once, when the
// Input struct was assembled — rather than re-deriving which bucket a field
// belongs to a second time.
//
// The mechanism that makes this generic rather than 12 hand-cased templates:
// every one of this handler's three groups is round-tripped through
// encoding/json directly from the caller's raw input, not assigned
// field-by-field in generated Go source. This sidesteps a real hazard that
// field-by-field assignment would not: oapi-codegen's own Go field names for
// a query parameter or body property do not always match this generator's
// (registries.registryUpdatePayload's "ProjectId" renders as this
// generator's "ProjectID" via commonInitialisms, but oapi-codegen keeps the
// wire name's own casing verbatim — see naming.go's goFieldName doc comment).
// A generated field-by-field literal would have to know that mapping per
// field; a JSON round trip only has to agree on the *tag*, and it does,
// proved by a repository-wide invariant rather than per-field enumeration:
// bodyJSONTag/JSONName never add, drop or reorder a property name's
// characters, only recase them, so lower-casing either side's tag always
// produces the same string regardless of which generator chose which case
// for which word. Query parameter names go through even less: OpenAPI
// already declares them in lower-camel-case, and both generators use that
// name verbatim, so the two tags are not just case-insensitively but
// literally equal.
// The one place this invariant does not hold is where the vendored spec's
// own Go type disagrees in width from its declared JSON Schema type —
// registries.registryConfigurePayload's TLSCACertFile is `[]int32` in the
// generated client despite a JSON Schema "integer" (Go `int`) declaration,
// which is exactly why registries.go hand-converts it. A generated handler
// hitting this would still compile and mostly work (encoding/json checks
// integer range on unmarshal into a sized int type), but a domain carrying
// this shape is better served by the escape hatch below than by trusting it
// silently.
//
// Safety this buys back, since no per-field Go-name introspection means no
// per-field cross-check either: reflection on the real generated client
// (pathArgTypesFor, responseInfoFor) verifies path-argument arity and type,
// and refuses generation the moment fields.go's own path-parameter order or
// count disagrees with what oapi-codegen actually generated for the same
// operationId — the collision-free invariant assembleOperationFields
// enforces on the JSON side, checked from the Go side too.

// pathArg is one field routed to the generated client method's positional
// path arguments, carried in call order (see assembleOperationFields'
// pathOrder), not the alphabetical order fields.go sorts Input struct
// fields into for rendering.
type pathArg struct {
	GoName string
	GoType string // "int", "string", "float64" or "bool" — the only types typeOf can produce for a required scalar path parameter
}

// handlerSpec is everything renderHandlerFunc needs to emit one operation's
// handler function body.
type handlerSpec struct {
	Domain      string
	OperationID string
	FuncName    string
	// InputStruct is the Input struct's Go type name, or "" when no path
	// argument needs one. A handler with query and/or body fields but no
	// path fields still round-trips the raw input directly into the query
	// and body types (see the package doc above) and never declares a
	// `params` variable of this type at all — declaring one it never reads
	// would be a "declared and not used" compile error, not a style choice.
	InputStruct string
	PathArgs    []pathArg
	HasQuery    bool
	HasBody     bool
	Response    responseInfo
}

// responseInfo is what handler.go needs from an operation's response shape:
// which single success field to return, or that it must refuse rather than
// guess between two.
type responseInfo struct {
	// SuccessField is the generated response type's field name for its one
	// success body, e.g. "JSON200" or "JSON204" — empty when the operation's
	// response carries no body at all (129 of 442 in the vendored client).
	SuccessField string
}

// clientType is apigen.ClientWithResponses' method set, reflected once so
// responseInfoFor and pathArgTypesFor both answer from the one real,
// compiled client rather than a second, hand-maintained table of response
// shapes or a source-level parse of internal/portainer/gen that could
// disagree with what oapi-codegen actually emitted.
var clientType = reflect.TypeOf(&apigen.ClientWithResponses{})

// clientMethodFor looks up apigen.ClientWithResponses' <operationID>WithResponse
// method. Every one of the 429 "pure" operations (see this file's package
// doc) has exactly this method, generated from the same operationId
// assembleOperationFields was called with — a WithBody-only multipart
// variant, which this generator never calls, does not shadow it.
func clientMethodFor(operationID string) (reflect.Method, bool) {
	return clientType.MethodByName(operationID + "WithResponse")
}

// parseJSONFieldCode reports whether name is one of oapi-codegen's
// "JSON<status>" response fields (e.g. "JSON200", "JSON204"), returning the
// parsed status code. "JSONDefault" (a catch-all some operations declare)
// and non-JSON fields ("Body", "HTTPResponse") both correctly report false.
func parseJSONFieldCode(name string) (int, bool) {
	digits, ok := strings.CutPrefix(name, "JSON")
	if !ok || digits == "" {
		return 0, false
	}
	code, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return code, true
}

// responseInfoFor inspects <operationID>WithResponse's return type via
// reflection to find its single success (2xx) body field, refusing the one
// operation across the vendored client (AddonInstall, JSON200 and JSON201
// both *AddonsAddonLifecycleResponse) that declares two rather than
// guessing which a caller wanted.
func responseInfoFor(operationID string) (responseInfo, error) {
	m, ok := clientMethodFor(operationID)
	if !ok {
		return responseInfo{}, fmt.Errorf("no generated client method %sWithResponse", operationID)
	}
	// m was obtained from a reflect.Type, not a reflect.Value, so In(0) is
	// the receiver — that only shifts In, not Out, so Out(0) is genuinely
	// the method's first return value, *<operationID>Response.
	respPtr := m.Type.Out(0)
	if respPtr.Kind() != reflect.Pointer || respPtr.Elem().Kind() != reflect.Struct {
		return responseInfo{}, fmt.Errorf("%sWithResponse's first return value is %s, not a pointer to a response struct", operationID, respPtr)
	}
	respType := respPtr.Elem()

	var success []string
	for i := 0; i < respType.NumField(); i++ {
		name := respType.Field(i).Name
		if code, ok := parseJSONFieldCode(name); ok && code >= 200 && code < 300 {
			success = append(success, name)
		}
	}
	if len(success) > 1 {
		return responseInfo{}, fmt.Errorf(
			"%s's response declares %d success bodies (%s); refusing to guess which one a caller wants",
			operationID, len(success), strings.Join(success, ", "))
	}
	if len(success) == 1 {
		return responseInfo{SuccessField: success[0]}, nil
	}
	return responseInfo{}, nil
}

// goTypeMatchesReflectType reports whether goType (one of typeOf's four
// possible required-scalar renderings) is compatible with rt, the generated
// client method's own declared type for the same positional argument.
func goTypeMatchesReflectType(goType string, rt reflect.Type) bool {
	switch goType {
	case "int":
		return rt.Kind() == reflect.Int
	case "string":
		return rt.Kind() == reflect.String
	case "float64":
		return rt.Kind() == reflect.Float64
	case "bool":
		return rt.Kind() == reflect.Bool
	default:
		return false
	}
}

// pathArgTypesFor returns the generated client method's own path-argument
// types (skipping the receiver and ctx), used only to cross-check what
// fields.go independently derived from the OpenAPI path parameters — never
// as a second source of field identity or order, since reflection carries
// no parameter names, only types and position.
func pathArgTypesFor(operationID string, pathArgCount int) ([]reflect.Type, error) {
	m, ok := clientMethodFor(operationID)
	if !ok {
		return nil, fmt.Errorf("no generated client method %sWithResponse", operationID)
	}
	types := make([]reflect.Type, 0, pathArgCount)
	for i := 0; i < pathArgCount; i++ {
		idx := 2 + i // 0: receiver, 1: ctx
		if idx >= m.Type.NumIn() {
			return nil, fmt.Errorf("%sWithResponse takes fewer arguments than the %d path parameter(s) the specification declares for it", operationID, pathArgCount)
		}
		types = append(types, m.Type.In(idx))
	}
	return types, nil
}

// handlerFuncName derives a generated handler's mechanical function name
// from an operation's exported OperationID: "TagCreate" -> "tagCreate",
// matching the pilot domains' hand-written handler names for every
// operation whose handler they name after its OperationID directly (all of
// tags' and registries', and five of system's six — system.go's
// systemNodes is the pilot's own deliberate, shorter deviation for
// SystemNodesCount, which is exactly why scanHandOverrides below treats an
// operation as overridden when its *OperationID* is already declared by
// hand, not only when a function of this exact name is).
func handlerFuncName(operationID string) string {
	r := []rune(operationID)
	if len(r) == 0 {
		return operationID
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// buildHandlerSpec assembles one operation's handlerSpec from the fields and
// pathOrder assembleOperationFields already computed for its Input struct —
// the single source both the rendered struct and this handler are built
// from, per this file's package doc.
func buildHandlerSpec(domain string, op operation, fields []fieldSpec, pathOrder []string, inputStruct string) (handlerSpec, error) {
	byJSON := make(map[string]fieldSpec, len(fields))
	for _, f := range fields {
		byJSON[f.JSONName] = f
	}

	pathArgs := make([]pathArg, 0, len(pathOrder))
	for _, name := range pathOrder {
		f, ok := byJSON[name]
		if !ok {
			return handlerSpec{}, fmt.Errorf("%s: path parameter %q has no matching field in its own Input struct (internal error in assembleOperationFields)", op.OperationID, name)
		}
		pathArgs = append(pathArgs, pathArg{GoName: f.GoName, GoType: f.GoType})
	}

	var hasQuery, hasBody bool
	for _, f := range fields {
		switch f.Origin {
		case originPath:
			// accounted for via pathOrder/pathArgs above
		case originQuery:
			hasQuery = true
		case originBody:
			hasBody = true
		default:
			return handlerSpec{}, fmt.Errorf("%s: field %q carries no Origin; every field assembleOperationFields returns must be classified path, query or body", op.OperationID, f.JSONName)
		}
	}

	argTypes, err := pathArgTypesFor(op.OperationID, len(pathArgs))
	if err != nil {
		return handlerSpec{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}
	for i, a := range pathArgs {
		if !goTypeMatchesReflectType(a.GoType, argTypes[i]) {
			return handlerSpec{}, fmt.Errorf(
				"%s: path argument %d (%s) is Go type %s in the generated Input, but the generated client's positional argument is %s; refusing rather than binding a mismatched type",
				op.OperationID, i+1, a.GoName, a.GoType, argTypes[i])
		}
	}

	resp, err := responseInfoFor(op.OperationID)
	if err != nil {
		return handlerSpec{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}

	return handlerSpec{
		Domain:      domain,
		OperationID: op.OperationID,
		FuncName:    handlerFuncName(op.OperationID),
		InputStruct: inputStruct,
		PathArgs:    pathArgs,
		HasQuery:    hasQuery,
		HasBody:     hasBody,
		Response:    resp,
	}, nil
}

// renderHandlerFunc writes one handler function to buf.
func renderHandlerFunc(buf *strings.Builder, spec handlerSpec) {
	needsInput := len(spec.PathArgs) > 0 || spec.HasQuery || spec.HasBody
	inputParam := "input json.RawMessage"
	if !needsInput {
		inputParam = "_ json.RawMessage"
	}

	fmt.Fprintf(buf, "// %s is the generated handler for operation %s.\n", spec.FuncName, spec.OperationID)
	fmt.Fprintf(buf, "func %s(ctx context.Context, c *portainer.Client, %s) (any, error) {\n", spec.FuncName, inputParam)

	if len(spec.PathArgs) > 0 {
		fmt.Fprintf(buf, "\tvar params %s\n", spec.InputStruct)
		buf.WriteString("\tif err := json.Unmarshal(input, &params); err != nil {\n")
		fmt.Fprintf(buf, "\t\treturn nil, fmt.Errorf(\"%s: parse input: %%w\", err)\n", spec.OperationID)
		buf.WriteString("\t}\n")
	}
	if spec.HasQuery {
		fmt.Fprintf(buf, "\tvar queryParams apigen.%sParams\n", spec.OperationID)
		buf.WriteString("\tif err := json.Unmarshal(input, &queryParams); err != nil {\n")
		fmt.Fprintf(buf, "\t\treturn nil, fmt.Errorf(\"%s: parse query parameters: %%w\", err)\n", spec.OperationID)
		buf.WriteString("\t}\n")
	}
	if spec.HasBody {
		fmt.Fprintf(buf, "\tvar body apigen.%sJSONRequestBody\n", spec.OperationID)
		buf.WriteString("\tif err := json.Unmarshal(input, &body); err != nil {\n")
		fmt.Fprintf(buf, "\t\treturn nil, fmt.Errorf(\"%s: parse request body: %%w\", err)\n", spec.OperationID)
		buf.WriteString("\t}\n")
	}

	args := []string{"ctx"}
	for _, a := range spec.PathArgs {
		args = append(args, "params."+a.GoName)
	}
	if spec.HasQuery {
		args = append(args, "&queryParams")
	}
	if spec.HasBody {
		args = append(args, "body")
	}
	fmt.Fprintf(buf, "\tresp, err := c.API.%sWithResponse(%s)\n", spec.OperationID, strings.Join(args, ", "))
	buf.WriteString("\tif err != nil {\n")
	fmt.Fprintf(buf, "\t\treturn nil, fmt.Errorf(\"%s: %%w\", err)\n", spec.OperationID)
	buf.WriteString("\t}\n")
	buf.WriteString("\tif err := toolutil.Check(resp); err != nil {\n")
	fmt.Fprintf(buf, "\t\treturn nil, fmt.Errorf(\"%s: %%w\", err)\n", spec.OperationID)
	buf.WriteString("\t}\n")
	if spec.Response.SuccessField != "" {
		fmt.Fprintf(buf, "\treturn resp.%s, nil\n", spec.Response.SuccessField)
	} else {
		buf.WriteString("\treturn map[string]any{\"status\": \"ok\"}, nil\n")
	}
	buf.WriteString("}\n")
}

// renderActionsFile assembles one domain's actions.gen.go: the same
// generated-code header and formatting-through-go/format discipline as
// renderFile (inputs.gen.go), plus the apigen import only when at least one
// handler in specs actually references it — a domain whose generated
// handlers are all path-only or bodyless would otherwise get an unused
// import and fail to compile, which go/format's own parse step would not
// catch (an unused import is a go/types-level error, not a syntax one).
func renderActionsFile(packageName, specPath string, specs []handlerSpec) ([]byte, error) {
	needsAPIGen := false
	for _, s := range specs {
		if s.HasQuery || s.HasBody {
			needsAPIGen = true
			break
		}
	}

	var buf strings.Builder
	buf.WriteString("// Code generated by cmd/gen_action_inputs from " + specPath + ". DO NOT EDIT.\n\n")
	buf.WriteString("package " + packageName + "\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"encoding/json\"\n")
	buf.WriteString("\t\"fmt\"\n\n")
	buf.WriteString("\t\"github.com/jmrplens/portainer-mcp/internal/portainer\"\n")
	if needsAPIGen {
		buf.WriteString("\tapigen \"github.com/jmrplens/portainer-mcp/internal/portainer/gen\"\n")
	}
	buf.WriteString("\t\"github.com/jmrplens/portainer-mcp/internal/toolutil\"\n")
	buf.WriteString(")\n\n")

	for i, s := range specs {
		if i > 0 {
			buf.WriteString("\n")
		}
		renderHandlerFunc(&buf, s)
	}

	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated actions for package %s: %w", packageName, err)
	}
	return formatted, nil
}

// handOverrides is what a domain's own non-generated files already declare
// by hand: every operationId some ActionSpec composite literal names, and
// every top-level function name declared in the package. Both are checked
// (see domainOverridden below) because the two can diverge: system.go
// declares SystemNodesCount's ActionSpec by hand under the handler name
// systemNodes, not the name this generator would mechanically mint
// (systemNodesCount) — matching only on function name would miss it and
// mint a second, dead handlerNode nothing calls; matching only on
// OperationID would miss the rarer case of some unrelated function already
// occupying the mechanical name, which would otherwise fail generation with
// a duplicate-declaration compile error no test running go/format alone
// would catch.
type handOverrides struct {
	operationIDs map[string]bool
	funcNames    map[string]bool
}

// scanHandOverrides parses every hand-written (non ".gen.go") *.go file
// directly inside domainDir and returns what it already declares. Test files
// are included deliberately: a hand-written _test.go can also declare a
// helper function whose name would collide with a mechanical one, and this
// generator has no way to know that is "only a test" is safe to ignore.
func scanHandOverrides(domainDir string) (handOverrides, error) {
	out := handOverrides{operationIDs: map[string]bool{}, funcNames: map[string]bool{}}
	entries, err := os.ReadDir(domainDir)
	if err != nil {
		return out, fmt.Errorf("read %s: %w", domainDir, err)
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), ".gen.go") {
			continue
		}
		path := filepath.Join(domainDir, e.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return out, fmt.Errorf("parse %s: %w", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					out.funcNames[decl.Name.Name] = true
				}
			case *ast.KeyValueExpr:
				key, ok := decl.Key.(*ast.Ident)
				if !ok || key.Name != "OperationID" {
					return true
				}
				lit, ok := decl.Value.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if value, err := strconv.Unquote(lit.Value); err == nil {
					out.operationIDs[value] = true
				}
			}
			return true
		})
	}
	return out, nil
}

// overrideReason reports whether op is already covered by a hand-written
// handler in domainDir's non-generated files, and why — so the caller can
// name it in the generation report rather than silently generating nothing
// for it (or, worse, a second, colliding declaration).
func (h handOverrides) overrideReason(op operation) (string, bool) {
	if h.operationIDs[op.OperationID] {
		return "an ActionSpec for this operationId is already declared by hand", true
	}
	if h.funcNames[handlerFuncName(op.OperationID)] {
		return fmt.Sprintf("a function named %s already exists in this package", handlerFuncName(op.OperationID)), true
	}
	return "", false
}
