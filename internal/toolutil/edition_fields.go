package toolutil

import (
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// FieldEditions returns, for each top-level JSON field name of rt that
// requires Business Edition, edition.EE. A field requires it by carrying an
// `edition:"EE"` struct tag; a field without the tag — the common case,
// since it exists on a Community server too — is omitted, so a caller can
// tell "gated" from "ungated" by a plain map lookup rather than a default
// value that looks the same as "not present".
//
// This mirrors gitlab-mcp-server's FieldTiers (that sibling's
// internal/toolutil/metatool.go:463-500): rt may be a struct or a pointer to
// one (anything else, or nil, returns nil); an anonymous embedded struct is
// flattened into its parent exactly the way JSON field promotion already
// works (jsonFieldName below returns "" for an untagged anonymous field, the
// same signal FieldTiers' own jsonFieldName uses), so a tag on a promoted
// field is still found; an unexported, non-embedded field is skipped, since
// it can never be part of a JSON-reflected schema in the first place.
//
// The tag's value is read leniently in one specific way and strictly in
// every other: a trailing ",..." suffix is tolerated and ignored (the same
// convention this project's own `json` and `jsonschema` tags already use,
// so `edition:"EE,omitempty"` — the shape a reader familiar with either
// convention would naturally reach for — is treated identically to plain
// `edition:"EE"`), and the value left after that suffix is stripped must
// then be exactly "EE" or "CE", case-insensitively, or FieldEditions refuses
// with an error naming the field and the value it found. "CE" is accepted
// but gates nothing (every untagged field is already Community by default,
// so an explicit "CE" is a documented no-op, never an error); any other
// value — "BE", a typo, a value from an unrelated tiering scheme — is
// refused rather than silently ignored.
//
// An earlier revision of this function ignored any non-"EE" value silently,
// on the premise that "nothing hand-writes this tag" (cmd/gen_action_inputs
// is the only place it is generated). That premise did not survive its own
// branch: internal/tools/registries/inputs.go hand-writes three of these
// tags directly (see that file's own doc comment on registryCreateInput's
// Github field), with a comment stating the generator will never revisit
// that file to add or correct one. A hand-written tag is exactly where a
// typo — "EE " with trailing whitespace missed, "Ee", "BE" copied from a
// different project's tiering convention — becomes possible, and silent
// fail-open is the wrong default for a tag whose entire job is keeping a
// Business-only parameter off a Community catalog.
//
// cmd/gen_action_inputs/fields.go is the only place this tag is written
// mechanically, for a field present in the Business Edition operation's
// resolved schema and absent from the Community one; internal/tools/
// registries/inputs.go is the one place today it is written by hand.
func FieldEditions(rt reflect.Type) (map[string]edition.Edition, error) {
	if rt == nil {
		return nil, nil
	}
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil, nil
	}

	editions := map[string]edition.Edition{}
	for i := range rt.NumField() {
		field := rt.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue // unexported, and not a promoted anonymous field
		}
		jsonName, excluded := fieldEditionJSONName(field)
		if excluded {
			continue
		}
		if field.Anonymous && jsonName == "" {
			nested, err := FieldEditions(field.Type)
			if err != nil {
				return nil, err
			}
			maps.Copy(editions, nested)
			continue
		}
		if tag, ok := field.Tag.Lookup("edition"); ok {
			required, err := parseEditionTagValue(tag)
			if err != nil {
				return nil, fmt.Errorf("toolutil: %s.%s: %w", rt.Name(), field.Name, err)
			}
			if required == edition.EE {
				editions[jsonName] = edition.EE
			}
		}
	}
	if len(editions) == 0 {
		return nil, nil
	}
	return editions, nil
}

// parseEditionTagValue resolves one `edition:"..."` tag's raw value into the
// edition.Edition it names, tolerating a trailing ",..." suffix the way this
// project's own `json`/`jsonschema` tag convention already does (see
// FieldEditions' own doc comment for why that shape is the one a reader
// reaches for by habit, not a stretch to support). "EE" and "CE"
// (case-insensitively, either side of that suffix) are the only two
// recognised values — edition.Edition has no third — so anything else,
// comma-suffixed or not, is refused rather than silently treated as
// Community by default.
func parseEditionTagValue(tag string) (edition.Edition, error) {
	value, _, _ := strings.Cut(tag, ",")
	value = strings.TrimSpace(value)
	switch {
	case strings.EqualFold(value, string(edition.EE)):
		return edition.EE, nil
	case strings.EqualFold(value, string(edition.CE)):
		return edition.CE, nil
	default:
		return "", fmt.Errorf(
			"edition tag %q is neither %q nor %q (a trailing \",...\" suffix, as in %q, is tolerated)",
			tag, edition.CE, edition.EE, string(edition.EE)+",omitempty")
	}
}

// fieldEditionJSONName returns field's JSON property name the way
// encoding/json (and this project's own schema reflection, via
// google/jsonschema-go) would resolve it, mirroring guidance.go's
// jsonFieldNames precisely so a name valid there is valid here: the "json"
// tag's name portion when present (a literal "-," tag name, distinct from an
// excluding "-" tag, is honoured as a real field name), excluded entirely
// for a bare `json:"-"` tag, "" for an untagged anonymous field (promoted,
// so it has no name of its own to gate — a tag on the embedded type's own
// fields is still found, by FieldEditions' recursive call, the same
// anonymous-detection gitlab-mcp-server's jsonFieldName in metatool.go
// uses), and the Go field name otherwise.
func fieldEditionJSONName(field reflect.StructField) (name string, excluded bool) {
	tag, hasTag := field.Tag.Lookup("json")
	cutName, _, _ := strings.Cut(tag, ",")
	if hasTag && cutName == "-" && tag == "-" {
		return "", true
	}
	if cutName != "" {
		return cutName, false
	}
	if field.Anonymous {
		return "", false
	}
	return field.Name, false
}
