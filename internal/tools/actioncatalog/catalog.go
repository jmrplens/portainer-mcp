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
	renderedNames := make(map[string]string, len(specs))
	for _, spec := range specs {
		if err := spec.Validate(); err != nil {
			return nil, fmt.Errorf("actioncatalog: %w", err)
		}
		if _, dup := seen[spec.Name]; dup {
			return nil, fmt.Errorf("actioncatalog: duplicate action name %q", spec.Name)
		}
		seen[spec.Name] = struct{}{}

		// A valid action name may contain both "." and "_", and RenderToolName
		// maps "." to "_" — so two distinct, individually valid names can
		// render identically (e.g. "tags.list_all" and "tags_list.all" both
		// become "portainer_tags_list_all"). The individual surface's
		// mcp.AddTool upserts by name, so a rendering collision would silently
		// replace one action's tool with the other's with no error anywhere.
		// Checked unconditionally, like the duplicate-name check above, rather
		// than only for actions that survive edition/version filtering: a
		// colliding pair is a defect in the declared specs themselves,
		// independent of which server they end up being served against.
		rendered := RenderToolName(spec.Name)
		if other, dup := renderedNames[rendered]; dup {
			return nil, fmt.Errorf(
				"actioncatalog: %s and %s both render as tool name %q",
				other, spec.Name, rendered)
		}
		renderedNames[rendered] = spec.Name

		// The link the whole catalog rests on. oapi-codegen names every
		// generated method after the operationId, and Available reports an
		// unknown operation as unavailable — so a typo here would delete an
		// action with no error anywhere. Refuse to build instead.
		//
		// Both editions are resolved unconditionally, before either check
		// below runs, so that the declared-edition cross-check always sees
		// both results. Resolving lazily and `continue`-ing as soon as the
		// edition being built misses would skip that cross-check for
		// whichever edition happens not to be built — a mis-declared spec
		// would then build cleanly in one edition and only fail fatally in
		// the other, instead of being fatal in both as the contract requires.
		ceOp, inCE := apiversion.ByOperationID(edition.CE, spec.OperationID)
		eeOp, inEE := apiversion.ByOperationID(edition.EE, spec.OperationID)
		if !inCE && !inEE {
			return nil, fmt.Errorf(
				"actioncatalog: %s: OperationID %q resolves in neither edition; it is a typo or the spec no longer declares it",
				spec.Name, spec.OperationID)
		}

		// The declared Edition must agree with what the applicability index
		// says, or it is documentation that lies. For an edition-exclusive
		// operation the index already filters correctly, so a wrong field
		// would go unnoticed; for a shared operation a wrong field would gate
		// an action the data says is available. Neither is acceptable in a
		// declaration a reader trusts. This runs unconditionally too, for the
		// same reason as above.
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

		// Only now filter for the edition actually being built: an operation
		// may legitimately be absent from this edition's index while present
		// in the other, and that is a filter, not an error.
		var op apiversion.Operation
		var ok bool
		switch opts.Edition {
		case edition.CE:
			op, ok = ceOp, inCE
		case edition.EE:
			op, ok = eeOp, inEE
		}
		if !ok {
			continue
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

// RenderToolName renders an action's canonical name as an MCP tool name, by
// replacing every "." with "_" and prefixing "portainer_". For example
// "tags.list" becomes "portainer_tags_list".
//
// Exported so that every surface needing this rendering — today, only the
// individual surface, which registers one MCP tool per action — calls this
// one implementation. Build's collision check above runs against the exact
// same rendering, so the check and the rendering can never drift apart: a
// surface with its own copy could render names the check never saw.
func RenderToolName(actionName string) string {
	out := make([]rune, 0, len(actionName)+10)
	out = append(out, []rune("portainer_")...)
	for _, r := range actionName {
		if r == '.' {
			r = '_'
		}
		out = append(out, r)
	}
	return string(out)
}
