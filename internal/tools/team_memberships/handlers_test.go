package team_memberships

import (
	"os"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// The two tests below are the domain-local guards
// internal/tools/endpoint_groups/handlers_test.go already carries, brought
// here for the same reasons. Neither is specific to a hand-written action:
// every one of this domain's five specs was frozen as ordinary source the
// day it was scaffolded and can drift from the vendored document, or lose
// its narrative, by an ordinary hand edit from that moment on.

// vendoredSpec is the Business Edition document cmd/gen_action_inputs
// generated this domain from and cmd/audit_spec_drift compares against
// first (resolveSpecOperation's EE-then-CE precedence). Every one of this
// domain's five routes carries an operationId in it — including the bare
// "TeamMemberships" that names GET /teams/{id}/memberships, which is a real
// operationId the document publishes, not a synthetic one; what that name
// could not survive was cmd/gen_action_inputs's mechanical *action* naming
// rule, which is a separate table (actionNameOverrides) and not consulted
// here at all.
const vendoredSpec = "../../../api/specs/ee-2.44.0.json"

// TestUnit_DeclaredInputs_PublishTheShapeTheSpecificationDeclares runs the
// real drift comparison over every action this domain declares, reaching it
// through specdiff rather than through cmd/audit_spec_drift's own parser —
// the same difference internal/tools/endpoint_groups/handlers_test.go's
// identically-named test documents.
//
// It also pins the structural half of the narrative rule: a Title or
// Description that differs from the vendored wording and is NOT marked
// overridden fails here, which is exactly what an ActionSpec literal
// assigning Title/Description directly instead of routing through
// toolutil.WithNarrative would produce.
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
				t.Fatalf("LoadSpecOperation(%s): %v", spec.OperationID, err)
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
// internal/tools/endpoint_groups/handlers_test.go's identically-named check.
//
// narrative() has no default that returns the vendored wording: an action
// declared without a case there falls back to the specification's own
// sentence, silently. Nothing else notices — not the drift audit (catalog
// and document agree, so there is no drift to report), not the discovery
// audit, not any e2e test. That matters more here than in most domains:
// team_memberships.list and team_memberships.list_for_team are two
// differently-scoped reads whose vendored summaries are the identical string
// "List team memberships"; the narrative is the only thing that tells them
// apart for a model, and losing it costs nothing any other check can see.
func TestUnit_EveryDeclaredAction_HasANarrative(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if !s.TitleOverridden || !s.DescriptionOverridden {
			t.Errorf("%s (%s) has no narrative() entry", s.Name, s.OperationID)
		}
	}
}
