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
// content type, merge two of {path parameter, query parameter, body
// property} that contribute the same wire name, emit a handler for an
// operation with no matching generated client method (or one shaped
// differently than assembleOperationFields expects), or emit a bare handler
// for an operation whose success response can carry a credential-shaped
// field with no domain-supplied redaction wrapper declared. At 441
// operations nobody reads the generated output, so anything this command
// emits must be trustworthy without review, and a refusal that is loud beats
// a struct that is silently wrong.
//
// None of these refusals aborts the whole run, and — since P3.3 task 3 —
// none of them poisons the whole domain either. Every one is scoped to *the
// one operation that triggered it*: that operation contributes nothing at
// all to its domain (no input struct, no nested type, no MinimumParams/
// EnumParams method, no redaction wrapper stub, no guard row, no handler, no
// ActionSpec entry), it is named in the report at the end of run(), and
// every other operation in the same domain — and every other domain — still
// generates normally. The command still exits non-zero, since an operation
// nobody can prove safe to generate for never gets a handler, but it fails
// having regenerated everything it legitimately could; the domain author's
// job is to read the report and hand-write exactly the named operations,
// landed in the same commit as the rest of the scaffold.
//
// This is narrower than an earlier revision of this comment described.
// Before P3.3 task 1, only an unresolvable success-response schema (see
// errResponseSchemaUnresolvable) was collected and continued past; every
// other refusal returned straight out of run()'s domain loop, so one defect
// in one operation stopped every domain sorting after it, alphabetically,
// from regenerating — the vendored spec's self-referential
// portaineree.EdgeConfig was the case that first exposed it, and a field
// collision in stacks.StackMigrate later stopped every one of the twelve
// domains sorting after "stacks" the same way, discovered before any of the
// twelve existed. Task 1 fixed that by collecting every refusal and
// continuing, but still discarded the *entire* domain containing one: a
// domain with even a single operation this generator cannot support (a
// multipart file upload is the real, permanent example — no version of this
// generator's own body handling will ever support one) could never be
// scaffolded at all, not even for its other, perfectly generatable
// operations. Task 3 narrowed the unit of refusal from "domain" to
// "operation" for exactly that reason.
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

// scaffoldedMarkers are the filenames a domain directory carries once this
// command has written to it at least once — either this task's current
// names (actions.go, inputs.go) or the pre-freeze *.gen.go/redaction_gen_test.go
// names a domain scaffolded before P3.2 may still carry until it is next
// touched. Checking both is what makes domainAlreadyScaffolded correct for a
// domain mid-migration, not only one already fully renamed.
var scaffoldedMarkers = []string{
	"actions.go", "inputs.go", "redaction_test.go",
	"actions.gen.go", "inputs.gen.go", "redaction_gen_test.go",
}

