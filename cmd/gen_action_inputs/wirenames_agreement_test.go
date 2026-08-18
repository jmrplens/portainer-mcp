package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// TestUnit_WireNames_MatchSpecdiffOnEveryRealOperation is the standing
// anti-drift check for the disambiguation rule internal/specnaming holds and
// both this package and internal/specdiff apply.
//
// The hazard is specific, and worse than a plain disagreement. cmd/audit_spec_drift
// compares a declared action's published InputSchema (whose field names this
// package produced) against specdiff.ShapeFromSpec's names for the same
// operation. If the two sides ever name one field differently, the audit does
// not report "these two implementations disagree" — it reports one field
// added and one field removed, on an operation nobody has touched, and the
// obvious repair is an api/spec-drift-allowlist.yaml entry excusing a
// divergence that does not exist. One failure would have been traded for a
// subtler one.
//
// Following TestUnit_CleanTitleAndDescription_MatchesSpecdiffOnEveryRealOperation
// (actionspec_test.go), this runs *both* derivations over *every* operation in
// *both* vendored specifications, every time the suite runs: a one-off
// comparison proves the two agree today and says nothing about tomorrow.
// It compares the full ordered list of (JSON name, origin) pairs, not merely
// the disambiguated ones, so it also pins the two verbatim copies of
// bodyJSONTag/splitWords (see internal/specdiff/naming.go) against each
// other — previously only their field *counts* were cross-checked
// (TestUnit_FieldCounts_GeneratorAndSpecdiffAgree_AcrossBothVendoredSpecs),
// which a pure renaming passes untouched.
//
// Operations either side refuses are skipped rather than compared: that
// direction is the field-count cross-check's job, and it names every open
// residual there. This test's premise is instead pinned by disambiguated
// below — if the rule ever stopped firing anywhere, agreement would become
// trivially true and this test would say nothing at all.
func TestUnit_WireNames_MatchSpecdiffOnEveryRealOperation(t *testing.T) {
	t.Parallel()

	// The five operations in the two vendored documents where a parameter and
	// a request-body property contribute the same wire name — measured, not
	// assumed: every operation in both specs was scanned when this rule was
	// written. StackMigrate is the query-parameter case; the other four are
	// the path-parameter case (see internal/specnaming's doc comment for why
	// the body wins in each).
	collides := map[string]string{
		"StackMigrate":            "endpointIdQuery",
		"CreateKubernetesIngress": "namespacePath",
		"UpdateKubernetesIngress": "namespacePath",
		"CreateKubernetesService": "namespacePath",
		"UpdateKubernetesService": "namespacePath",
	}
	// Of those five, the two Kubernetes *service* operations cannot be
	// observed here: this package refuses them outright for a reason that has
	// nothing to do with the collision (a nested additionalProperties value
	// schema declaring no type — the "generator-refuses" class named in
	// knownFieldCountResidual), so the loop below never gets fields for them
	// to inspect. They stay in collides so that a future fix to that refusal
	// does not trip the "fired somewhere unexpected" check; they are excluded
	// from mustObserve because requiring them would be asserting something
	// this test cannot see.
	unobservable := map[string]bool{"CreateKubernetesService": true, "UpdateKubernetesService": true}
	disambiguated := map[string]bool{}

	compared := 0
	for _, specPath := range []string{"../../api/specs/ee-2.44.0.json", "../../api/specs/ce-2.44.0.json"} {
		data, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read %s: %v", specPath, err)
		}
		doc, paths, err := loadDocument(specPath)
		if err != nil {
			t.Fatalf("loadDocument(%s) error = %v", specPath, err)
		}
		byTag, err := operationsByDomain(paths)
		if err != nil {
			t.Fatalf("operationsByDomain(%s) error = %v", specPath, err)
		}
		rawIDs := rawOperationIDsByExportedName(paths)

		var allOps []operation
		for _, ops := range byTag {
			allOps = append(allOps, ops...)
		}
		sort.Slice(allOps, func(i, j int) bool { return allOps[i].OperationID < allOps[j].OperationID })

		for _, op := range allOps {
			res := &resolver{doc: doc}
			var nested []structSpec
			genFields, _, genErr := assembleOperationFields(op, res, doc, inputStructName(op.OperationID), &nested)
			if genErr != nil {
				continue
			}
			rawID, ok := rawIDs[op.OperationID]
			if !ok {
				t.Fatalf("%s: no raw operationId for exported name %q", specPath, op.OperationID)
			}
			specOp, err := specdiff.LoadSpecOperation(data, rawID)
			if err != nil {
				continue
			}
			shape, err := specdiff.ShapeFromSpec(specOp)
			if err != nil {
				continue
			}

			for _, f := range genFields {
				if f.WireName == "" || f.WireName == f.JSONName {
					continue
				}
				disambiguated[op.OperationID] = true
				if want, known := collides[op.OperationID]; known && f.JSONName != want {
					t.Errorf("%s: %s: parameter %q was published as %q, want %q",
						specPath, op.OperationID, f.WireName, f.JSONName, want)
				}
			}

			got := make([]string, 0, len(genFields))
			for _, f := range genFields {
				got = append(got, f.Origin+" "+f.JSONName)
			}
			want := make([]string, 0, len(shape.Fields))
			for _, f := range shape.Fields {
				want = append(want, f.Origin+" "+f.JSONName)
			}
			if strings.Join(got, ", ") != strings.Join(want, ", ") {
				t.Errorf("%s: %s: assembleOperationFields published %v, specdiff.ShapeFromSpec published %v; the two must name every field identically or cmd/audit_spec_drift reports a phantom added/removed field",
					specPath, op.OperationID, got, want)
			}
			compared++
		}
	}

	if compared < 400 {
		t.Errorf("only %d operation(s) were compared across both vendored specifications; this test's premise (both sides shaping most of the catalog) went stale", compared)
	}

	var missing []string
	for id := range collides {
		if !disambiguated[id] && !unobservable[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the disambiguation rule never fired for %v; either the rule stopped applying (in which case every comparison above is trivially true and proves nothing) or these operations changed in the vendored specifications and this list must be re-measured",
			missing)
	}
	for id := range disambiguated {
		if _, known := collides[id]; !known {
			t.Errorf("the disambiguation rule fired for %s, which is not one of the five operations it was measured to apply to; a rule that renames more than it must is a rename of the whole catalog waiting to happen", id)
		}
	}
}

