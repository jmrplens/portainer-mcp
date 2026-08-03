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
