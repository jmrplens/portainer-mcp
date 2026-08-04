package toolutil

import (
	"reflect"
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

	got := FieldEditions(reflect.TypeOf(input{}))
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
	if got := FieldEditions(reflect.TypeOf(plain{})); got != nil {
		t.Errorf("FieldEditions() = %v, want nil for a struct with no edition tag", got)
	}
}

// TestUnit_FieldEditions_NonEETagValue_IsIgnored guards the one meaningful
// value this tag has today: the generator only ever writes "EE" (see
// cmd/gen_action_inputs/fields.go), and a stray value — a typo, or an
// explicit "CE" that would be redundant since Community is already every
// untagged field's default — must not be reported as gating anything.
func TestUnit_FieldEditions_NonEETagValue_IsIgnored(t *testing.T) {
	t.Parallel()
	type input struct {
		Name string `json:"name" edition:"CE"`
	}
	if got := FieldEditions(reflect.TypeOf(input{})); got != nil {
		t.Errorf("FieldEditions() = %v, want nil: an edition:\"CE\" tag gates nothing", got)
	}
}

// TestUnit_FieldEditions_NilOrNonStruct_ReturnsNil guards the same
// non-struct/nil handling toolutil.jsonFieldNames and gitlab-mcp-server's
// FieldTiers both apply, so a caller that hands this a pointer type, a nil
// reflect.Type, or a non-struct type gets a safe empty answer rather than a
// panic reaching in through NumField.
func TestUnit_FieldEditions_NilOrNonStruct_ReturnsNil(t *testing.T) {
	t.Parallel()
	if got := FieldEditions(nil); got != nil {
		t.Errorf("FieldEditions(nil) = %v, want nil", got)
	}
	if got := FieldEditions(reflect.TypeOf("")); got != nil {
		t.Errorf("FieldEditions(string) = %v, want nil", got)
	}

	type input struct {
		Name string `json:"name" edition:"EE"`
	}
	if got := FieldEditions(reflect.TypeOf(&input{})); len(got) != 1 {
		t.Errorf("FieldEditions(*input) = %v, want the pointer dereferenced to the struct", got)
	}
}
