package main

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// unmappedTagPrefix marks a domain label this tool had to fabricate because
// no entry in toolutil.DomainTags claims the operation's own OpenAPI tag —
// the case of a brand-new tag a candidate spec introduces before any domain
// package exists to own it. Prefixed rather than silently grouped under the
// raw tag, so a reader scanning the work list can tell "an unmapped tag" from
// "a domain named identically to its tag" at a glance; several domains in
// this project are named identically to their tag, so an unprefixed raw tag
// would be indistinguishable from a resolved one.
const unmappedTagPrefix = "(unmapped tag) "

// reverseDomainTags inverts toolutil.DomainTags into tag -> domain, the
// direction this tool actually needs: a spec operation carries a tag, not a
// domain, and grouping a work list by the OpenAPI tag directly rather than
// through this table is the exact defect this project's own domain.go
// warning already names — tag and domain differ for twelve of the vendored
// spec's tags today (see toolutil.DomainTags's own doc comment).
//
// It validates domainTags first with toolutil.ValidateDomainTags: a tag
// claimed by two domains would make "which domain owns this operation" a
// matter of map iteration order, and this tool must refuse to guess rather
// than silently pick one.
func reverseDomainTags(domainTags map[string][]string) (map[string]string, error) {
	if err := toolutil.ValidateDomainTags(domainTags); err != nil {
		return nil, fmt.Errorf("reverse domain tags: %w", err)
	}
	reverse := make(map[string]string, len(domainTags))
	for domain, tags := range domainTags {
		for _, tag := range tags {
			reverse[tag] = domain
		}
	}
	return reverse, nil
}

// domainForTag resolves tag to the domain that owns it, through reverse
// (built by reverseDomainTags), falling back to a clearly marked, still
// visible label rather than dropping the operation from the report: an
// operation belonging to a tag no domain package has claimed yet is exactly
// the case a newly-added operation in a candidate spec can produce, and this
// tool's job is to surface a work list, not to silently discard the part it
// cannot yet place.
func domainForTag(reverse map[string]string, tag string) string {
	if domain, ok := reverse[tag]; ok {
		return domain
	}
	if tag == "" {
		return unmappedTagPrefix + "(no tag)"
	}
	return unmappedTagPrefix + tag
}
