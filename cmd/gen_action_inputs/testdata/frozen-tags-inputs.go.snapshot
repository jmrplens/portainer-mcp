// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package tags

// tagCreateInput is the parameter shape for operation TagCreate (POST /tags).
type tagCreateInput struct {
	Name string `json:"name"`
}

// tagDeleteInput is the parameter shape for operation TagDelete (DELETE /tags/{id}).
type tagDeleteInput struct {
	// ID Tag identifier
	ID int `json:"id" jsonschema:"Tag identifier"`
}

func (tagDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}
