package main

import (
	"fmt"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// specEntry is one operation's rendered ActionSpec ingredients, ready to
// compose into a generatedSpecs() literal: buildActionSpecFields' own
// mechanical output, plus the two things Task 1 and Task 2 already produced
// from the same operation (buildHandlerSpec.FuncName and the Input struct
// name, "" when the operation takes none) — never re-derived here, per this
// generator's own standing rule against computing the same fact twice.
type specEntry struct {
	Fields      actionSpecFields
	HandlerFunc string
	InputStruct string
}

// renderGeneratedSpecs renders `func generatedSpecs() []toolutil.ActionSpec`
// into buf: one toolutil.ActionSpec literal per entry, in order.
//
// hasNarrativeHook is whether this domain's own hand-written, non-generated
// file already declares a function named narrative (detected by
// scanHandOverrides alongside every other hand-declared symbol, since a
// narrative hook is exactly that: a hand-declared symbol this generator must
// never emit itself). When it does, every literal is wrapped in
// toolutil.WithNarrative(literal, narrative(OperationID)) — a real function
// call this generated code makes at runtime, against a function whose
// implementation lives entirely in the domain's own hand file. Regenerating
// this file therefore can never discard whatever that hook returns: nothing
// about the hook's return value is baked into this file at all, only the
// call site is. When the domain has not declared one yet, the literal is
// emitted bare — a domain with nothing hand-authored is not made to declare
// a hook only to return its zero value forever.
func renderGeneratedSpecs(buf *strings.Builder, entries []specEntry, hasNarrativeHook bool) {
	buf.WriteString("// generatedSpecs returns every ActionSpec this generator derives for this\n")
	buf.WriteString("// domain. The domain's own Specs() (in its own, non-generated file) combines\n")
	buf.WriteString("// this with whatever action this generator refused or the domain otherwise\n")
	buf.WriteString("// declares by hand.\n")
	buf.WriteString("func generatedSpecs() []toolutil.ActionSpec {\n")
	buf.WriteString("\treturn []toolutil.ActionSpec{\n")
	for _, e := range entries {
		renderSpecLiteral(buf, e, hasNarrativeHook)
	}
	buf.WriteString("\t}\n")
	buf.WriteString("}\n")
}

// renderSpecLiteral renders one specEntry as a toolutil.ActionSpec composite
// literal (or such a literal wrapped in a call to toolutil.WithNarrative —
// see renderGeneratedSpecs). Every field renderSpecLiteral itself decides is
// exactly what actionSpecFields carries: Name, Domain, OperationID, Title,
// Description, Edition and the three danger flags, plus Handler and Input
// from e — nothing here re-derives any of them. A false danger flag is
// omitted entirely rather than rendered as `Mutating: false`: composite
// literals default every unset field to its zero value already, and
// toolutil.ActionSpec.Validate's own error messages read the same either
// way, so the omission is pure legibility, matching go/format's own
// preference (gofmt does not itself strip explicit zero-value fields, but
// nothing downstream distinguishes the two forms).
func renderSpecLiteral(buf *strings.Builder, e specEntry, hasNarrativeHook bool) {
	f := e.Fields
	if hasNarrativeHook {
		buf.WriteString("\t\ttoolutil.WithNarrative(toolutil.ActionSpec{\n")
	} else {
		buf.WriteString("\t\t{\n")
	}
	fmt.Fprintf(buf, "\t\t\tName: %q, Domain: %q, OperationID: %q,\n", f.Name, f.Domain, f.OperationID)
	fmt.Fprintf(buf, "\t\t\tTitle: %q,\n", f.Title)
	fmt.Fprintf(buf, "\t\t\tDescription: %q,\n", f.Description)
	fmt.Fprintf(buf, "\t\t\tEdition: edition.%s,\n", editionConstName(f.Edition))
	if f.Mutating {
		buf.WriteString("\t\t\tMutating: true,\n")
	}
	if f.Destructive {
		buf.WriteString("\t\t\tDestructive: true,\n")
	}
	if f.Idempotent {
		buf.WriteString("\t\t\tIdempotent: true,\n")
	}
	fmt.Fprintf(buf, "\t\t\tHandler: %s,\n", e.HandlerFunc)
	if e.InputStruct != "" {
		fmt.Fprintf(buf, "\t\t\tInput: %s{},\n", e.InputStruct)
	}
	if hasNarrativeHook {
		fmt.Fprintf(buf, "\t\t}, narrative(%q)),\n", f.OperationID)
	} else {
		buf.WriteString("\t\t},\n")
	}
}

// editionConstName renders e as the edition package's own constant
// identifier ("CE" or "EE") — actionSpecFields.Edition is always one of the
// two (editionOf's only possible outputs), so this refuses rather than
// guess if a future change ever produces a third value: silently rendering
// an unrecognised edition as, say, CE would widen an EE-only action's
// availability instead of failing generation loudly.
func editionConstName(e edition.Edition) string {
	switch e {
	case edition.CE:
		return "CE"
	case edition.EE:
		return "EE"
	default:
		panic(fmt.Sprintf("gen_action_inputs: editionConstName: unrecognised edition %q", e))
	}
}
