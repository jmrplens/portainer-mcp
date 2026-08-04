package main

import (
	"fmt"
	"go/format"
	"strconv"
	"strings"
)

// scaffoldHeader is the doc comment every file this command writes opens
// with: internal/portainer/gen and internal/apiversion/applicability_gen.go
// (regenerated forever, never hand-edited) still carry the standard "Code
// generated ... DO NOT EDIT." header .golangci.yml's "generated: strict"
// exclusion keys off, but a domain's own actions/inputs are scaffolded once
// and owned from that moment (see P3.2's freeze and
// docs/domain-wave-checklist.md) — deliberately not that header, and
// deliberately not matched by golangci-lint's generated-file heuristic
// either, so an owned file is linted like any other source file the moment
// it exists. A domain author who hand-edits it afterwards gets no special
// treatment and no special exemption; that is the point.
func scaffoldHeader(specPath string) string {
	return "// Scaffolded by cmd/gen_action_inputs from " + specPath + ", then frozen: this\n" +
		"// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).\n" +
		"// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that\n" +
		"// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.\n\n"
}

// renderFile assembles one domain's scaffolded input structs: scaffoldHeader,
// the package clause, and every struct in structs in order — then formats the
// whole thing through go/format so the output is gofmt-clean by construction
// rather than by a separate `gofmt -w` step that could be skipped.
func renderFile(packageName, specPath string, structs []structSpec) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString(scaffoldHeader(specPath))
	buf.WriteString("package " + packageName + "\n\n")
	for i, s := range structs {
		if i > 0 {
			buf.WriteString("\n")
		}
		renderStruct(&buf, s)
	}
	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated source for package %s: %w", packageName, err)
	}
	return formatted, nil
}

// duplicateStructName returns the first struct name that appears more than
// once in structs, and whether one was found.
//
// Nested struct names are bare concatenation of their parent's name and the
// property's Go field name (see typeOf), with nothing that detects two
// different properties concatenating to the same name — a body with a
// property "foo" (an object with its own object property "bar") and a
// sibling property "fooBar" (also an object) both produce a struct named
// "...FooBar". go/format only parses and re-indents; it has no notion of
// package-level redeclaration, so it formats such a file successfully and
// this generator would exit 0 having written source that does not compile.
// No such collision exists in the vendored spec today, which is exactly why
// this must be checked rather than left to be noticed: it is latent until
// whichever future spec bump introduces one.
func duplicateStructName(structs []structSpec) (string, bool) {
	seen := make(map[string]bool, len(structs))
	for _, s := range structs {
		if seen[s.Name] {
			return s.Name, true
		}
		seen[s.Name] = true
	}
	return "", false
}

func renderStruct(buf *strings.Builder, s structSpec) {
	if s.Doc != "" {
		writeComment(buf, s.Doc)
	}
	fmt.Fprintf(buf, "type %s struct {\n", s.Name)
	for _, f := range s.Fields {
		if f.Description != "" {
			writeIndentedComment(buf, f.GoName+" "+f.Description)
		}
		fmt.Fprintf(buf, "%s %s %s\n", f.GoName, f.GoType, fieldTag(f.JSONName, f.Required, f.Description))
	}
	buf.WriteString("}\n")

	renderEnumParams(buf, s)
	renderMinimumParams(buf, s)
}

// renderEnumParams emits an EnumParams() method for s if any of its fields
// carry an enum constraint, implementing toolutil.EnumParams by structural
// typing — no import of internal/toolutil is needed for a generated domain
// package to satisfy an interface it already depends on transitively through
// toolutil.ActionSpec.
func renderEnumParams(buf *strings.Builder, s structSpec) {
	var withEnum []fieldSpec
	for _, f := range s.Fields {
		if len(f.Enum) > 0 {
			withEnum = append(withEnum, f)
		}
	}
	if len(withEnum) == 0 {
		return
	}
	fmt.Fprintf(buf, "\nfunc (%s) EnumParams() map[string][]any {\n", s.Name)
	buf.WriteString("\treturn map[string][]any{\n")
	for _, f := range withEnum {
		fmt.Fprintf(buf, "%q: {", f.JSONName)
		for i, v := range f.Enum {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(renderEnumLiteral(v, f.EnumScalarType))
		}
		buf.WriteString("},\n")
	}
	buf.WriteString("\t}\n}\n")
}

