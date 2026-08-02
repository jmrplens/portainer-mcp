// Package actioncatalog turns declared action specs into the validated,
// filtered set of actions a particular server can serve.
//
// Filtering happens once, here, so that no surface has to remember to check
// edition, server version or read-only mode. A surface projects whatever the
// catalog contains.
package actioncatalog

import (
	"fmt"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/apiversion"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Options describes the server the catalog is being built for.
type Options struct {
	// Edition is the resolved edition of the target server.
	Edition edition.Edition
	// ServerVersion is its reported version, such as "2.44.0".
	ServerVersion string
	// ReadOnly excludes every mutating action.
	ReadOnly bool
}

// Catalog is the validated, filtered set of actions for one server.
type Catalog struct {
	byName   map[string]toolutil.ActionSpec
	ordered  []toolutil.ActionSpec
	byDomain map[string][]toolutil.ActionSpec
	domains  []string
}

// Build validates every spec and keeps those the target server can serve.
//
// Validation is fatal: a spec that is malformed, duplicated, or whose
// OperationID does not resolve in the applicability table is a programming
// error, and failing the build is the only way to catch it. Filtering, by
// contrast, is expected: an action absent from this edition or version is
// simply not offered.
func Build(specs []toolutil.ActionSpec, opts Options) (*Catalog, error) {
	switch opts.Edition {
	case edition.CE, edition.EE:
	default:
		// Not merely "non-empty": an unknown edition makes every ByOperationID
		// lookup miss, which the filter branch would quietly read as "this
		// action belongs to the other edition", yielding a silently empty
		// catalog. Validation is fatal here for the same reason it is for specs.
		return nil, fmt.Errorf("actioncatalog: edition %q is not CE or EE", opts.Edition)
	}

	c := &Catalog{
		byName:   make(map[string]toolutil.ActionSpec, len(specs)),
		byDomain: map[string][]toolutil.ActionSpec{},
	}

	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if err := spec.Validate(); err != nil {
			return nil, fmt.Errorf("actioncatalog: %w", err)
		}
		if _, dup := seen[spec.Name]; dup {
			return nil, fmt.Errorf("actioncatalog: duplicate action name %q", spec.Name)
		}
		seen[spec.Name] = struct{}{}

		// The link the whole catalog rests on. oapi-codegen names every
		// generated method after the operationId, and Available reports an
		// unknown operation as unavailable — so a typo here would delete an
		// action with no error anywhere. Refuse to build instead.
		op, ok := apiversion.ByOperationID(opts.Edition, spec.OperationID)
		if !ok {
			// An operation may legitimately be absent from this edition's
			// index while present in the other; that is a filter, not an error.
			if _, other := apiversion.ByOperationID(otherEdition(opts.Edition), spec.OperationID); other {
				continue
			}
			return nil, fmt.Errorf(
				"actioncatalog: %s: OperationID %q resolves in neither edition; it is a typo or the spec no longer declares it",
				spec.Name, spec.OperationID)
		}

		// The declared Edition must agree with what the applicability index
		// says, or it is documentation that lies. For an edition-exclusive
		// operation the index already filters correctly, so a wrong field
		// would go unnoticed; for a shared operation a wrong field would gate
		// an action the data says is available. Neither is acceptable in a
		// declaration a reader trusts.
		_, inCE := apiversion.ByOperationID(edition.CE, spec.OperationID)
		_, inEE := apiversion.ByOperationID(edition.EE, spec.OperationID)
		switch {
		case spec.Edition == edition.CE && !inCE:
			return nil, fmt.Errorf(
				"actioncatalog: %s declares Edition CE but %q exists only in Business Edition",
				spec.Name, spec.OperationID)
		case spec.Edition == edition.EE && (!inEE || inCE):
			return nil, fmt.Errorf(
				"actioncatalog: %s declares Edition EE but %q is not exclusive to Business Edition",
				spec.Name, spec.OperationID)
		}

		// Provably redundant given the edition cross-check above, which forces
		// every valid spec's declared edition to agree with the applicability
		// index — so by this point Includes is always true. Kept as a backstop
		// in case that cross-check is ever relaxed, and because the cost is one
		// comparison. Deleting it breaks no test, which is the point: this is
		// belt and braces, not the mechanism.
		if !opts.Edition.Includes(spec.Edition) {
			continue
		}
		// Note this is a third, independent edition filter: Available is
		// edition-keyed because version spans are partitioned by edition, so it
		// re-excludes an operation absent from this edition even if the checks
		// above were bypassed. Worth knowing before mutating any one of them in
		// isolation and concluding the filtering is untested.
		if opts.ServerVersion != "" && !apiversion.Available(opts.Edition, op, opts.ServerVersion) {
			continue
		}
		if opts.ReadOnly && spec.Mutating {
			continue
		}

		c.byName[spec.Name] = spec
		c.ordered = append(c.ordered, spec)
		c.byDomain[spec.Domain] = append(c.byDomain[spec.Domain], spec)
	}

	sort.Slice(c.ordered, func(i, j int) bool { return c.ordered[i].Name < c.ordered[j].Name })
	for domain := range c.byDomain {
		actions := c.byDomain[domain]
		sort.Slice(actions, func(i, j int) bool { return actions[i].Name < actions[j].Name })
		c.domains = append(c.domains, domain)
	}
	sort.Strings(c.domains)

	return c, nil
}

func otherEdition(e edition.Edition) edition.Edition {
	if e == edition.CE {
		return edition.EE
	}
	return edition.CE
}

// Actions returns every action in the catalog, sorted by name.
//
// The slice is a copy: one catalog is shared by every surface, so handing out
// the internal one would let any consumer corrupt what the others see.
func (c *Catalog) Actions() []toolutil.ActionSpec {
	return append([]toolutil.ActionSpec(nil), c.ordered...)
}

// Domains returns the domain names present, sorted.
func (c *Catalog) Domains() []string {
	return append([]string(nil), c.domains...)
}

// ByDomain returns one domain's actions, sorted by name. The slice is a copy.
func (c *Catalog) ByDomain(domain string) []toolutil.ActionSpec {
	return append([]toolutil.ActionSpec(nil), c.byDomain[domain]...)
}

// Lookup finds an action by its canonical name.
func (c *Catalog) Lookup(name string) (toolutil.ActionSpec, bool) {
	spec, ok := c.byName[name]
	return spec, ok
}
