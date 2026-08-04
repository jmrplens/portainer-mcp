// Package actioncatalog turns declared action specs into the validated,
// filtered set of actions a particular server can serve.
//
// Filtering happens once, here, so that no surface has to remember to check
// edition, server version or read-only mode. A surface projects whatever the
// catalog contains.
package actioncatalog

import (
	"fmt"
	"maps"
	"reflect"
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
	// edition is the Options.Edition Build was called with — what
	// InputSchema prunes a shared action's parameters against. See that
	// method's own doc comment for why this is a Catalog concern and not
	// ActionSpec.InputSchema's.
	edition edition.Edition
}

// Edition returns the edition this catalog was built for.
func (c *Catalog) Edition() edition.Edition { return c.edition }

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
		edition:  opts.Edition,
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
// Every element is a deep copy: one catalog is shared by every surface, so
// handing out the internal slices and maps a spec now carries (Aliases, Tags,
// RelatedActions, ParameterGuidance) would let any consumer corrupt what the
// others see. A shallow top-level slice copy stopped being enough the moment
// ActionSpec grew composite fields — see cloneActionSpec.
func (c *Catalog) Actions() []toolutil.ActionSpec {
	return cloneActionSpecs(c.ordered)
}

// Domains returns the domain names present, sorted.
//
// A plain copy is enough here: unlike ActionSpec, a string cannot be mutated
// through a returned value, so there is nothing nested for a caller to reach
// into.
func (c *Catalog) Domains() []string {
	return append([]string(nil), c.domains...)
}

// ByDomain returns one domain's actions, sorted by name. Every element is a
// deep copy, for the same reason as Actions.
func (c *Catalog) ByDomain(domain string) []toolutil.ActionSpec {
	return cloneActionSpecs(c.byDomain[domain])
}

// Lookup finds an action by its canonical name. The returned spec is a deep
// copy, for the same reason as Actions: a caller that grew found.Aliases or
// found.ParameterGuidance in place must not reach the catalog's own copy.
func (c *Catalog) Lookup(name string) (toolutil.ActionSpec, bool) {
	spec, ok := c.byName[name]
	if !ok {
		return toolutil.ActionSpec{}, false
	}
	return cloneActionSpec(spec), true
}

// cloneActionSpec deep-copies the slice and map fields ActionSpec carries, so
// that mutating the result can never reach what the catalog stores internally.
//
// Handler and Input are deliberately left as-is: Handler is a function value
// with no mutable state of its own, and Input is read only through
// ActionSpec.InputSchema, which already returns its own defensive deep copy —
// cloning Input here would protect nothing an existing caller depends on.
func cloneActionSpec(spec toolutil.ActionSpec) toolutil.ActionSpec {
	spec.Aliases = append([]string(nil), spec.Aliases...)
	spec.Tags = append([]string(nil), spec.Tags...)
	spec.RelatedActions = append([]string(nil), spec.RelatedActions...)
	if spec.ParameterGuidance != nil {
		cloned := make(map[string]toolutil.ParameterGuidance, len(spec.ParameterGuidance))
		for name, guidance := range spec.ParameterGuidance {
			guidance.CommonConfusions = append([]string(nil), guidance.CommonConfusions...)
			cloned[name] = guidance
		}
		spec.ParameterGuidance = cloned
	}
	return spec
}