// renderMinimumParams emits a MinimumParams() method for s if any of its
// fields carry a Minimum constraint (see fieldSpec.Minimum and
// isIdentifierPathParam), implementing toolutil.MinimumParams by structural
// typing — the same mechanism renderEnumParams already uses for EnumParams,
// for the same reason: no import of internal/toolutil is needed for a
// generated domain package to satisfy an interface it already depends on
// transitively through toolutil.ActionSpec.
func renderMinimumParams(buf *strings.Builder, s structSpec) {
	var withMin []fieldSpec
	for _, f := range s.Fields {
		if f.Minimum != nil {
			withMin = append(withMin, f)
		}
	}
	if len(withMin) == 0 {
		return
	}
	fmt.Fprintf(buf, "\nfunc (%s) MinimumParams() map[string]int {\n", s.Name)
	buf.WriteString("\treturn map[string]int{\n")
	for _, f := range withMin {
		fmt.Fprintf(buf, "%q: %d,\n", f.JSONName, *f.Minimum)
	}
	buf.WriteString("\t}\n}\n")
}

// renderEnumLiteral renders one enum value as a Go literal. Values arrive as
// decoded from the vendored spec's JSON via encoding/json into `any`, so a
// JSON number is always float64 regardless of the schema's declared
// "integer"/"number" type — scalarType is what tells this function which Go
// literal form the field's own type expects.
func renderEnumLiteral(v any, scalarType string) string {
	switch scalarType {
	case "integer":
		if f, ok := v.(float64); ok {
			return strconv.FormatInt(int64(f), 10)
		}
	case "number":
		if f, ok := v.(float64); ok {
			return strconv.FormatFloat(f, 'g', -1, 64)
		}
	case "boolean":
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b)
		}
	case "string":
		if s, ok := v.(string); ok {
			return fmt.Sprintf("%q", s)
		}
	}
	return fmt.Sprintf("%q", fmt.Sprint(v))
}

// writeComment renders text as a package-level doc comment.
func writeComment(buf *strings.Builder, text string) {
	for _, line := range strings.Split(text, "\n") {
		buf.WriteString("// " + line + "\n")
	}
}

// writeIndentedComment renders text as a field-level doc comment, indented to
// sit directly above the struct field it documents.
func writeIndentedComment(buf *strings.Builder, text string) {
	for _, line := range strings.Split(text, "\n") {
		buf.WriteString("\t// " + line + "\n")
	}
}

// redactionGuard is one operation whose success response can carry a
// credential and whose domain has supplied a redaction wrapper for it.
type redactionGuard struct {
	OperationID string
	FuncName    string // the domain's redact<OperationID>
}

