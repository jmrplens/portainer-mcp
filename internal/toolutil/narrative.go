package toolutil

// ActionNarrative carries the ActionSpec fields no OpenAPI specification
// determines: Usage, RelatedActions, Aliases and Tags (ParameterGuidance
// beyond FillScopeParameterGuidance's central defaults belongs here too),
// plus an optional Title/Description override for the rarer case where a
// domain author judges the specification's own summary/description is
// genuinely poor.
//
// This is what a domain's own hand-written narrative hook returns, keyed by
// OperationID — see cmd/gen_action_inputs's generated actions.gen.go, which
// calls that hook and feeds its return value straight into WithNarrative.
// The call happens at runtime, inside the compiled binary, not while
// cmd/gen_action_inputs itself runs: regenerating the mechanical fields
// therefore can never discard whatever the hook returns, because there is
// no generated literal carrying it to discard — the hook's own source is the
// only place that value lives.
type ActionNarrative struct {
	Title             string
	Description       string
	Usage             string
	RelatedActions    []string
	Aliases           []string
	Tags              []string
	ParameterGuidance map[string]ParameterGuidance
}

// WithNarrative returns spec with n's fields overlaid.
//
// A non-empty n.Title or n.Description fully *replaces* spec's own value —
// never concatenated, prefixed or suffixed — matching the convention
// cmd/gen_action_inputs's actionspec.go already documents for the mechanical
// side of this same merge. Usage, RelatedActions, Aliases, Tags and
// ParameterGuidance are taken from n outright: no specification determines
// them at all, so there is no mechanical value to preserve when n leaves
// them at their zero value.
//
// Setting n.Title or n.Description also marks spec.TitleOverridden /
// spec.DescriptionOverridden — see ActionSpec's own doc comment on those two
// fields for why: this is what lets cmd/audit_spec_drift tell a deliberate,
// permanent improvement on the specification's own wording apart from
// accidental drift, without an allow-list entry recording the same decision
// a second time. Previously the allow-list was the only mechanism available
// for this (toolutil.ActionSpec retained no trace of the override at all).
//
// Be precise about what this mechanism does and does not do: it
// *recognises* a deliberate divergence, it does not *resolve* one. All 35
// Title/Description divergences api/spec-drift-allowlist.yaml used to name
// still exist — every one of the 19 catalogued actions with a narrative
// override still says something different from the specification's own
// wording, on purpose — this field only changes how cmd/audit_spec_drift is
// told not to gate the build over them, from a YAML line naming the
// operation and field a second time to a fact recorded once, at the point
// the override is made. Emptying api/spec-drift-allowlist.yaml down to zero
// entries is real progress (one fewer place the same 35 decisions could
// silently drift out of sync with the narrative hook that actually made
// them), but it is not the same claim as "the divergence went away", and
// cmd/audit_spec_drift's own report renders every one of these findings
// tagged [OVERRIDDEN], not silently — see findingLine in that command's
// report.go — precisely so a reader of the report, not only of this
// comment, can see the distinction too.
//
// spec is passed and returned by value, so a caller building a []ActionSpec
// literal can wrap each entry inline (WithNarrative(ActionSpec{...}, hook(id)))
// without needing an intermediate variable.
func WithNarrative(spec ActionSpec, n ActionNarrative) ActionSpec {
	if n.Title != "" {
		spec.Title = n.Title
		spec.TitleOverridden = true
	}
	if n.Description != "" {
		spec.Description = n.Description
		spec.DescriptionOverridden = true
	}
	spec.Usage = n.Usage
	spec.RelatedActions = n.RelatedActions
	spec.Aliases = n.Aliases
	spec.Tags = n.Tags
	spec.ParameterGuidance = n.ParameterGuidance
	return spec
}
