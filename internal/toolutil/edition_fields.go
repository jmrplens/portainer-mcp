package toolutil

import (
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
// it can never be part of a JSON-reflected schema in the first place. Only
// "EE" (case-insensitively) is a meaningful tag value — Portainer has two
// editions, and Community is the default every untagged field already gets
// — so any other value is silently ignored rather than refused: nothing
// hand-writes this tag (see below), so a stray value would be this
// function's own bug to fix, not a domain author's typo to report.
//
// cmd/gen_action_inputs/fields.go is the only place this tag is ever
// written, mechanically: a field present in the Business Edition operation's
// resolved schema and absent from the Community one gets it, nested structs
// included (that file's own doc comment calls the divergence it measures
// "nested-inclusive" for exactly this reason). Nobody hand-writes it.
func FieldEditions(rt reflect.Type) map[string]edition.Edition {
	if rt == nil {
		return nil
	}
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil
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
			maps.Copy(editions, FieldEditions(field.Type))
			continue
		}
		if tag := strings.TrimSpace(field.Tag.Get("edition")); strings.EqualFold(tag, string(edition.EE)) {
			editions[jsonName] = edition.EE
		}
	}
	if len(editions) == 0 {
		return nil
	}
	return editions
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
