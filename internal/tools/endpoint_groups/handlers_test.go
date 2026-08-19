package endpoint_groups

import (
	"os"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// This domain carried no test file at all until it grew its first
// hand-written ActionSpec and handler (endpoint_groups.inspect, for the route
// neither vendored document names). The two tests below are the domain-local
// guards every sibling that hand-writes a spec already carries, and they are
// deliberately the two that a hand-written declaration can get wrong on its
// own: the shape it publishes, and whether its prose ever reaches a model.

// vendoredSpec is the Business Edition document cmd/gen_action_inputs
// generates from and cmd/audit_spec_drift compares against first
// (resolveSpecOperation's EE-then-CE precedence), mirroring
// internal/tools/stacks/handlers_test.go's identically-named constant.
//
// Every route this domain declares is in it, including
// GET /endpoint_groups/{id} — which is the whole point: that route is fully
// described there (summary, description, both parameters, a response schema)
// and carries no operationId, so it is findable only through the name
// internal/specnaming's table gives it.
const vendoredSpec = "../../../api/specs/ee-2.44.0.json"

// TestUnit_DeclaredInputs_PublishTheShapeTheSpecificationDeclares runs the
// real drift comparison over every action this domain declares, following
// internal/tools/stacks/handlers_test.go's
// TestUnit_HandWrittenInputs_PublishTheShapeTheSpecificationDeclares.
//
// It is not a duplicate of cmd/audit_spec_drift. It is the same comparison
// reached by a different route, and that difference is the point:
// endpoint_groups.inspect is resolved here through
// specdiff.LoadSpecOperation, and by the audit through that command's own
// parseSpecOperations. Both had to learn internal/specnaming's synthetic
// name, and until this test existed the specdiff half was held up by exactly
// one test written for it — revert internal/specdiff/shape.go's lookup and
// the whole of `go test ./...` and `make audit-spec-drift` stayed green.
// This is what makes that change load-bearing from inside the domain that
// depends on it.
//
// Run over all seven actions rather than only the hand-written one, because
// nothing about the check is specific to how a spec was authored: the six
// scaffolded ones were frozen as ordinary source the day they were generated
// and can drift from the document by hand-edit exactly as this one can.
func TestUnit_DeclaredInputs_PublishTheShapeTheSpecificationDeclares(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(vendoredSpec)
	if err != nil {
		t.Fatalf("read %s: %v", vendoredSpec, err)
	}

	specs := Specs()
	if len(specs) == 0 {
		t.Fatal("Specs() declared nothing; this test would compare nothing and pass")
	}

	for _, spec := range specs {
		t.Run(spec.Name, func(t *testing.T) {
			t.Parallel()

			catalog, err := specdiff.ShapeFromCatalog(spec)
			if err != nil {
				t.Fatalf("ShapeFromCatalog(%s): %v", spec.OperationID, err)
			}
			op, err := specdiff.LoadSpecOperation(data, spec.OperationID)
			if err != nil {
				t.Fatalf("LoadSpecOperation(%s): %v — a name the vendored document does not publish must still resolve through internal/specnaming's table", spec.OperationID, err)
			}
			vendored, err := specdiff.ShapeFromSpec(op)
			if err != nil {
				t.Fatalf("ShapeFromSpec(%s): %v", spec.OperationID, err)
			}

			for _, change := range specdiff.Compare(vendored, catalog) {
				switch change.Kind {
				case specdiff.ChangeTitle, specdiff.ChangeOperationDescription:
					if !change.AfterOverridden {
						t.Errorf("%s: %s on %s is not declared an override; build the spec through toolutil.WithNarrative",
							spec.OperationID, change.Kind, change.JSONName)
					}
				default:
					t.Errorf("%s: %s on field %q — before %q, after %q. This gates cmd/audit_spec_drift; the published shape must match the vendored one or carry a dated api/spec-drift-allowlist.yaml entry",
						spec.OperationID, change.Kind, change.JSONName, change.Before, change.After)
				}
			}
		})
	}
}

// TestUnit_EveryDeclaredAction_HasANarrative mirrors
// internal/tools/endpoints/handlers_test.go's identically-named check.
//
// narrative() has no default that returns the vendored wording: an action
// declared without a case there falls back to the specification's own
// sentence, silently. Nothing else notices — not the drift audit (catalog
// and document agree, so there is no drift to report), not the discovery
// audit, not any e2e test. The action keeps working and simply stops telling
// a model the measured facts it was written to carry.
func TestUnit_EveryDeclaredAction_HasANarrative(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if !s.TitleOverridden || !s.DescriptionOverridden {
			t.Errorf("%s (%s) has no narrative() entry", s.Name, s.OperationID)
		}
	}
}
