package main

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// editionFieldCounts is one edition's contribution to this audit's
// per-edition breakdown: how many of the declared actions a real server of
// that edition is even offered at all (actioncatalog.Build's whole-action
// edition/version filter), and how many top-level input fields those actions
// publish once Catalog.InputSchema has pruned every Business-Edition-only
// field a Community catalog must never see (see that method's own doc
// comment for the mechanism this exists to keep visible in this report).
type editionFieldCounts struct {
	Actions int
	Fields  int
}

// editionFieldCountsResult is both editions' editionFieldCounts.
//
// This is deliberately its own small result, computed independently of
// auditResult's own ActionsAudited/FieldsAudited (audit.go): those two
// compare the catalog's *unpruned* spec.InputSchema against the vendored
// specification — deliberately so, per Task 2's own seam decision (see
// actioncatalog.Catalog.InputSchema's doc comment: this audit "never builds
// a Catalog at all ... so its comparison ... stays unpruned by construction,
// not by a special case"). Folding per-edition catalog counts into that same
// comparison would blur two different questions this report answers: "does
// the catalog still match the specification it was generated from" (gating,
// unpruned, auditResult's job) and "is per-field edition pruning (Task 2)
// still actually removing anything" (informational only, this type's job).
type editionFieldCountsResult struct {
	EE editionFieldCounts
	CE editionFieldCounts
}

// auditEditionFieldCounts builds the real catalog for both Community and
// Business Edition from actions — the identical declared specs auditDrift
// itself audits (wiring.AllSpecs(), in every real run) — and sums each
// catalog's own published field count.
//
// Nothing here gates the build: a silent regression in Task 2's per-field
// pruning does not, on its own, mean the catalog has drifted from the
// vendored specification (auditResult's own concern), so this never
// contributes to HasDrift. It exists only so buildReport can print the one
// number that would otherwise go unnoticed if that pruning silently stopped:
// today, Community Edition is offered 40 fields across 15 actions against
// Business Edition's 51 across 18 (registries.create/update's "Github" and
// registries.inspect's "endpointId" pruned from the 14 actions both editions
// share); an implementation that stopped pruning would make Community's
// count climb to Business's own 51, invisible in "Fields audited: 51" alone.
//
// This is a separate function from auditDrift, not a parameter threaded
// through it, because auditDrift's own tests (audit_test.go) declare actions
// against invented OperationIDs ("Fixture", "NoSuchOperation", ...) that
// deliberately do not resolve in the real applicability table — exactly what
// several of those tests need in order to prove auditDrift's own OperationID
// cross-check. actioncatalog.Build refuses exactly that unconditionally (see
// its own doc comment), so calling it from inside auditDrift would fail
// every one of those fixtures for a reason unrelated to what each test
// proves. Called only from run(), against the one real actions slice this
// whole command otherwise audits.
func auditEditionFieldCounts(actions []toolutil.ActionSpec, serverVersion string) (*editionFieldCountsResult, error) {
	ee, err := oneEditionFieldCounts(actions, edition.EE, serverVersion)
	if err != nil {
		return nil, fmt.Errorf("business edition catalog: %w", err)
	}
	ce, err := oneEditionFieldCounts(actions, edition.CE, serverVersion)
	if err != nil {
		return nil, fmt.Errorf("community edition catalog: %w", err)
	}
	return &editionFieldCountsResult{EE: ee, CE: ce}, nil
}

// oneEditionFieldCounts builds actions' catalog for ed and sums the
// top-level "properties" every action in it publishes through
// Catalog.InputSchema — the identical map every real tool surface (dynamic,
// meta, individual) hands a model, pruning included.
func oneEditionFieldCounts(actions []toolutil.ActionSpec, ed edition.Edition, serverVersion string) (editionFieldCounts, error) {
	catalog, err := actioncatalog.Build(actions, actioncatalog.Options{Edition: ed, ServerVersion: serverVersion})
	if err != nil {
		return editionFieldCounts{}, fmt.Errorf("build %s catalog: %w", ed, err)
	}

	counts := editionFieldCounts{Actions: len(catalog.Actions())}
	for _, action := range catalog.Actions() {
		schema, err := catalog.InputSchema(action)
		if err != nil {
			return editionFieldCounts{}, fmt.Errorf("%s catalog input schema for %s: %w", ed, action.Name, err)
		}
		props, _ := schema["properties"].(map[string]any)
		counts.Fields += len(props)
	}
	return counts, nil
}
