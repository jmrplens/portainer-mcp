package toolutil

import (
	"reflect"
	"testing"
)

// TestWalkForCredentialShapedFields_DescendsThroughMapsSlicesAndInterfaces
// proves the walk actually descends into maps, slices and interfaces, not
// merely that it does not crash on them.
//
// Moved from internal/tools/registries_test.go unchanged in behaviour: this
// uses synthetic local types, purpose-built to carry one credential-shaped
// field behind each shape, run through the exact WalkForCredentialShapedFields
// every domain (registries today, more once cmd/gen_action_inputs's generator
// starts requiring redaction wrappers elsewhere) now shares.
func TestWalkForCredentialShapedFields_DescendsThroughMapsSlicesAndInterfaces(t *testing.T) {
	t.Parallel()

	type viaMap struct{ Password *string }
	type viaSlice struct{ Password *string }
	type viaInterface struct{ Password *string }

	secret := "SENTINEL-SECRET"
	fixture := struct {
		ViaMap       map[string]viaMap
		ViaSlice     []viaSlice
		ViaInterface any
	}{
		ViaMap:       map[string]viaMap{"k": {Password: &secret}},
		ViaSlice:     []viaSlice{{Password: &secret}},
		ViaInterface: viaInterface{Password: &secret},
	}

	var flagged []string
	WalkForCredentialShapedFields(reflect.ValueOf(fixture), "Fixture", nil, func(path string) {
		flagged = append(flagged, path)
	})

	for _, want := range []string{
		"Fixture.ViaMap[k].Password",
		"Fixture.ViaSlice[0].Password",
		"Fixture.ViaInterface.Password",
	} {
		found := false
		for _, got := range flagged {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("walk did not flag %s; flagged = %v", want, flagged)
		}
	}
}

