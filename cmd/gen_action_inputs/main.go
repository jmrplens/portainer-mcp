// Command gen_action_inputs generates one Input struct per operation from a
// vendored OpenAPI specification, one file per domain package under
// internal/tools.
//
// Every tool surface publishes an action's JSON Schema by reflecting it from
// toolutil.ActionSpec.Input (see internal/toolutil/schema.go), so an Input
// struct is what turns "441 operations" into 441 real, model-facing
// parameter shapes rather than one vacuous empty-object schema repeated 441
// times. Measured against the vendored Business Edition specification: 608
// parameters across path, query and request-body sources, 100% carrying a
// description, 419 marked required, 13 enums across 7 operations.
// Transcribing that by hand into a `jsonschema` struct tag on every one of
// 441 actions is both enormous and, worse, a second declaration of the same
// contract the spec already states once — precisely the drift that took the
// previous incarnation of this server to 37% coverage. See
// docs/superpowers's design specification and cmd/audit_1to1 for the measured
// baseline this command exists to close.
//
// What this command refuses to do, rather than guess: express a parameter
// whose schema uses oneOf/anyOf/not (no single Go type represents a union),
// express a free-form object with no declared properties and no typed
// additionalProperties, resolve a $ref outside #/components/schemas,
// generate for an operation whose request body declares more than one
// content type, or merge two of {path parameter, query parameter, body
// property} that contribute the same wire name. Every one of these aborts
// the whole run with the offending operation named — at 441 operations
// nobody reads the generated output, so anything this command emits must be
// trustworthy without review, and a refusal that is loud beats a struct that
// is silently wrong.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gen_action_inputs: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gen_action_inputs", flag.ContinueOnError)
	specPath := fs.String("spec", "api/specs/ee-2.44.0.json", "vendored OpenAPI spec to generate Input structs from")
	ceSpecPath := fs.String("ce-spec", "",
		"vendored Community Edition spec, used only to decide which generated actions are CE vs EE-only; "+
			"defaults to -spec's own filename with its \"ee-\" prefix swapped for \"ce-\"")
	toolsDir := fs.String("tools-dir", "internal/tools", "directory holding one subdirectory per domain package")
	skipDirs := fs.String("skip", "actioncatalog,dynamic,individual,meta",
		"comma-separated subdirectories of tools-dir that are tool surfaces, not domain packages")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	doc, paths, err := loadDocument(*specPath)
	if err != nil {
		return err
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		return err
	}

	// ceOperationIDs is the only extra input buildActionSpecFields needs
	// beyond what Task 1 and Task 2 already carry (see editionOf's own doc
	// comment): every operationId the vendored Community Edition
	// specification declares. An operationId absent from *specPath
	// altogether (system.upgrade is the pilot's own example) never reaches
	// buildActionSpecFields at all — this generator's domain processing
	// enumerates operations from *specPath, never from the CE spec — so it
	// stays exactly what it is today, a hand-written, hand-appended
	// ActionSpec in its own domain's Specs().
	if *ceSpecPath == "" {
		dir, base := filepath.Split(*specPath)
		*ceSpecPath = filepath.Join(dir, strings.Replace(base, "ee-", "ce-", 1))
	}
	_, cePaths, err := loadDocument(*ceSpecPath)
	if err != nil {
		return fmt.Errorf("load CE spec %s (used only to classify Edition): %w", *ceSpecPath, err)
	}
	ceByTag, err := operationsByDomain(cePaths)
	if err != nil {
		return fmt.Errorf("group CE spec %s by domain: %w", *ceSpecPath, err)
	}
	ceOperationIDs := ceOperationIDSet(ceByTag)

	// Informational, like the override report below: a reviewer reads these
	// once per wave and decides which entries need a hand-written override
	// in their own domain file (system.go's SystemUpgrade already carries
	// exactly the kind of override dangerMismatchWarnings exists to surface;
	// see actionspec.go's own doc comment on dangerKeywords).
	var allOps []operation
	for _, ops := range byTag {
		allOps = append(allOps, ops...)
	}
	if warnings := dangerMismatchWarnings(allOps); len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "%d operation(s) whose verb-derived danger flags may be misleading:\n", len(warnings))
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
	}
	if empty, restates := descriptionQualityWarnings(allOps); len(empty) > 0 || len(restates) > 0 {
		fmt.Fprintf(os.Stderr, "%d operation(s) with no description beyond boilerplate, %d whose description merely restates its summary — candidates for a narrative Description override:\n",
			len(empty), len(restates))
	}

	// toolutil.DomainTags is the single table both this generator and a
	// domain author read to know which OpenAPI tag(s) a domain directory
	// covers. Checked in both directions before anything is written: a
	// domain directory this table has no entry for used to fall through to
	// a silent no-op (see below); a tag with real operations that no domain
	// here claims is the same defect from the other side, and a table entry
	// naming a tag the vendored spec does not actually have is the same
	// defect again, reintroduced by a typo in the table itself.
	if err := toolutil.ValidateDomainTags(toolutil.DomainTags); err != nil {
		return fmt.Errorf("domain tag table: %w", err)
	}
	if err := checkDomainTagsCoverSpec(toolutil.DomainTags, byTag); err != nil {
		return fmt.Errorf("domain tag table vs %s: %w", *specPath, err)
	}

	skip := map[string]bool{}
	for _, d := range strings.Split(*skipDirs, ",") {
		if d = strings.TrimSpace(d); d != "" {
			skip[d] = true
		}
	}

	entries, err := os.ReadDir(*toolsDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", *toolsDir, err)
	}
	var domains []string
	for _, e := range entries {
		if e.IsDir() && !skip[e.Name()] {
			domains = append(domains, e.Name())
		}
	}
	sort.Strings(domains)

	res := &resolver{doc: doc}
	written := 0
	for _, domainName := range domains {
		ops, err := domainOperations(domainName, toolutil.DomainTags, byTag)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			continue
		}

		domainDir := filepath.Join(*toolsDir, domainName)
		overrides, err := scanHandOverrides(domainDir)
		if err != nil {
			return fmt.Errorf("domain %s: %w", domainName, err)
		}

		var allStructs []structSpec
		var handlerSpecs []handlerSpec
		var specEntries []specEntry
		var overriddenOps []string
		for _, op := range ops {
			structName := inputStructName(op.OperationID)
			var nested []structSpec
			// fields and pathOrder are computed exactly once here, from this
			// one call, and feed both the Input struct below and the
			// handler built from them further down — see fieldSpec.Origin's
			// and assembleOperationFields' own doc comments for why a
			// second, independent derivation of either is refused rather
			// than risked.
			fields, pathOrder, err := assembleOperationFields(op, res, doc, structName, &nested)
			if err != nil {
				return fmt.Errorf("%s %s (operationId %s): %w", op.Method, op.Path, op.OperationID, err)
			}
			// A parameterless action needs no Input struct at all: ActionSpec.Input
			// stays nil and InputSchema publishes the empty-object schema — the
			// pilot domains' system.* actions and registries.list/tags.list are
			// all of this shape today.
			inputStruct := ""
			if len(fields) > 0 {
				inputStruct = structName
				top := structSpec{
					Name: structName,
					Doc: fmt.Sprintf("%s is the parameter shape for operation %s (%s %s).",
						structName, op.OperationID, op.Method, op.Path),
					Fields: fields,
				}
				allStructs = append(allStructs, top)
				allStructs = append(allStructs, nested...)
			}

			// The escape hatch: a domain's own non-generated files may
			// already declare a handler for this operationId (or already
			// occupy the mechanical function name this generator would
			// otherwise mint) — recorded here and reported below, never
			// silently skipped, so a reviewer can see which actions this
			// generator does not cover.
			if reason, overridden := overrides.overrideReason(op); overridden {
				overriddenOps = append(overriddenOps, fmt.Sprintf("%s (%s)", op.OperationID, reason))
				continue
			}

			spec, err := buildHandlerSpec(domainName, op, fields, pathOrder, nested, inputStruct)
			if err != nil {
				return fmt.Errorf("%s %s (operationId %s): %w", op.Method, op.Path, op.OperationID, err)
			}
			handlerSpecs = append(handlerSpecs, spec)

			// Built from the exact same op the handler above was just built
			// from, never re-fetched or re-derived — see specEntry's own doc
			// comment on why FuncName and InputStruct are carried through
			// rather than recomputed here.
			actionFields, err := buildActionSpecFields(domainName, op, ceOperationIDs)
			if err != nil {
				return fmt.Errorf("%s %s (operationId %s): %w", op.Method, op.Path, op.OperationID, err)
			}
			specEntries = append(specEntries, specEntry{Fields: actionFields, HandlerFunc: spec.FuncName, InputStruct: inputStruct})
		}

		if len(overriddenOps) > 0 {
			sort.Strings(overriddenOps)
			fmt.Fprintf(os.Stderr, "domain %s: %d operation(s) already covered by hand-written code, no handler generated:\n", domainName, len(overriddenOps))
			for _, o := range overriddenOps {
				fmt.Fprintf(os.Stderr, "  - %s\n", o)
			}
		}

		if len(allStructs) > 0 {
			if name, dup := duplicateStructName(allStructs); dup {
				return fmt.Errorf("domain %s: struct %q would be declared more than once; rename the colliding operation or nested property before generating (go/format only checks syntax and would not catch this)", domainName, name)
			}

			source, err := renderFile(domainName, *specPath, allStructs)
			if err != nil {
				return fmt.Errorf("domain %s: %w", domainName, err)
			}

			outPath := filepath.Join(domainDir, "inputs.gen.go")
			if err := os.WriteFile(outPath, source, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", outPath, err)
			}
			written++
			fmt.Fprintf(os.Stderr, "wrote %s (%d struct(s) across %d operation(s))\n", outPath, len(allStructs), len(ops))
		}

		actionsPath := filepath.Join(domainDir, "actions.gen.go")
		if len(handlerSpecs) > 0 {
			// hasNarrativeHook: whether this domain's own hand-written file
			// already declares a function named narrative — the same
			// funcNames set overrideReason above checks, since a narrative
			// hook is exactly the kind of hand-declared symbol this
			// generator must detect and never collide with. See emit.go's
			// renderGeneratedSpecs for what the generated call site looks
			// like either way.
			hasNarrativeHook := overrides.funcNames["narrative"]
			source, err := renderActionsFile(domainName, *specPath, handlerSpecs, specEntries, hasNarrativeHook)
			if err != nil {
				return fmt.Errorf("domain %s: %w", domainName, err)
			}
			if err := os.WriteFile(actionsPath, source, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", actionsPath, err)
			}
			written++
			fmt.Fprintf(os.Stderr, "wrote %s (%d handler(s) across %d operation(s))\n", actionsPath, len(handlerSpecs), len(ops))
		} else if _, statErr := os.Stat(actionsPath); statErr == nil {
			// Every operation this domain covers is now hand-written or
			// overridden: a previously generated actions.gen.go would be
			// stale rather than merely empty, and CI's freshness check
			// (git diff --exit-code internal/tools/) is exactly what would
			// catch a lingering file this run no longer has any content
			// for.
			if err := os.Remove(actionsPath); err != nil {
				return fmt.Errorf("remove stale %s: %w", actionsPath, err)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "%d file(s) written\n", written)
	return nil
}
