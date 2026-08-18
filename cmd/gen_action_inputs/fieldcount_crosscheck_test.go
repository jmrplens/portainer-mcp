package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// rawOperationIDsByExportedName scans paths directly (bypassing
// operationsByDomain, which only keeps the already-exported form — see
// operation.OperationID's own doc comment) and returns a map from each
// operation's exported OperationID back to the raw operationId the vendored
// specification actually declares ("systemInfo", not "SystemInfo").
// specdiff.LoadSpecOperation matches against that raw string verbatim, with
// no case-folding of its own (mirroring cmd/audit_spec_drift's own
// exportedName/parseSpecOperations split for the identical reason): calling
// it with an already-exported name — the mistake an early draft of this
// test made — makes every operation whose raw operationId starts with a
// lowercase letter silently fail to resolve, misreporting dozens of
// legitimate agreements as disagreements.
func rawOperationIDsByExportedName(paths map[string]map[string]json.RawMessage) map[string]string {
	out := map[string]string{}
	for _, methods := range paths {
		for method, raw := range methods {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			var o struct {
				OperationID string `json:"operationId"`
			}
			if err := json.Unmarshal(raw, &o); err != nil || o.OperationID == "" {
				continue
			}
			out[exportedName(o.OperationID)] = o.OperationID
		}
	}
	return out
}

// knownFieldCountResidual names every operation, in either vendored
// specification, where this package's own assembleOperationFields and
// internal/specdiff.ShapeFromSpec currently disagree about how many
// top-level fields an operation has — measured directly, not assumed zero,
// after C1 (non-object bodies) and C2 (map-shaped bodies) closed the two
// instances of this class that were actually armed against the current
// 19-action catalog. Every entry here is a live, open gap, named so it
// cannot be mistaken for "already handled" the way C1 and C2 both went
// unnoticed until inspected directly.
//
// One root cause remains, of two this map used to carry:
//
//   - "multipart" (closed): the requestBody's only content type was
//     multipart/form-data. This package's requestBodySchema always resolved
//     a multipart schema into real fields (file uploads and their
//     metadata); internal/specdiff.requestBodySchemaNode used to look only
//     at "application/json" content and treat a multipart-only body as no
//     body at all, reporting 0 (or, when the operation also has path/query
//     parameters, only those) fields — the same failure shape C1 and C2
//     closed (the generator emits real fields; specdiff silently reports
//     fewer), on a third dimension (content type, not body top-level type)
//     neither fix touched. The 13 operations this named ("Create",
//     "CloudCredsUpdate", "CustomTemplateCreateFile", "EdgeConfigCreate",
//     "EdgeConfigUpdate", "EdgeJobCreateFile", "EdgeStackCreateFile",
//     "EdgeStackParseRegistries", "EndpointCreate",
//     "EndpointDockerBrowsePut", "StackCreateDockerStandaloneFile",
//     "StackCreateDockerSwarmFile", "UploadTLS" — every multipart-only
//     requestBody in either vendored spec) are gone from this map because
//     specdiff.requestBodySchemaNode now reads whichever single content
//     type a requestBody declares, not only "application/json". None of
//     these operations was declared by any action in the catalog when this
//     closed, so closing it here is what keeps every one of wave-1's five
//     real multipart operations (EndpointCreate, CustomTemplateCreateFile,
//     both StackCreateDockerSwarm/StandaloneFile,
//     EndpointDockerBrowsePut) from needing an api/spec-drift-allowlist.yaml
//     entry per field the moment a catalog action declares one. (
//     CustomTemplateCreate declares both application/json and
//     multipart/form-data content: the generator refuses outright, on
//     principle, because no single Go type represents both; specdiff now
//     refuses too, for the identical reason (more than one content type,
//     see requestBodySchemaNode's own doc comment) rather than incidentally
//     landing on the same free-form-object refusal it used to reach through
//     the application/json variant alone. Still not listed below: the two
//     sides still agree, by refusing, just for the same reason now instead
//     of two different ones.)
//
//   - "generator-refuses" (still open): this package refuses
//     to scaffold because a *nested* value inside the body has no
//     expressible type ("map value: schema has no type" — an
//     additionalProperties value schema declaring no "type" keyword at any
//     level), while specdiff.ShapeFromSpec succeeds, because it never needs
//     to resolve past a body property's own top-level type/description/enum
//     (see resolvedNode's own doc comment: OperationShape is flat by
//     design). This direction is the opposite of the dangerous one: the
//     generator's own refusal already means none of these operations can
//     ever be mechanically scaffolded, so there is no risk of a catalog
//     action existing for one of them whose real shape this package's field
//     count then contradicts — the failure mode C1/C2/the closed multipart
//     class shared (generator succeeds, specdiff undercounts) cannot occur
//     here, because the generator never succeeds.
//
//     CreateKubernetesService and UpdateKubernetesService joined this class
//     without themselves changing: both always carried this same
//     unresolvable nested map value ("Applications[].Configurations[].Data"),
//     but until internal/specnaming existed, specdiff refused them too — for
//     the unrelated reason that their path parameter "namespace" collides
//     with their body property "Namespace" — so the two sides agreed by both
//     refusing, and neither needed naming here. Disambiguating that
//     collision let specdiff shape them and left this package's own, older
//     refusal standing alone. They are the only two of the five colliding
//     operations in that position: CreateKubernetesIngress and
//     UpdateKubernetesIngress now shape on both sides, and so does
//     StackMigrate. They are also the only two entries here that are not
//     EE-only.
//
// Every entry must currently correspond to a real disagreement:
// TestUnit_FieldCounts_GeneratorAndSpecdiffAgree_AcrossBothVendoredSpecs
// fails on both a disagreement absent from this map (unnamed regression) and
// an entry present here that no longer disagrees anywhere (stale entry
// silently forgiving whatever unrelated operation happens to reuse its
// OperationID next) — the identical "excluded from the failure, never
// allowed to go stale unnoticed" discipline cmd/audit_1to1's and
// cmd/audit_spec_drift's own allow-lists apply, for the identical reason.
var knownFieldCountResidual = map[string]string{
	"AddonInstall":            "generator-refuses",
	"CreateKubernetesService": "generator-refuses",
	"UpdateKubernetesService": "generator-refuses",
	"OmniCreateCluster":       "generator-refuses",
	"OmniUpdateCluster":       "generator-refuses",
	"OmniValidateCluster":     "generator-refuses",
	"PolicyUpdate":            "generator-refuses",
	"UpdateAlertingSettings":  "generator-refuses",
}

