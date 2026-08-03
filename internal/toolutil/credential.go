package toolutil

import (
	"fmt"
	"reflect"
	"strings"
)

// credentialFieldMarkers are substrings that make an exported field name — a
// Go struct field, or an OpenAPI/JSON property name written in this API's own
// Go-struct-field style (see cmd/gen_action_inputs's assembleFields doc
// comment on that convention: "Name", "BaseURL", "TLSSkipVerify") — look like
// it might carry a credential. Deliberately narrow: "auth" would also match
// AuthorizedTeams/AuthorizedUsers/Authentication and produce noise, so it and
// similarly broad candidates (bearer, signature, pat) are left out pending an
// explicit ruling rather than added unilaterally.
var credentialFieldMarkers = []string{"password", "token", "secret", "key", "credential", "cert"}

// IsCredentialShapedName reports whether name contains one of
// credentialFieldMarkers, case-insensitively.
//
// This is the one derivation of what "credential-shaped" means, read by two
// consumers that must never be allowed to quietly disagree:
// WalkForCredentialShapedFields below, which walks a real, populated response
// value at test time (every domain whose response can carry a credential —
// registries today, and every domain cmd/gen_action_inputs's generator will
// in the future refuse to generate a bare handler for — runs this same
// check); and cmd/gen_action_inputs itself, which walks a response's
// *resolved OpenAPI schema* (property names only — generation time has no
// values to inspect) to decide whether an operation needs a domain-supplied
// redaction wrapper before it will emit a generated handler for it. Before
// this, the marker list lived only in internal/tools/registries's own test
// file, so nothing tied a future copy of it in the generator to this one; a
// P3 domain wave editing one without the other would not fail loudly, it
// would simply redact against a different definition than the one its own
// tests check against.
func IsCredentialShapedName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range credentialFieldMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// WalkForCredentialShapedFields walks v and calls report with the path of
// every populated field whose name matches IsCredentialShapedName, skipping
// any path present in skip.
//
// It descends through pointers, interfaces, maps, slices and arrays as well
// as structs — not structs alone. An earlier version of this walk (see
// internal/tools/registries's history, where it originated) stopped at
// pointer/struct, which silently missed a credential-named field reached
// through any of the other four kinds. PortainereeRegistry.RegistryAccesses
// is exactly that shape (map[string]PortainerRegistryAccessPolicies), so the
// gap was not hypothetical: it was already present in the first type this
// walk existed to guard.
//
// Moved here from internal/tools/registries_test.go so every domain whose
// response can carry a credential-shaped field runs the identical check,
// rather than each domain's own test reimplementing the walk with its own,
// subtly different blind spots. This is a move, not a rewrite: behaviour is
// unchanged from the version that lived in registries_test.go.
func WalkForCredentialShapedFields(v reflect.Value, path string, skip map[string]string, report func(path string)) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		WalkForCredentialShapedFields(v.Elem(), path, skip, report)
		return
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		WalkForCredentialShapedFields(v.Elem(), path, skip, report)
		return
	case reflect.Map:
		// RegistryAccesses is map-shaped, and a future credential field is as
		// likely to appear inside a map value as anywhere else. A walk that
		// stops at the map boundary would report coverage it does not have.
		for _, key := range v.MapKeys() {
			WalkForCredentialShapedFields(v.MapIndex(key), fmt.Sprintf("%s[%v]", path, key.Interface()), skip, report)
		}
		return
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			WalkForCredentialShapedFields(v.Index(i), fmt.Sprintf("%s[%d]", path, i), skip, report)
		}
		return
	case reflect.Struct:
	default:
		return
	}

	for i := range v.NumField() {
		field := v.Type().Field(i)
		if !field.IsExported() {
			continue
		}
		child := v.Field(i)
		fieldPath := path + "." + field.Name
		if _, allowed := skip[fieldPath]; !allowed {
			if IsCredentialShapedName(field.Name) && !child.IsZero() {
				report(fieldPath)
			}
		}
		WalkForCredentialShapedFields(child, fieldPath, skip, report)
	}
}