// cloneActionSpecs applies cloneActionSpec to every element of specs,
// returning a new slice: the slice itself must be independent too, or
// appending to one caller's result could reorder what another caller sees.
func cloneActionSpecs(specs []toolutil.ActionSpec) []toolutil.ActionSpec {
	out := make([]toolutil.ActionSpec, len(specs))
	for i, spec := range specs {
		out[i] = cloneActionSpec(spec)
	}
	return out
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

// InputSchema returns spec's parameter schema — the identical map
// spec.InputSchema itself would return — with every Business-Edition-only
// property removed from "properties" and "required" whenever this catalog
// was built for Community Edition. A field is Business-only when Input's
// type declares it with an `edition:"EE"` struct tag (see
// toolutil.FieldEditions); cmd/gen_action_inputs is the only place that tag
// is ever written, mechanically, for a field present in the Business
// Edition operation's resolved schema and absent from the Community one
// (nested structs included).
//
// ActionSpec.Edition already gates a whole action off a Community catalog;
// this is what gates the fields of an action that is CE-shared but whose
// input shape still isn't — 18 of the wave-1 domains' 53 CE-shared
// operations, carrying 65 EE-only parameters between them, all declared
// Edition: CE, at the time this method was added.
//
// # Why every model-facing surface must call this, not spec.InputSchema
//
// spec.InputSchema itself never prunes — it has no notion of which edition
// a catalog was built for, and it must not grow one, because
// cmd/audit_spec_drift depends on that: it never builds a Catalog at all
// (it reads wiring.AllSpecs() directly — see that command's own package
// doc), so its comparison against the vendored specification's *Business*
// Edition operation stays unpruned by construction, not by a special case
// carved out for it here. That is the seam this mechanism's brief asked for:
// pruning is a Catalog concern, layered on top of spec.InputSchema, and
// anything that never goes through a Catalog never sees it. Individual,
// meta and dynamic — the three surfaces that do build one — are what must
// call this method to publish a schema; calling spec.InputSchema directly
// from one of them would silently re-offer a Business-only parameter to a
// Community server, exactly the defect this whole mechanism exists to
// close.
//
// # No output-schema half
//
// gitlab-mcp-server's identical pruneSchemaFieldsByTier (that sibling's
// internal/tools/action_catalog.go:130-200) also prunes an *output* schema,
// leniently (an excluded field may still come back in a real response, so
// it sets additionalProperties: true rather than refusing it). ActionSpec
// publishes no output schema at all — every tool surface's result is
// whatever the handler returns, never validated against a published shape
// — so there is nothing here for a lenient half to re-admit. Ported as "no
// such thing to prune", not silently dropped.
func (c *Catalog) InputSchema(spec toolutil.ActionSpec) (map[string]any, error) {
	schema, err := spec.InputSchema()
	if err != nil {
		return nil, fmt.Errorf("catalog input schema for %s: %w", spec.Name, err)
	}
	if spec.Input == nil {
		return schema, nil
	}
	t := reflect.TypeOf(spec.Input)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return pruneInputSchemaByEdition(schema, toolutil.FieldEditions(t), c.edition), nil
}

// pruneInputSchemaByEdition removes from schema every top-level property
// named in fieldEditions whose required edition target does not include —
// mirroring gitlab-mcp-server's pruneSchemaProperties (internal/tools/
// action_catalog.go in that sibling), with the identical two properties its
// own doc comment calls out as worth copying exactly:
//
//   - schema — spec.InputSchema's own return value, already an independent
//     deep copy no cache anywhere else holds a reference to (see that
//     method's doc comment) — is returned completely unchanged, the same
//     map, not a clone, whenever nothing needs to be dropped: an action
//     with no `edition:"EE"` field (every action today, until wave 1 lands)
//     costs this call nothing beyond the map lookup that discovers there
//     is nothing to prune.
//   - When something must be dropped, only "properties" and "required" are
//     replaced; every other top-level key is carried over by the initial
//     maps.Copy, so a property this function does not touch is never
//     second-guessed.
func pruneInputSchemaByEdition(schema map[string]any, fieldEditions map[string]edition.Edition, target edition.Edition) map[string]any {
	if len(fieldEditions) == 0 {
		return schema
	}
	drop := make(map[string]bool, len(fieldEditions))
	for name, required := range fieldEditions {
		if !target.Includes(required) {
			drop[name] = true
		}
	}
	if len(drop) == 0 {
		return schema
	}

	cloned := make(map[string]any, len(schema))
	maps.Copy(cloned, schema)

	if props, ok := schema["properties"].(map[string]any); ok {
		newProps := make(map[string]any, len(props))
		for name, value := range props {
			if !drop[name] {
				newProps[name] = value
			}
		}
		cloned["properties"] = newProps
	}
	switch req := schema["required"].(type) {
	case []any:
		newReq := make([]any, 0, len(req))
		for _, item := range req {
			if name, _ := item.(string); !drop[name] {
				newReq = append(newReq, item)
			}
		}
		cloned["required"] = newReq
	case []string:
		newReq := make([]string, 0, len(req))
		for _, name := range req {
			if !drop[name] {
				newReq = append(newReq, name)
			}
		}
		cloned["required"] = newReq
	}
	return cloned
}