// TestUnit_FieldCounts_GeneratorAndSpecdiffAgree_AcrossBothVendoredSpecs is
// the standing guard the coordinator asked for after C2 (the map-bodied
// Kubernetes bulk-delete fix): internal/specdiff.ShapeFromSpec is a
// reimplementation of this package's own assembleOperationFields (see
// specdiff.ShapeFromSpec's own doc comment for why reimplemented rather than
// imported — this package is `package main`), and a reimplementation that
// silently drifts from what it mirrors is exactly the defect class this
// project keeps re-discovering: C1 found it for a non-object body, C2 found
// it for a map-shaped one. Both were found by inspection, one operation at a
// time. This test instead runs every operation with an operationId in both
// vendored specifications through both derivations and fails the moment
// their field counts disagree on an operation that is neither excluded by
// mutual refusal nor already named in knownFieldCountResidual — the
// generalisation of the manual check that found C1 and C2, rather than
// trusting the next such gap to be found the same accidental way.
//
// "Field count" here, not full structural equality: assembleOperationFields
// and ShapeFromSpec are allowed to name a field differently in prose (a
// description) without that being a defect this test cares about — Compare
// already exists for structural equality, at the one-operation-at-a-time
// scale internal/specdiff's own tests exercise it at. This test's job is
// narrower and complementary: across literally every operation, does the
// generator and this package's own comparison engine even agree on *how
// many* top-level fields a body/parameter set contributes. A silent
// discrepancy there is worse than a wrong description, because it is
// invisible to Compare too when the field is simply missing from one side's
// output entirely — the exact C1/C2 failure mode.
//
// An operation is excluded from comparison entirely, counted as neither a
// disagreement nor a known residual, when both sides refuse it outright (an
// unsupported schema shape: oneOf/anyOf, a header parameter, a genuinely
// free-form object with neither properties nor a typed additionalProperties,
// and so on): both this package and specdiff refusing the identical
// operation for the identical shape is agreement, not disagreement.
//
// Deliberately not t.Parallel() at the top level, unlike almost every other
// test in this repository: the closing subtest below needs every named
// residual entry's hit recorded before it runs, which requires the two
// per-spec subtests to have actually executed their bodies first — true
// only if they run synchronously with the parent, not paused until the
// parent function returns the way a t.Parallel() subtest would be.
func TestUnit_FieldCounts_GeneratorAndSpecdiffAgree_AcrossBothVendoredSpecs(t *testing.T) {
	hitsSeen := make(map[string]bool, len(knownFieldCountResidual))

	for _, specPath := range []string{
		"../../api/specs/ee-2.44.0.json",
		"../../api/specs/ce-2.44.0.json",
	} {
		t.Run(specPath, func(t *testing.T) {
			data, err := os.ReadFile(specPath)
			if err != nil {
				t.Fatalf("read %s: %v", specPath, err)
			}

			doc, paths, err := loadDocument(specPath)
			if err != nil {
				t.Fatalf("loadDocument(%s) error = %v", specPath, err)
			}
			byDomain, err := operationsByDomain(paths)
			if err != nil {
				t.Fatalf("operationsByDomain(%s) error = %v", specPath, err)
			}
			rawIDs := rawOperationIDsByExportedName(paths)

			var allOps []operation
			for _, ops := range byDomain {
				allOps = append(allOps, ops...)
			}
			sort.Slice(allOps, func(i, j int) bool { return allOps[i].OperationID < allOps[j].OperationID })

			var (
				compared      int
				bothRefused   int
				knownResidual int
				disagreements []string
			)
			for _, op := range allOps {
				res := &resolver{doc: doc}
				var nested []structSpec
				genFields, _, genErr := assembleOperationFields(op, res, doc, inputStructName(op.OperationID), &nested)

				rawID, ok := rawIDs[op.OperationID]
				if !ok {
					t.Fatalf("no raw operationId found for exported name %q; rawOperationIDsByExportedName and operationsByDomain disagree about what this document declares", op.OperationID)
				}
				specOp, specErr := specdiff.LoadSpecOperation(data, rawID)
				var shape specdiff.OperationShape
				var shapeErr error
				if specErr == nil {
					shape, shapeErr = specdiff.ShapeFromSpec(specOp)
				} else {
					shapeErr = specErr
				}

				disagrees := false
				switch {
				case genErr != nil && shapeErr != nil:
					bothRefused++
				case genErr != nil || shapeErr != nil:
					disagrees = true
				default:
					compared++
					if len(genFields) != len(shape.Fields) {
						disagrees = true
					}
				}

				if !disagrees {
					continue
				}
				if _, named := knownFieldCountResidual[op.OperationID]; named {
					knownResidual++
					hitsSeen[op.OperationID] = true
					continue
				}
				disagreements = append(disagreements, fmt.Sprintf(
					"%s: generator error=%v (fields=%d), specdiff error=%v (fields=%d)",
					op.OperationID, genErr, len(genFields), shapeErr, len(shape.Fields)))
			}

			sort.Strings(disagreements)
			t.Logf("%s: %d operation(s) compared, %d both refused, %d known residual, %d unnamed disagreement(s)",
				specPath, compared, bothRefused, knownResidual, len(disagreements))
			if len(disagreements) > 0 {
				t.Errorf("%d unnamed field-count disagreement(s) between the generator and specdiff.ShapeFromSpec — either fix them or name them in knownFieldCountResidual with a reason:\n%s",
					len(disagreements), joinLines(disagreements))
			}
		})
	}

	t.Run("every named residual entry still disagrees somewhere", func(t *testing.T) {
		var stale []string
		for id := range knownFieldCountResidual {
			if !hitsSeen[id] {
				stale = append(stale, id)
			}
		}
		sort.Strings(stale)
		if len(stale) > 0 {
			t.Errorf("%d knownFieldCountResidual entr(y/ies) no longer disagree with the generator in either vendored spec — remove them, the gap they named is closed:\n%s",
				len(stale), joinLines(stale))
		}
	})
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += "  - " + l + "\n"
	}
	return out
}