// TestIsCredentialShapedName_MatchesKnownMarkersCaseInsensitively pins the
// shared derivation both cmd/gen_action_inputs (static schema walk) and this
// package's own WalkForCredentialShapedFields (runtime reflective walk) call,
// so a change here is felt by both rather than by whichever one happens to
// keep its own copy in sync.
func TestIsCredentialShapedName_MatchesKnownMarkersCaseInsensitively(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Password", "AccessToken", "SecretAccessKey", "APICredential", "TLSCert", "password", "TOKEN"} {
		if !IsCredentialShapedName(name) {
			t.Errorf("IsCredentialShapedName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"Name", "URL", "Username", "AuthorizedUsers"} {
		if IsCredentialShapedName(name) {
			t.Errorf("IsCredentialShapedName(%q) = true, want false", name)
		}
	}
}

// TestUnit_CredentialFieldMarkers_IncludesJWT is deliberately an assertion
// about the marker list itself, not about some downstream effect of it.
// "jwt" is the property name Portainer's own auth domain returns a session
// token under — AuthenticateUser (POST /auth) and ValidateOAuth (POST
// /auth/oauth/validate) both answer {"jwt": "..."} on success — and none of
// the other markers matches it: not "token", not "key", not "credential". The
// auth domain has no package under internal/tools yet, so no integration test
// can cover this today; without a direct assertion the marker could be
// dropped and nothing would notice until a wave shipped a handler that hands
// a bearer token to a model verbatim.
func TestUnit_CredentialFieldMarkers_IncludesJWT(t *testing.T) {
	t.Parallel()
	var found bool
	for _, marker := range credentialFieldMarkers {
		if marker == "jwt" {
			found = true
		}
	}
	if !found {
		t.Errorf("credentialFieldMarkers = %v, want it to contain \"jwt\"", credentialFieldMarkers)
	}
	if !IsCredentialShapedName("jwt") {
		t.Error(`IsCredentialShapedName("jwt") = false, want true`)
	}
	if !IsCredentialShapedField("jwt", ShapeText) {
		t.Error(`IsCredentialShapedField("jwt", ShapeText) = false, want true`)
	}
}

// TestUnit_SplitFieldWords_HandlesTheConventionsTheSpecsMix pins the splitter
// the whole-word rule depends on. Getting "keywords" right is the entire
// reason the rule stopped matching substrings, and getting "rawAPIKey" right
// is what stops the plural/initialism handling from throwing away a real one.
func TestUnit_SplitFieldWords_HandlesTheConventionsTheSpecsMix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"Password", []string{"Password"}},
		{"AccessToken", []string{"Access", "Token"}},
		{"keywords", []string{"keywords"}},
		{"rawAPIKey", []string{"raw", "API", "Key"}},
		{"TLSCACert", []string{"TLSCA", "Cert"}},
		{"secretKeyRef", []string{"secret", "Key", "Ref"}},
		{"endpoint_id", []string{"endpoint", "id"}},
		{"GitCredentialID", []string{"Git", "Credential", "ID"}},
		{"", nil},
	} {
		got := splitFieldWords(tc.name)
		if len(got) != len(tc.want) {
			t.Errorf("splitFieldWords(%q) = %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitFieldWords(%q) = %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

// TestUnit_IsCredentialShapedField_KeepsRealCredentialsAndDropsMeasuredFalsePositives
// is the two-direction proof for the tightened rule. Both halves matter and
// neither is sufficient alone: a rule that flagged everything would pass the
// first half, and a rule that flagged nothing would pass the second.
//
// Every name in the second half was measured against the vendored Business
// Edition specification, with its real resolved type — these are not invented
// counterexamples. The old raw-substring rule flagged 101 of 441 operations,
// 392 (operation, field) pairs across 77 distinct property names; this rule
// flags 85 operations and 267 pairs across 54 names, while every genuine
// credential below stays flagged.
func TestUnit_IsCredentialShapedField_KeepsRealCredentialsAndDropsMeasuredFalsePositives(t *testing.T) {
	t.Parallel()

	t.Run("genuine credentials stay flagged", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name  string
			shape FieldShape
		}{
			{"Password", ShapeText},                  // 57 occurrences, the largest genuine group
			{"password", ShapeText},                  // same field, lower-cased elsewhere in the spec
			{"AccessToken", ShapeText},               // 16
			{"AuthenticationKey", ShapeText},         // 7
			{"EdgeKey", ShapeText},                   // 7
			{"AgentSecret", ShapeText},               // 3
			{"ClientSecret", ShapeText},              // 3
			{"CivoApiKey", ShapeText},                // 3
			{"DigitalOceanToken", ShapeText},         // 3
			{"secretAccessKey", ShapeText},           // AWS secret half
			{"accessKeyID", ShapeText},               // ends in ID, but "key" is the marker, not "credential"
			{"rawAPIKey", ShapeText},                 // the real secret on UserGenerateAPIKey
			{"licenseKey", ShapeText},                //
			{"clientCertificate", ShapeText},         // whole-word "certificate", not the "cert" prefix
			{"clientCertificatePassword", ShapeText}, //
			{"jwt", ShapeText},                       // the auth domain's session token
			{"AzureCredentials", ShapeContainer},     // plural, and a container that holds the material
			{"CloudApiKeys", ShapeContainer},         // plural
			{"RegistryCredentials", ShapeContainer},  //
			{"conflictingKeys", ShapeContainer},      // a list of real licence keys
			{"Secret", ShapeUnknown},                 // no type information: flag on the name alone
		} {
			if !IsCredentialShapedField(tc.name, tc.shape) {
				t.Errorf("IsCredentialShapedField(%q, %v) = false, want true: this names real credential material", tc.name, tc.shape)
			}
		}
	})

	t.Run("measured false positives are dropped", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name   string
			shape  FieldShape
			reason string
		}{
			{"keywords", ShapeContainer, "a Helm chart's keyword list; contains the substring \"key\" and nothing else"},
			{"GitCredentialID", ShapeNumeric, "foreign key to a credential record (35 occurrences, the single largest false positive)"},
			{"CredentialID", ShapeNumeric, "foreign key"},
			{"sharedCredentialId", ShapeNumeric, "foreign key"},
			{"AccessTokenExpiry", ShapeNumeric, "unix timestamp"},
			{"TokenIssueAt", ShapeNumeric, "unix timestamp"},
			{"RequiredPasswordLength", ShapeNumeric, "password policy setting, an integer count"},
			{"secretsCount", ShapeNumeric, "dashboard counter"},
			{"RestrictSecrets", ShapeBoolean, "Kubernetes policy toggle"},
			{"restrictSecrets", ShapeBoolean, "the same toggle, lower-cased"},
			{"IsCertificateAuthority", ShapeBoolean, "X.509 basicConstraints flag"},
			{"UseSeparateCert", ShapeBoolean, "configuration toggle"},
			{"automountServiceAccountToken", ShapeBoolean, "Kubernetes flag"},
			{"RequiresSetupToken", ShapeBoolean, "flag"},
			{"forceChangePassword", ShapeBoolean, "flag"},
			{"secretRef", ShapeContainer, "Kubernetes LocalObjectReference: {name}, never the secret"},
			{"secretKeyRef", ShapeContainer, "Kubernetes SecretKeySelector: {name, key}, never the value"},
			{"configMapKeyRef", ShapeContainer, "Kubernetes selector"},
			{"nodePublishSecretRef", ShapeContainer, "CSI SecretReference"},
			{"controllerExpandSecretRef", ShapeContainer, "CSI SecretReference"},
		} {
			if IsCredentialShapedField(tc.name, tc.shape) {
				t.Errorf("IsCredentialShapedField(%q, %v) = true, want false: %s", tc.name, tc.shape, tc.reason)
			}
		}
	})

	t.Run("shape alone never rescues an unrelated name", func(t *testing.T) {
		t.Parallel()
		// The shape rule must only ever subtract. A text field with no marker
		// word is still not a credential.
		for _, name := range []string{"Name", "URL", "Username", "AuthorizedUsers", "Description"} {
			if IsCredentialShapedField(name, ShapeText) {
				t.Errorf("IsCredentialShapedField(%q, ShapeText) = true, want false", name)
			}
		}
	})
}

// TestUnit_WalkForCredentialShapedFields_IsShapeAware proves the runtime walk
// applies the same shape rule the generator's schema walk does. If it did not,
// a domain's redaction wrapper would be required by its generated guard test
// to strip a boolean policy toggle out of a real response — removing correct,
// non-secret data to satisfy a check the generator never intended to make.
func TestUnit_WalkForCredentialShapedFields_IsShapeAware(t *testing.T) {
	t.Parallel()

	secret := "SENTINEL-SECRET"
	fixture := struct {
		Password               *string
		RestrictSecrets        bool
		AccessTokenExpiry      int
		RequiredPasswordLength int
	}{
		Password:               &secret,
		RestrictSecrets:        true,
		AccessTokenExpiry:      1735689600,
		RequiredPasswordLength: 12,
	}

	var flagged []string
	WalkForCredentialShapedFields(reflect.ValueOf(fixture), "Fixture", nil, func(path string) {
		flagged = append(flagged, path)
	})

	if len(flagged) != 1 || flagged[0] != "Fixture.Password" {
		t.Errorf("flagged = %v, want exactly [Fixture.Password]: the bool and the two ints are not credentials whatever they are called", flagged)
	}
}
