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
// spec is passed and returned by value, so a caller building a []ActionSpec
// literal can wrap each entry inline (WithNarrative(ActionSpec{...}, hook(id)))
// without needing an intermediate variable.
func WithNarrative(spec ActionSpec, n ActionNarrative) ActionSpec {
	if n.Title != "" {
		spec.Title = n.Title
	}
	if n.Description != "" {
		spec.Description = n.Description
	}
	spec.Usage = n.Usage
	spec.RelatedActions = n.RelatedActions
	spec.Aliases = n.Aliases
	spec.Tags = n.Tags
	spec.ParameterGuidance = n.ParameterGuidance
	return spec
}