// renderRedactionGuardFile emits one domain's generated redaction guard: a
// test per wrapper that populates a whole response value, runs the wrapper
// over it, and fails if any credential-shaped field survived.
//
// This closes the gap checkCredentialRedaction cannot: that check only proves
// a function of the right name exists. A pass-through
// (`func redactX(v T) any { return v }`) satisfies it completely, and would
// have shipped a handler returning every credential it was supposed to
// remove. Only running the wrapper against a populated value and inspecting
// what comes back distinguishes a real wrapper from a stub, and generating
// that test means no domain author has to remember to write it.
//
// The file is named "..._gen_test.go" rather than the "....gen.go" this
// generator uses elsewhere for one hard reason: Go decides what is a test
// file purely by the "_test.go" suffix, so a file called
// "redaction_test.gen.go" would be compiled into the domain package proper
// and its "testing" import would land in the shipped binary. The
// generated-code header is what marks it generated (and what .golangci.yml's
// "generated: strict" exclusion keys off), not the file name.
//
// Each guard reflects over the wrapper rather than naming the response type
// in the generated source. That keeps this renderer from having to turn an
// arbitrary reflect.Type back into valid Go source with the right package
// qualifier — and it means the generated test needs no import of the
// generated API package at all, whatever shape the wrapper happens to take.
func renderRedactionGuardFile(packageName, specPath string, guards []redactionGuard) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString(scaffoldHeader(specPath))
	buf.WriteString("package " + packageName + "\n\n")
	buf.WriteString("import (\n\t\"reflect\"\n\t\"testing\"\n\n\t\"github.com/jmrplens/portainer-mcp/internal/toolutil\"\n)\n\n")

	// Field names are deliberately unexported (funcName/operationID/fn, not
	// FuncName/OperationID/Fn): scanHandOverrides (handler.go) walks every
	// non-".gen.go" *.go file in a domain directory — test files included,
	// on purpose, to catch a hand-written helper colliding with a mechanical
	// name — looking for a composite literal keyed "OperationID" to detect
	// an ActionSpec a domain author already declared by hand. This file is
	// named "..._gen_test.go", not "...gen.go" (see this function's own doc
	// comment for why), so it is not excluded from that scan. An exported
	// "OperationID" field here would therefore be misread, on the next
	// regeneration, as a hand-declared ActionSpec for every operation this
	// table names — silently starving actions.go of the handlers this
	// generator is supposed to keep producing for them. Reproduced directly:
	// an earlier draft of this template did use "OperationID", and a second
	// consecutive `go run ./cmd/gen_action_inputs` dropped every one of
	// registries' four generated handlers whose OperationID this table
	// named, misreporting them as "already covered by hand-written code."
	buf.WriteString("// redactionGuards is one table entry per generated redaction wrapper in this\n")
	buf.WriteString("// domain, consulted by TestUnit_RedactionGuards_RemoveEveryCredentialShapedField.\n")
	buf.WriteString("var redactionGuards = []struct {\n\tfuncName    string\n\toperationID string\n\tfn          any\n}{\n")
	for _, g := range guards {
		fmt.Fprintf(&buf, "\t{funcName: %q, operationID: %q, fn: %s},\n", g.FuncName, g.OperationID, g.FuncName)
	}
	buf.WriteString("}\n")

	buf.WriteString(`
// TestUnit_RedactionGuards_RemoveEveryCredentialShapedField proves every
// generated redaction wrapper in this domain actually strips what it is
// required to strip. cmd/gen_action_inputs refuses to emit a handler for a
// credential-returning operation unless a function of the expected name
// exists; existing is not the same as working, and this is what tells the
// two apart.
func TestUnit_RedactionGuards_RemoveEveryCredentialShapedField(t *testing.T) {
	t.Parallel()
	for _, g := range redactionGuards {
		t.Run(g.operationID, func(t *testing.T) {
			t.Parallel()
			fn := reflect.ValueOf(g.fn)
			if fn.Type().NumIn() != 1 || fn.Type().NumOut() != 1 {
				t.Fatalf("%s must take exactly one argument and return exactly one value, got %d in and %d out",
					g.funcName, fn.Type().NumIn(), fn.Type().NumOut())
			}

			// Populated, not zero-valued: WalkForCredentialShapedFields only
			// reports fields that actually carry a value, so a guard run
			// against a zero response would pass whatever the wrapper did.
			arg := reflect.New(fn.Type().In(0)).Elem()
			toolutil.PopulateForCredentialAudit(arg)

			survived := toolutil.AssertRedacted(fn.Call([]reflect.Value{arg})[0].Interface(), g.operationID)
			if len(survived) > 0 {
				t.Errorf("%s left %d credential-shaped field(s) populated: %v", g.funcName, len(survived), survived)
			}
		})
	}
}
`)

	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated redaction guards for package %s: %w", packageName, err)
	}
	return formatted, nil
}
