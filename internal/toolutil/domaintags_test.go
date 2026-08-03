package toolutil

import (
	"strings"
	"testing"
)

func TestValidateDomainTags_RealTable_Succeeds(t *testing.T) {
	t.Parallel()
	if err := ValidateDomainTags(DomainTags); err != nil {
		t.Fatalf("ValidateDomainTags(DomainTags) error = %v, want the real table to be internally consistent", err)
	}
}

// TestValidateDomainTags_TagClaimedByTwoDomains_ReturnsError is the mutation
// this guard exists for: two domains independently listing the same tag
// would make "which domain does this operation belong to" depend on map
// iteration order. Constructs its own table rather than mutating the package
// var, so this test cannot leak into any other test running in this package.
func TestValidateDomainTags_TagClaimedByTwoDomains_ReturnsError(t *testing.T) {
	t.Parallel()
	broken := map[string][]string{
		"backup": {"backup"},
		"cloud":  {"cloud_credentials"},
		"kaas":   {"cloud_credentials"}, // duplicate: cloud_credentials already belongs to "cloud"
	}
	err := ValidateDomainTags(broken)
	if err == nil {
		t.Fatal("ValidateDomainTags() = nil, want an error: cloud_credentials is claimed by both cloud and kaas")
	}
	if !strings.Contains(err.Error(), "cloud_credentials") {
		t.Errorf("error = %q, want it to name the doubly-claimed tag", err)
	}
}

// TestValidateDomainTags_DomainWithNoTags_ReturnsError guards a domain entry
// that lists zero tags, which cmd/gen_action_inputs would otherwise resolve
// to zero operations and quietly skip — a variant of the same silent-empty
// generation defect this whole table exists to prevent, just moved into the
// table itself instead of into a directory-name mismatch.
func TestValidateDomainTags_DomainWithNoTags_ReturnsError(t *testing.T) {
	t.Parallel()
	broken := map[string][]string{
		"empty": {},
	}
	err := ValidateDomainTags(broken)
	if err == nil {
		t.Fatal("ValidateDomainTags() = nil, want an error for a domain with no tags")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want it to name the empty domain", err)
	}
}

// TestDomainTags_CoversFortySixDomains pins the count the whole-branch
// review demanded this table be populated to: every one of the 46 tags the
// vendored spec declares, not only the three pilot domains. See
// cmd/gen_action_inputs's TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec
// for the cross-check against the real spec file; this test guards the
// count regressing silently if an entry is ever removed.
func TestDomainTags_CoversFortySixDomains(t *testing.T) {
	t.Parallel()
	if len(DomainTags) != 46 {
		t.Errorf("len(DomainTags) = %d, want 46: one entry per OpenAPI tag in the vendored spec", len(DomainTags))
	}
	tagCount := 0
	for _, tags := range DomainTags {
		tagCount += len(tags)
	}
	if tagCount != 46 {
		t.Errorf("DomainTags names %d tags in total, want 46 (one each, no domain merging more than one)", tagCount)
	}
}

// The three pilot domains' Domain fields (system.go, tags.go, registries.go)
// must remain keys here unchanged, or cmd/gen_action_inputs would stop
// finding their operations and regeneration would no longer be byte-identical.
func TestDomainTags_PilotDomainsUnchanged(t *testing.T) {
	t.Parallel()
	for _, domain := range []string{"system", "tags", "registries"} {
		tags, ok := DomainTags[domain]
		if !ok {
			t.Fatalf("DomainTags has no entry for pilot domain %q", domain)
		}
		if len(tags) != 1 || tags[0] != domain {
			t.Errorf("DomainTags[%q] = %v, want [%q]: the pilot domains rely on directory name == tag name", domain, tags, domain)
		}
	}
}