// domainAlreadyScaffolded reports whether domainDir already carries any file
// this command would itself write, and names the first one found.
//
// This is the whole premise of the freeze this command now scaffolds
// into: a domain's actions.go/inputs.go are written once and owned by that
// domain from that moment (see docs/domain-wave-checklist.md). Regenerating
// over an already-scaffolded domain would silently discard every hand edit
// made since — exactly the "own it, then hand-maintain it" workflow this
// command exists to seed, undone by its own next invocation. run() checks
// this before doing anything else for a domain, and only -allow-overwrite
// bypasses it — an explicit, rare "start this domain over from the spec"
// action, not something an ordinary `make scaffold-domain` run does by
// accident.
func domainAlreadyScaffolded(domainDir string) (string, bool) {
	for _, name := range scaffoldedMarkers {
		if _, err := os.Stat(filepath.Join(domainDir, name)); err == nil {
			return name, true
		}
	}
	return "", false
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
	allowOverwrite := fs.Bool("allow-overwrite", false,
		"regenerate a domain that already has scaffolded files (actions.go/inputs.go, or their pre-freeze "+
			"*.gen.go names), discarding any hand edit made since it was scaffolded; refused by default (see "+
			"domainAlreadyScaffolded's doc comment)")
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
	resolvedCESpecPath := *ceSpecPath
	if resolvedCESpecPath == "" {
		dir, base := filepath.Split(*specPath)
		resolvedCESpecPath = filepath.Join(dir, strings.Replace(base, "ee-", "ce-", 1))
		if resolvedCESpecPath == *specPath {
			// strings.Replace found no "ee-" substring in the base filename
			// to swap, so it returned the base unchanged: resolvedCESpecPath
			// would silently become *specPath itself, ceOperationIDs would
			// contain every EE operationId, and editionOf would classify
			// every action as CE — the exact failure editionConstName
			// already refuses at render time (see emit.go), just one step
			// earlier and unguarded. Refuse loudly instead of loading the EE
			// spec a second time and calling it CE.
			return fmt.Errorf(
				"cannot derive a Community Edition spec path from -spec %q: its filename carries no \"ee-\" to swap for \"ce-\"; pass -ce-spec explicitly rather than classifying every action as CE",
				*specPath)
		}
	}
	// ceDoc is kept (not discarded the way a pre-Task-2 revision of this line
	// did) because classifying Edition is no longer this spec's only use: a
	// CE-shared operation's own *fields* still need the Community
	// specification's resolved shape to compare against, so per-field
	// edition gating (see ceRes/ceOpsByID below, and fields.go's
	// ceEEFieldDiff) can tell which of a shared action's parameters are
	// Business-only.
	ceDoc, cePaths, err := loadDocument(resolvedCESpecPath)
	if err != nil {
		return fmt.Errorf("load CE spec %s: %w", resolvedCESpecPath, err)
	}
	ceByTag, err := operationsByDomain(cePaths)
	if err != nil {
		return fmt.Errorf("group CE spec %s by domain: %w", resolvedCESpecPath, err)
	}
	ceOperationIDs := ceOperationIDSet(ceByTag)
	// ceOpsByID and ceRes are what ceEEFieldDiff needs to resolve a
	// CE-shared operation's own Community-side fields: the operation itself
	// (ceOpsByID; ceOperationIDs alone only records presence) and a resolver
	// bound to the Community document (ceRes), never doc/res above, which
	// resolve against the Business Edition document this generator's own
	// Input structs are otherwise built from.
	ceOpsByID := ceOperationsByID(ceByTag)
	ceRes := &resolver{doc: ceDoc}

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
		for _, w := range empty {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
		for _, w := range restates {
			fmt.Fprintf(os.Stderr, "  - %s\n", w)
		}
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
	// refusals collects every per-operation refusal that must not abort the
	// whole run: a field collision, an unsupported parameter location, a
	// request body with more than one content type, an unresolvable
	// success-response schema (see errResponseSchemaUnresolvable), a
	// credential-shaped response field with no domain-supplied redaction
	// wrapper, a generated client method this generator's own derivation
	// disagrees with, or an operation this generator otherwise cannot derive
	// an ActionSpec for. Each entry names its domain, operation and reason
	// (formatted "domain %s: ..." so a lexical sort groups the final report by
	// domain); the operation that produced it contributes nothing at all
	// (see commitOperation below), every other operation — in the same
	// domain and every other one — still generates, and the run still exits
	// non-zero — see the report at the end of run().
	var refusals []string
	// domainsWithRefusals is the set of domain names that own at least one
	// entry in refusals above, tracked directly rather than re-parsed out of
	// the formatted strings, purely to state the domain count in the final
	// report. Named for what it now means since P3.3 task 3: these domains
	// are not left unwritten (a domain with only its permanently-unsupported
	// operations refused still generates everything else), only marked as
	// still having a hand-written gap to fill.
	domainsWithRefusals := map[string]bool{}
	for _, domainName := range domains {
		ops, err := domainOperations(domainName, toolutil.DomainTags, byTag)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			continue
		}

		domainDir := filepath.Join(*toolsDir, domainName)

		if existing, scaffolded := domainAlreadyScaffolded(domainDir); scaffolded && !*allowOverwrite {
			fmt.Fprintf(os.Stderr,
				"domain %s: already scaffolded (%s exists); refusing to overwrite files this domain now owns — "+
					"pass -allow-overwrite to force regeneration, discarding any hand edit made since\n",
				domainName, existing)
			continue
		}

		overrides, err := scanHandOverrides(domainDir)
		if err != nil {
			return fmt.Errorf("domain %s: %w", domainName, err)
		}

		var allStructs []structSpec
		var handlerSpecs []handlerSpec
		var specEntries []specEntry
		var overriddenOps []string
		var redactionGuards []redactionGuard
		// domainRefusalCount is this domain's own share of refusals, printed
		// immediately below so a reviewer does not have to count the final
		// report by domain themselves. Since P3.3 task 3 a refusal no longer
		// poisons the domain that contains it: only the refused operation
		// itself contributes nothing (see commitInput/commitGuard below),
		// every other operation in this domain still generates, and the
		// domain's files are still written from whatever succeeded.
		domainRefusalCount := 0
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
				// A field collision, an unsupported parameter location or a
				// request body with more than one content type: a refusal about
				// this one operation, not a reason to abort the other 42
				// domains — see refusals' own doc comment above. This operation
				// contributes nothing (there is no struct to discard: fields was
				// never even produced) and is named at the end of the run, which
				// still exits non-zero. This continues rather than breaks out of
				// this domain's own loop: every other operation in it is still
				// checked, so a domain with two independently-refusable
				// operations gets both reported in one run, not just the first
				// one found (see the standing warning about non-discriminating
				// tests this project has hit before: a test with only one
				// refusable operation cannot tell "collects it" apart from
				// "aborts on it").
				refusals = append(refusals, fmt.Sprintf("domain %s: %s %s (operationId %s): %v", domainName, op.Method, op.Path, op.OperationID, err))
				domainRefusalCount++
				continue
			}

			// Per-field edition gating: only relevant to an operation the
			// Community Edition specification also declares (editionOf, called
			// again by buildActionSpecFields below, would mark anything else
			// Edition: EE outright — the whole-action gate is already enough
			// for those). ceEEFieldDiff resolves ceOp's own fields against the
			// Community document and compares them, structurally and
			// nested-inclusively, against fields/nested above; applyFieldEditionGate
			// then stamps RequiresEdition on whichever ones the comparison found
			// Business-only, mutating fields/nested in place before commitInput
			// (below) ever reads them, so every consumer — the Input struct
			// this operation contributes, and every nested type it needed — sees
			// the gate.
			//
			// A failure here degrades rather than refuses: it means this
			// generator cannot resolve the *Community* side of an operation it
			// already resolved the Business Edition side of (the identical
			// classes of shape this generator refuses generally — a field
			// collision, an unsupported parameter location — just on the other
			// document), which says nothing about whether the Business Edition
			// Input struct itself is safe to emit. Reported so a human notices,
			// but this operation still generates, ungated, exactly as every
			// action did before this mechanism existed.
			if ceOperationIDs[op.OperationID] {
				if ceOp, ok := ceOpsByID[op.OperationID]; ok {
					eeOnly, diffErr := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
					if diffErr != nil {
						fmt.Fprintf(os.Stderr,
							"domain %s: %s %s (operationId %s): could not resolve the Community Edition shape for per-field edition gating, generating ungated: %v\n",
							domainName, op.Method, op.Path, op.OperationID, diffErr)
					} else {
						applyFieldEditionGate(structName, fields, nested, eeOnly)
					}
				}
			}

			// A parameterless action needs no Input struct at all: ActionSpec.Input
			// stays nil and InputSchema publishes the empty-object schema — the
			// pilot domains' system.* actions and registries.list/tags.list are
			// all of this shape today.
			inputStruct := ""
			if len(fields) > 0 {
				inputStruct = structName
			}
			// commitInput appends this operation's Input struct and every
			// nested type it needed to allStructs. Called only once every
			// refusal this operation could still hit (checkCredentialRedaction
			// below, and — for a non-overridden operation — buildHandlerSpec
			// and buildActionSpecFields further down) has already passed: a
			// refused operation must never reach this call, or its struct,
			// its nested types and their MinimumParams/EnumParams methods
			// would be declared and then called by nothing this generator
			// ever emits — exactly the orphan-code defect P3.3 task 3 exists
			// to close. See this file's own package doc.
			commitInput := func() {
				if inputStruct == "" {
					return
				}
				allStructs = append(allStructs, structSpec{
					Name: structName,
					Doc: fmt.Sprintf("%s is the parameter shape for operation %s (%s %s).",
						structName, op.OperationID, op.Method, op.Path),
					Fields: fields,
				})
				allStructs = append(allStructs, nested...)
			}

			// The escape hatch: a domain's own non-generated files may
			// already declare a handler for this operationId (or already
			// occupy the mechanical function name this generator would
			// otherwise mint) — recorded here and reported below, never
			// silently skipped, so a reviewer can see which actions this
			// generator does not cover.
			reason, overridden := overrides.overrideReason(op)

			// checkCredentialRedaction is task-4b's structural half of the
			// registries redaction defect P2's review caught: an operation
			// whose success response can carry a credential-shaped field
			// (per toolutil.IsCredentialShapedName, resolved through the
			// spec's own $refs) never gets a bare generated handler. Either
			// the domain has already declared the redaction wrapper this
			// generator's naming convention expects, or generation refuses
			// right here, naming the operation and the field — not a test
			// that might happen to exercise it.
			//
			// This runs *before* the override skip below, and deliberately so:
			// an operation covered by hand-written code is exactly the case
			// P2's defect actually occurred in, and skipping the guard for it
			// made the escape hatch from generation an escape hatch from the
			// guard too. See checkCredentialRedaction's own doc comment for
			// how a hand-written handler states that it redacts.
			redactWith, err := checkCredentialRedaction(op, res, overrides.funcNames, overridden)
			if err != nil {
				// Whether the success response could not be resolved at all
				// (errResponseSchemaUnresolvable) or it resolved to a
				// credential-shaped field with no domain-supplied redaction
				// wrapper declared: both are a refusal about this one
				// operation, not a reason to stop generating the other 42
				// domains — see refusals' own doc comment above. Neither
				// commitInput nor anything below ever runs for it, so it
				// contributes no struct either, even though fields was
				// already assembled above.
				refusals = append(refusals, fmt.Sprintf("domain %s: %s %s (operationId %s): %v", domainName, op.Method, op.Path, op.OperationID, err))
				domainRefusalCount++
				continue
			}

			if overridden {
				// A hand-written handler is expected to exist already (or to
				// land in the same commit); its Input struct and, if
				// redactWith is non-empty, its redaction-wrapper guard row
				// are both real requirements for that hand-written code, so
				// they are committed now — this operation cannot be refused
				// any further, unlike the non-overridden path below.
				commitInput()
				if redactWith != "" {
					// HandlerFuncName stays "": an overridden operation's
					// handler is hand-declared under whatever name its
					// author chose, which this generator never learns, so
					// the handler-level redaction guard
					// (renderRedactionGuardFile) skips it rather than
					// reference a function that may not exist under the
					// mechanical name at all.
					redactionGuards = append(redactionGuards, redactionGuard{OperationID: op.OperationID, FuncName: redactWith, HandlerFuncName: ""})
				}
				overriddenOps = append(overriddenOps, fmt.Sprintf("%s (%s)", op.OperationID, reason))
				continue
			}

			spec, err := buildHandlerSpec(domainName, op, fields, pathOrder, nested, inputStruct, redactWith)
			if err != nil {
				// No generated client method for this operationId (the real,
				// permanent case: a multipart file upload), one shaped
				// differently than assembleOperationFields expects, or a
				// wire-width mismatch checkWireWidth refuses to round-trip: a
				// refusal about this one operation, handled the same way as
				// every other one above. redactWith may be non-empty here —
				// a domain author may already have declared the redaction
				// wrapper this credential-shaped operation needs — but
				// commitInput/the guard append below never run for it, so
				// neither the Input struct nor a guard row referencing the
				// handler this refusal just prevented from existing is ever
				// emitted. The wrapper function itself, if already declared
				// by hand, is simply unused code the domain author wrote
				// ahead of the handler that will eventually call it; see
				// this file's own package doc.
				refusals = append(refusals, fmt.Sprintf("domain %s: %s %s (operationId %s): %v", domainName, op.Method, op.Path, op.OperationID, err))
				domainRefusalCount++
				continue
			}

			// Built from the exact same op the handler above was just built
			// from, never re-fetched or re-derived — see specEntry's own doc
			// comment on why FuncName and InputStruct are carried through
			// rather than recomputed here.
			actionFields, err := buildActionSpecFields(domainName, op, ceOperationIDs)
			if err != nil {
				refusals = append(refusals, fmt.Sprintf("domain %s: %s %s (operationId %s): %v", domainName, op.Method, op.Path, op.OperationID, err))
				domainRefusalCount++
				continue
			}

			// Every refusal this operation could still hit has now passed:
			// only from this point on is it safe to commit its struct, its
			// redaction guard row (recorded for generated and hand-written
			// handlers alike: the guard proves the wrapper works, and a
			// hand-written handler is no more trustworthy on that point than
			// a generated one — it is the case P2's defect actually occurred
			// in), its handler and its ActionSpec entry.
			commitInput()
			if redactWith != "" {
				redactionGuards = append(redactionGuards, redactionGuard{OperationID: op.OperationID, FuncName: redactWith, HandlerFuncName: handlerFuncName(op.OperationID)})
			}
			handlerSpecs = append(handlerSpecs, spec)
			specEntries = append(specEntries, specEntry{Fields: actionFields, HandlerFunc: spec.FuncName, InputStruct: inputStruct})
		}

		if domainRefusalCount > 0 {
			domainsWithRefusals[domainName] = true
			fmt.Fprintf(os.Stderr, "domain %s: %d operation(s) refused generation (named at the end of this run); every other operation in this domain still generated normally\n", domainName, domainRefusalCount)
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

			outPath := filepath.Join(domainDir, "inputs.go")
			if err := os.WriteFile(outPath, source, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", outPath, err)
			}
			written++
			fmt.Fprintf(os.Stderr, "wrote %s (%d struct(s) across %d operation(s))\n", outPath, len(allStructs), len(ops))
		}

		// The scaffolded redaction guard (see renderRedactionGuardFile).
		// Written whenever this domain has at least one redaction wrapper, and
		// removed when it no longer has any — a lingering guard for a wrapper
		// that has been deleted would not compile. This only happens on a
		// first-time scaffold, or an explicit -allow-overwrite regeneration;
		// once a domain owns this file, a hand edit that removes its last
		// wrapper is the domain author's own responsibility to clean up (see
		// docs/domain-wave-checklist.md's upgrade procedure).
		guardPath := filepath.Join(domainDir, "redaction_test.go")
		if len(redactionGuards) > 0 {
			sort.Slice(redactionGuards, func(i, j int) bool {
				return redactionGuards[i].OperationID < redactionGuards[j].OperationID
			})
			source, err := renderRedactionGuardFile(domainName, *specPath, redactionGuards)
			if err != nil {
				return fmt.Errorf("domain %s: %w", domainName, err)
			}
			if err := os.WriteFile(guardPath, source, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", guardPath, err)
			}
			written++
			fmt.Fprintf(os.Stderr, "wrote %s (%d redaction guard(s))\n", guardPath, len(redactionGuards))
		} else if _, statErr := os.Stat(guardPath); statErr == nil {
			if err := os.Remove(guardPath); err != nil {
				return fmt.Errorf("remove stale %s: %w", guardPath, err)
			}
		}

		actionsPath := filepath.Join(domainDir, "actions.go")
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
			// overridden: a previously scaffolded actions.go would be stale
			// rather than merely empty. This branch only runs on a first-time
			// scaffold or an explicit -allow-overwrite regeneration — the
			// domainAlreadyScaffolded refusal earlier is what stops it from
			// ever reaching (and silently deleting) an owned domain's file on
			// an ordinary run.
			if err := os.Remove(actionsPath); err != nil {
				return fmt.Errorf("remove stale %s: %w", actionsPath, err)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "%d file(s) written\n", written)

	// Reported once, at the end, after every domain has been written with
	// whatever it legitimately could generate: the run still fails, since an
	// operation nobody can prove safe to generate for never gets a handler,
	// but every named operation below is the domain author's own work item —
	// write it by hand, in the same commit as the rest of the domain's
	// scaffold — not a reason the rest of that domain, or any other domain,
	// failed to regenerate. Every entry already starts with "domain <name>: ",
	// so this lexical sort groups the report by domain rather than leaving it
	// in whatever order the refusals happened to be found in.
	if len(refusals) > 0 {
		sort.Strings(refusals)
		fmt.Fprintf(os.Stderr, "%d operation(s) refused generation across %d domain(s); write each one by hand (every other operation in these domains was still generated):\n", len(refusals), len(domainsWithRefusals))
		for _, r := range refusals {
			fmt.Fprintf(os.Stderr, "  - %s\n", r)
		}
		return fmt.Errorf("%d operation(s) refused generation across %d domain(s); see the list above", len(refusals), len(domainsWithRefusals))
	}
	return nil
}
