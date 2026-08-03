package toolutil

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// credentialFieldMarkers are the words that make a field name — a Go struct
// field, or an OpenAPI/JSON property name written in this API's own
// Go-struct-field style (see cmd/gen_action_inputs's assembleFields doc
// comment on that convention: "Name", "BaseURL", "TLSSkipVerify") — look like
// it names credential material.
//
// These are matched as whole words, not as substrings; see
// IsCredentialShapedName. Deliberately narrow: "auth" would also match
// AuthorizedTeams/AuthorizedUsers/Authentication and produce noise, so it and
// similarly broad candidates (bearer, signature, pat) are left out pending an
// explicit ruling rather than added unilaterally.
//
// "jwt" is on the list even though "auth", "bearer" and "pat" are not, and the
// distinction is not arbitrary: the excluded three are near-misses on
// unrelated English words, whereas "jwt" is the literal property name
// Portainer's own auth domain returns a session token under. AuthenticateUser
// (POST /auth) and ValidateOAuth (POST /auth/oauth/validate) both answer
// {"jwt": "..."} on success. That is a bearer token handed to a model
// verbatim — the exact defect this file exists to make structurally
// impossible — and without the marker it was invisible to every check here.
//
// "certificate" is listed alongside "cert" because whole-word matching does
// not treat one as a prefix of the other: without it, clientCertificate and
// MTLSCertificate would stop being flagged.
var credentialFieldMarkers = []string{
	"password", "token", "secret", "key", "credential", "cert", "certificate", "jwt",
}

// FieldShape is the coarse shape of a field's value: enough to rule out
// something being credential material, not enough to identify what it is.
// Both consumers of this file can supply one — the generator from a resolved
// OpenAPI "type", the runtime walk from a reflect.Kind — which is what lets a
// single rule serve both.
type FieldShape int

const (
	// ShapeUnknown means no type information is available. It is never a
	// reason to relax: a field of unknown shape is treated exactly as its name
	// alone suggests.
	ShapeUnknown FieldShape = iota
	// ShapeText is a string: the only shape a secret is ever carried in
	// directly.
	ShapeText
	// ShapeNumeric is an integer or a floating-point number. A credential is
	// never one, whatever the field is called.
	ShapeNumeric
	// ShapeBoolean is a flag. A credential is never one either.
	ShapeBoolean
	// ShapeContainer is an object, array or map — a shape that cannot itself
	// be a secret but can hold one at any depth, so it stays flagged.
	ShapeContainer
)

// FieldShapeOfJSONType maps a resolved JSON Schema "type" to a FieldShape. An
// empty or unrecognised type yields ShapeUnknown, which flags on the name
// alone.
func FieldShapeOfJSONType(jsonType string) FieldShape {
	switch jsonType {
	case "string":
		return ShapeText
	case "integer", "number":
		return ShapeNumeric
	case "boolean":
		return ShapeBoolean
	case "object", "array":
		return ShapeContainer
	default:
		return ShapeUnknown
	}
}

// FieldShapeOfKind maps a reflect.Kind to a FieldShape, for the runtime walk.
// Pointers and interfaces are not dereferenced here — WalkForCredentialShapedFields
// already descends through them and reports on the value it reaches.
func FieldShapeOfKind(kind reflect.Kind) FieldShape {
	switch kind {
	case reflect.String:
		return ShapeText
	case reflect.Bool:
		return ShapeBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return ShapeNumeric
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return ShapeContainer
	default:
		return ShapeUnknown
	}
}

// splitFieldWords splits a field name into its constituent words, handling the
// three conventions the vendored specs actually mix: PascalCase
// ("AccessToken"), lowerCamelCase ("secretKeyRef"), and separator-delimited
// ("endpoint_id"). Consecutive capitals are treated as one word up to the
// last capital that begins a new one, so "rawAPIKey" splits as
// raw/API/Key and "TLSCACert" as TLSCA/Cert.
//
// This is deliberately its own splitter rather than a shared one. The
// project's other word splitters serve wire-format jobs where the output must
// match the vendored API byte for byte (see cmd/gen_action_inputs's splitWords
// and its actionInitialisms doc comment on exactly this hazard); this one only
// has to find word boundaries for marker matching, and coupling the two means
// neither can change without risking the other.
func splitFieldWords(name string) []string {
	var words []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = nil
		}
	}

	runes := []rune(name)
	for i, r := range runes {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
			continue
		case unicode.IsUpper(r):
			prevIsLower := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			prevIsUpper := i > 0 && unicode.IsUpper(runes[i-1])
			if prevIsLower || (prevIsUpper && nextIsLower) {
				flush()
			}
		}
		current = append(current, r)
	}
	flush()
	return words
}