// TestUnit_WireNames_StackMigrate_BothSidesNameTheSameTwoFields asserts the
// specific names, on both sides, for the operation the rule was written for.
// The test above proves the two sides agree; this one proves what they agree
// *on*. They would agree just as well if both had qualified the body instead
// of the parameter, or if both had dropped the query parameter entirely.
func TestUnit_WireNames_StackMigrate_BothSidesNameTheSameTwoFields(t *testing.T) {
	t.Parallel()
	for _, specPath := range []string{"../../api/specs/ee-2.44.0.json", "../../api/specs/ce-2.44.0.json"} {
		t.Run(specPath, func(t *testing.T) {
			t.Parallel()
			doc, paths, err := loadDocument(specPath)
			if err != nil {
				t.Fatalf("loadDocument(%s) error = %v", specPath, err)
			}
			byTag, err := operationsByDomain(paths)
			if err != nil {
				t.Fatalf("operationsByDomain(%s) error = %v", specPath, err)
			}

			var migrate operation
			for _, op := range byTag["stacks"] {
				if op.OperationID == "StackMigrate" {
					migrate = op
				}
			}
			if migrate.OperationID == "" {
				t.Fatalf("StackMigrate is not tagged stacks in %s; this test's premise went stale", specPath)
			}

			res := &resolver{doc: doc}
			var nested []structSpec
			fields, _, err := assembleOperationFields(migrate, res, doc, "stackMigrateInput", &nested)
			if err != nil {
				t.Fatalf("assembleOperationFields(StackMigrate) error = %v, want the collision disambiguated, not refused", err)
			}
			generated := map[string]fieldSpec{}
			for _, f := range fields {
				generated[f.JSONName] = f
			}

			body, ok := generated["endpointId"]
			if !ok {
				t.Fatalf("the generated Input has no %q field: %v", "endpointId", fieldNames(fields))
			}
			if body.Origin != originBody || body.GoName != "EndpointID" || body.GoType != "int" {
				t.Errorf("generated %q = {Origin: %s, GoName: %s, GoType: %s}, want {body, EndpointID, int}: the required body property that names the migration target keeps the plain name",
					"endpointId", body.Origin, body.GoName, body.GoType)
			}

			query, ok := generated["endpointIdQuery"]
			if !ok {
				t.Fatalf("the generated Input has no %q field: %v", "endpointIdQuery", fieldNames(fields))
			}
			if query.Origin != originQuery || query.GoName != "EndpointIDQuery" || query.WireName != "endpointId" {
				t.Errorf("generated %q = {Origin: %s, GoName: %s, WireName: %s}, want {query, EndpointIDQuery, endpointId}",
					"endpointIdQuery", query.Origin, query.GoName, query.WireName)
			}
			if query.GoType != "*int" {
				t.Errorf("generated %q GoType = %s, want *int: the specification declares this parameter optional", "endpointIdQuery", query.GoType)
			}

			// The Go field names must differ too, not only the JSON tags: two
			// fields named EndpointID in one struct is a compile error, which
			// is the one failure mode a JSON-name-only check would miss.
			if body.GoName == query.GoName {
				t.Errorf("both fields render as Go field %s; the generated struct would not compile", body.GoName)
			}

			// And the mechanical handler for it is refused: the round trip
			// that distributes query parameters matches on the
			// specification's own name, which this field no longer carries.
			if _, err := buildHandlerSpec("stacks", migrate, fields, nil, nested, "stackMigrateInput", ""); err == nil {
				t.Error("buildHandlerSpec(StackMigrate) error = nil, want a refusal: a renamed query parameter cannot be distributed by the generated JSON round trip")
			} else if !strings.Contains(err.Error(), "endpointIdQuery") {
				t.Errorf("buildHandlerSpec(StackMigrate) error = %v, want it to name the renamed field", err)
			}
		})
	}
}

// fieldNames renders a field list for a failure message.
func fieldNames(fields []fieldSpec) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, fmt.Sprintf("%s(%s)", f.JSONName, f.Origin))
	}
	return out
}
