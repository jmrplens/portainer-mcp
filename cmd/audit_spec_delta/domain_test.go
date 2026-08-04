package main

import (
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func TestUnit_ReverseDomainTags_InvertsCleanTable(t *testing.T) {
	t.Parallel()
	reverse, err := reverseDomainTags(map[string][]string{
		"tags":  {"tags"},
		"cloud": {"cloud_credentials", "sshkeygen"},
	})
	if err != nil {
		t.Fatalf("reverseDomainTags() error = %v", err)
	}
	if reverse["tags"] != "tags" {
		t.Errorf(`reverse["tags"] = %q, want "tags"`, reverse["tags"])
	}
	if reverse["cloud_credentials"] != "cloud" || reverse["sshkeygen"] != "cloud" {
		t.Errorf("reverse = %v, want both cloud_credentials and sshkeygen mapped to cloud", reverse)
	}
}

// TestUnit_ReverseDomainTags_RejectsATagClaimedTwice proves the guard
// against an ambiguous table actually bites: two domains claiming the same
// tag must be a hard error, not resolved by map iteration order.
func TestUnit_ReverseDomainTags_RejectsATagClaimedTwice(t *testing.T) {
	t.Parallel()
	_, err := reverseDomainTags(map[string][]string{
		"a": {"shared"},
		"b": {"shared"},
	})
	if err == nil {
		t.Fatal("reverseDomainTags() error = nil, want an error for a tag claimed by two domains")
	}
}

func TestUnit_DomainForTag_ResolvesAMappedTagToItsDomain(t *testing.T) {
	t.Parallel()
	reverse := map[string]string{"cloud_credentials": "cloud"}
	if got := domainForTag(reverse, "cloud_credentials"); got != "cloud" {
		t.Errorf("domainForTag() = %q, want %q", got, "cloud")
	}
}

// TestUnit_DomainForTag_UnmappedTag_IsVisiblyMarked is the case a brand-new
// tag in a candidate spec produces: this must not be silently dropped, and
// must not read as an ordinary resolved domain (several real domains share a
// name with their tag, so an unprefixed raw tag would be indistinguishable
// from a resolved one).
func TestUnit_DomainForTag_UnmappedTag_IsVisiblyMarked(t *testing.T) {
	t.Parallel()
	reverse := map[string]string{"cloud_credentials": "cloud"}
	got := domainForTag(reverse, "brand_new_tag")
	if got == "brand_new_tag" {
		t.Errorf("domainForTag() = %q, want it prefixed to distinguish an unmapped tag from a resolved domain", got)
	}
	if got != unmappedTagPrefix+"brand_new_tag" {
		t.Errorf("domainForTag() = %q, want %q", got, unmappedTagPrefix+"brand_new_tag")
	}
}

// TestUnit_DomainTags_RealTableReversesWithoutError proves the real,
// production toolutil.DomainTags table this command actually uses passes
// its own consistency check — a regression here would break every real run
// of this command, not just a fixture.
func TestUnit_DomainTags_RealTableReversesWithoutError(t *testing.T) {
	t.Parallel()
	if _, err := reverseDomainTags(toolutil.DomainTags); err != nil {
		t.Fatalf("reverseDomainTags(toolutil.DomainTags) error = %v, want the real table to be internally consistent", err)
	}
}