// wordMatchesMarker reports whether one word of a field name is one of the
// marker words, tolerating a plural "s" ("Credentials" matches "credential",
// "Keys" matches "key"). Comparison is case-insensitive.
func wordMatchesMarker(word, marker string) bool {
	lower := strings.ToLower(word)
	return lower == marker || lower == marker+"s"
}

// IsCredentialShapedName reports whether name, on its own, looks like it names
// credential material. Prefer IsCredentialShapedField wherever the field's
// type is known: this is the name half of the rule, and the name half alone
// over-reports badly.
//
// This is the one derivation of what "credential-shaped" means, read by two
// consumers that must never be allowed to quietly disagree:
// WalkForCredentialShapedFields below, which walks a real, populated response
// value at test time (every domain whose response can carry a credential —
// registries today, and every domain cmd/gen_action_inputs's generator refuses
// to emit a bare handler for — runs this same check); and
// cmd/gen_action_inputs itself, which walks a response's *resolved OpenAPI
// schema* to decide whether an operation needs a domain-supplied redaction
// wrapper before it will emit a generated handler for it. Before this, the
// marker list lived only in internal/tools/registries's own test file, so
// nothing tied a future copy of it in the generator to this one; a P3 domain
// wave editing one without the other would not fail loudly, it would simply
// redact against a different definition than the one its own tests check
// against.
//
// # Why whole words, not substrings
//
// This matched raw substrings until a measurement against the vendored
// Business Edition specification showed the cost: 101 of 441 operations
// flagged, 392 (operation, field path) pairs, 77 distinct property names — of
// which only 28 names named credential material. Every flagged operation
// needs a hand-written redaction wrapper before this generator will emit a
// handler for it, so a false positive is not free: it is a wrapper somebody
// must write, review and keep correct for a field that was never a secret.
//
// Substring matching produced the whole "keywords" family: a Helm chart's
// keywords list contains "key" and nothing else. Whole-word matching with
// plural tolerance keeps Password, AccessToken, CloudApiKeys and
// AzureCredentials while dropping keywords, because "keywords" is one word
// that is neither "key" nor "keys".
//
// # Why a name rule alone is not enough
//
// Word boundaries do nothing for RestrictSecrets, AccessTokenExpiry or
// RequiredPasswordLength: "Secrets", "Token" and "Password" really are whole
// words in those names. What disqualifies them is their *shape*, which is why
// IsCredentialShapedField exists and why callers with type information should
// use it.
//
// The one name-level exception is a reference to a credential *record*:
// GitCredentialID, CredentialID and sharedCredentialId are foreign keys, and
// a name whose last word is "Id" cannot be the material it points at. This is
// scoped to the "credential" marker alone rather than generalised, because
// "accessKeyID" is a genuine credential name that also ends in "ID" — the
// suffix means "reference" next to "credential" and does not next to "key".
func IsCredentialShapedName(name string) bool {
	words := splitFieldWords(name)
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	lastIsID := strings.EqualFold(last, "id")
	// A field whose name ends in the word "Ref" is a reference object, not the
	// thing referred to. This is Kubernetes' own naming convention and the
	// referenced value is never inlined: secretRef and secretKeyRef resolve to
	// {name, namespace} and {name, key} respectively, and the five CSI
	// *SecretRef fields are the same LocalObjectReference shape. The secret
	// itself lives behind the reference, in another object this response does
	// not carry.
	if strings.EqualFold(last, "ref") {
		return false
	}

	for _, word := range words {
		for _, marker := range credentialFieldMarkers {
			if !wordMatchesMarker(word, marker) {
				continue
			}
			if marker == "credential" && lastIsID {
				// A *CredentialID is a foreign key to a credential record.
				// Keep looking: another word may still be a real marker.
				continue
			}
			return true
		}
	}
	return false
}

