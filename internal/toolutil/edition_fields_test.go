package toolutil

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// TestUnit_FieldEditions_TaggedField_IsReportedEE mirrors
// gitlab-mcp-server's TestFieldTiers_ReadsTierTag: an `edition:"EE"` tag is
// read (keyed by JSON name), an anonymous embed is flattened so a tag on one
// of its own fields is still found, an untagged field is absent from the
// result, and a `json:"-"` field is skipped even though it carries the tag.
func TestUnit_FieldEditions_TaggedField_IsReportedEE(t *testing.T) {
	t.Parallel()

	type embedded struct {
		TLSCACert string `json:"tlsCaCert,omitempty" edition:"EE"`
	}
	type input struct {
		embedded
		Name    string `json:"name"`
		EdgeKey string `json:"edgeKey,omitempty" edition:"EE"`
		Hidden  string `json:"-" edition:"EE"`
	}

	got, err := FieldEditions(reflect.TypeOf(input{}))
	if err != nil {
		t.Fatalf("FieldEditions() error = %v", err)
	}
	want := map[string]edition.Edition{"tlsCaCert": edition.EE, "edgeKey": edition.EE}
	if len(got) != len(want) {
		t.Fatalf("FieldEditions() = %v, want %v", got, want)
	}
	for name, ed := range want {
		if got[name] != ed {
			t.Errorf("FieldEditions()[%q] = %q, want %q", name, got[name], ed)
		}
	}
	if _, ok := got["name"]; ok {
		t.Error("untagged field \"name\" must not appear")
	}
	if _, ok := got["-"]; ok {
		t.Error("a json:\"-\" field must never appear, tagged or not")
	}
}

// TestUnit_FieldEditions_NoTaggedField_ReturnsNil is the property
// Catalog.InputSchema's own "no needless clone" guarantee depends on: a
// struct with nothing gated must report exactly no fields, not an empty-but-
// non-nil map a caller might mistake for "computed, found nothing to gate"
// versus "never checked".
func TestUnit_FieldEditions_NoTaggedField_ReturnsNil(t *testing.T) {
	t.Parallel()
	type plain struct {
		Name string `json:"name"`
	}
	got, err := FieldEditions(reflect.TypeOf(plain{}))
	if err != nil {
		t.Fatalf("FieldEditions() error = %v", err)
	}
	if got != nil {
		t.Errorf("FieldEditions() = %v, want nil for a struct with no edition tag", got)
	}
}

// TestUnit_FieldEditions_ExplicitCETagValue_IsIgnoredNotRefused guards the
// one other recognised value this tag has: edition.Edition has exactly two
// values, CE and EE, and an explicit `edition:"CE"` is a documented no-op —
// every untagged field is already Community by default — not an error.
func TestUnit_FieldEditions_ExplicitCETagValue_IsIgnoredNotRefused(t *testing.T) {
	t.Parallel()
	type input struct {
		Name string `json:"name" edition:"CE"`
	}
	got, err := FieldEditions(reflect.TypeOf(input{}))
	if err != nil {
		t.Fatalf("FieldEditions() error = %v, want nil: an explicit edition:\"CE\" tag is a recognised no-op, not a refusal", err)
	}
	if got != nil {
		t.Errorf("FieldEditions() = %v, want nil: an edition:\"CE\" tag gates nothing", got)
	}
}

// TestUnit_FieldEditions_UnrecognisedTagValue_IsRefused is I6's fix: an
// earlier revision of this function silently ignored any tag value other
// than "EE" — including "BE", a value from an unrelated tiering convention —
// on the premise that "nothing hand-writes this tag", a premise
// internal/tools/registries/inputs.go's own hand-written edition:"EE" tags
// (registryCreateInput.Github and siblings) had already falsified two
// commits later. A value this project's edition.Edition type does not
// recognise must fail loudly, naming the field and the value found, rather
// than fail open and quietly publish a field a Community server has never
// heard of.
func TestUnit_FieldEditions_UnrecognisedTagValue_IsRefused(t *testing.T) {
	t.Parallel()
	type input struct {
		Name string `json:"name" edition:"BE"`
	}
	_, err := FieldEditions(reflect.TypeOf(input{}))
	if err == nil {
		t.Fatal("FieldEditions() error = nil, want a refusal for edition:\"BE\", a value edition.Edition does not recognise")
	}
	for _, want := range []string{"Name", "BE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("FieldEditions() error = %q, want it to name %q", err, want)
		}
	}
}

// TestUnit_FieldEditions_CommaSuffixedEETagValue_IsTolerated is I6's other
// half: `edition:"EE,omitempty"` is the shape a reader familiar with this
// project's own `json`/`jsonschema` tag convention (both use a
// comma-separated first segment as the real value) would naturally reach
// for, and must gate the field exactly the same as a bare `edition:"EE"` —
// not be refused as an unrecognised value, and not be silently ignored
// either.
func TestUnit_FieldEditions_CommaSuffixedEETagValue_IsTolerated(t *testing.T) {
	t.Parallel()
	type input struct {
		Name    string `json:"name"`
		EdgeKey string `json:"edgeKey,omitempty" edition:"EE,omitempty"`
	}
	got, err := FieldEditions(reflect.TypeOf(input{}))
	if err != nil {
		t.Fatalf("FieldEditions() error = %v, want a comma-suffixed edition:\"EE,omitempty\" tolerated", err)
	}
	if got["edgeKey"] != edition.EE {
		t.Errorf("FieldEditions() = %v, want edgeKey gated as EE", got)
	}
}

// TestUnit_FieldEditions_NilOrNonStruct_ReturnsNil guards the same
// non-struct/nil handling toolutil.jsonFieldNames and gitlab-mcp-server's
// FieldTiers both apply, so a caller that hands this a pointer type, a nil
// reflect.Type, or a non-struct type gets a safe empty answer rather than a
// panic reaching in through NumField.
func TestUnit_FieldEditions_NilOrNonStruct_ReturnsNil(t *testing.T) {
	t.Parallel()
	type input struct {
		Name string `json:"name" edition:"EE"`
	}
	cases := []struct {
		name    string
		rt      reflect.Type
		wantLen int
		wantNil bool
	}{
		{"nil type", nil, 0, true},
		{"non-struct type", reflect.TypeOf(""), 0, true},
		{"pointer to struct", reflect.TypeOf(&input{}), 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := FieldEditions(tc.rt)
			if err != nil {
				t.Fatalf("FieldEditions() error = %v", err)
			}
			if len(got) != tc.wantLen {
				t.Errorf("FieldEditions() = %v, want %d gated field(s)", got, tc.wantLen)
			}
			// len(got) == 0 is true for both nil and an empty map, so the
			// documented nil contract needs its own assertion.
			if tc.wantNil && got != nil {
				t.Errorf("FieldEditions() = %v, want nil", got)
			}
		})
	}
}