// IsCredentialShapedField is IsCredentialShapedName refined by the field's
// shape, and is what any caller holding type information should use.
//
// A credential is text. A boolean or a number can never be one, whatever it is
// called: RestrictSecrets is a Kubernetes policy toggle, AccessTokenExpiry and
// TokenIssueAt are unix timestamps, RequiredPasswordLength is a policy
// setting, secretsCount is a dashboard counter, IsCertificateAuthority and
// automountServiceAccountToken are flags. Measured against the vendored
// specification, every boolean and every numeric field the name rule flagged
// was a false positive, without a single exception — which is what makes this
// a shape rule rather than a list of names that will go stale as the spec
// grows.
//
// Containers stay flagged. An object or array cannot itself be a secret, but
// it can hold one at any depth, and flagging it is sometimes the only thing
// that catches a credential whose own leaf name is unremarkable
// (AzureCredentials is the real example).
func IsCredentialShapedField(name string, shape FieldShape) bool {
	switch shape {
	case ShapeBoolean, ShapeNumeric:
		return false
	case ShapeUnknown, ShapeText, ShapeContainer:
	}
	return IsCredentialShapedName(name)
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
			// Shape-aware, exactly as the generator's schema walk is: a bool
			// named RestrictSecrets or an int named AccessTokenExpiry is not a
			// credential, and reporting one here would demand a redaction that
			// removes real, non-secret data from a response.
			shape := FieldShapeOfKind(child.Kind())
			if child.Kind() == reflect.Pointer || child.Kind() == reflect.Interface {
				if !child.IsNil() {
					shape = FieldShapeOfKind(child.Elem().Kind())
				}
			}
			if IsCredentialShapedField(field.Name, shape) && !child.IsZero() {
				report(fieldPath)
			}
		}
		WalkForCredentialShapedFields(child, fieldPath, skip, report)
	}
}

// auditFillSentinel is the value PopulateForCredentialAudit writes into every
// string field. It is deliberately recognisable in a test failure: a
// generated redaction guard that reports a surviving field prints the path,
// and a human reading the output should be in no doubt that the value came
// from the audit rather than from a real fixture. It is not a credential and
// is deliberately named so as not to read like one.
const auditFillSentinel = "AUDIT-FILL-SENTINEL"

// maxPopulateDepth bounds PopulateForCredentialAudit's recursion. Some
// generated response types are recursive (the vendored spec's
// portaineree.EdgeConfig has a "prev" field of its own type, which
// oapi-codegen renders as a self-referential pointer), so an unbounded walk
// would never terminate. The bound only limits how deep the audit populates:
// a credential field nested more than this far below a response root would go
// unpopulated and therefore unchecked, which is why it is set far beyond any
// real nesting depth in the vendored client rather than tightly against it.
const maxPopulateDepth = 12

// PopulateForCredentialAudit fills every settable field reachable from v with
// a non-zero value: strings get auditFillSentinel, numbers 1, booleans
// true, pointers a freshly allocated target, maps and slices exactly one
// populated entry.
//
// It exists so a domain's generated redaction guard can prove its wrapper
// actually removes credential-shaped fields. WalkForCredentialShapedFields
// only reports fields that are *populated* — a deliberate choice, since a zero
// field carries no secret — which means a guard run against a zero-valued
// response would pass no matter what the wrapper did, or did not, do. That is
// the difference between a test that checks redaction and a test that checks
// nothing; this function is what closes it.
func PopulateForCredentialAudit(v reflect.Value) {
	populateForCredentialAudit(v, 0)
}

func populateForCredentialAudit(v reflect.Value, depth int) {
	if depth > maxPopulateDepth || !v.CanSet() {
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(auditFillSentinel)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Pointer:
		elem := reflect.New(v.Type().Elem())
		populateForCredentialAudit(elem.Elem(), depth+1)
		v.Set(elem)
	case reflect.Struct:
		for i := range v.NumField() {
			populateForCredentialAudit(v.Field(i), depth+1)
		}
	case reflect.Slice:
		elem := reflect.New(v.Type().Elem()).Elem()
		populateForCredentialAudit(elem, depth+1)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), elem))
	case reflect.Array:
		for i := range v.Len() {
			populateForCredentialAudit(v.Index(i), depth+1)
		}
	case reflect.Map:
		key := reflect.New(v.Type().Key()).Elem()
		populateForCredentialAudit(key, depth+1)
		elem := reflect.New(v.Type().Elem()).Elem()
		populateForCredentialAudit(elem, depth+1)
		m := reflect.MakeMap(v.Type())
		m.SetMapIndex(key, elem)
		v.Set(m)
	case reflect.Interface:
		// Nothing to allocate: an interface field has no concrete type to
		// populate, and guessing one would put a value in a response the real
		// client would never produce.
	default:
	}
}

// AssertRedacted walks redacted — the value a domain's redaction wrapper
// returned — and returns the path of every credential-shaped field that is
// still populated. An empty result means the wrapper did its job.
//
// This is the assertion half of the generated redaction guard, kept here
// rather than emitted into every domain so that all domains check the same
// thing. A wrapper that is a pass-through (`func redactX(v T) any { return v }`)
// satisfies cmd/gen_action_inputs's "a function by this name exists" check
// perfectly well; only running it against a populated value and inspecting
// what comes back can tell the two apart.
func AssertRedacted(redacted any, rootPath string) []string {
	var survived []string
	WalkForCredentialShapedFields(reflect.ValueOf(redacted), rootPath, nil, func(path string) {
		survived = append(survived, path)
	})
	return survived
}
