// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package team_memberships

// teamMembershipCreateInput is the parameter shape for operation TeamMembershipCreate (POST /team_memberships).
type teamMembershipCreateInput struct {
	// Role Role for the user inside the team (1 for leader and 2 for regular member)
	Role int `json:"role" jsonschema:"Role for the user inside the team (1 for leader and 2 for regular member)"`
	// TeamID Team identifier
	TeamID int `json:"teamId" jsonschema:"Team identifier"`
	// UserID User identifier
	UserID int `json:"userId" jsonschema:"User identifier"`
}

func (teamMembershipCreateInput) EnumParams() map[string][]any {
	return map[string][]any{
		"role": {1, 2},
	}
}

// teamMembershipDeleteInput is the parameter shape for operation TeamMembershipDelete (DELETE /team_memberships/{id}).
type teamMembershipDeleteInput struct {
	// ID TeamMembership identifier
	ID int `json:"id" jsonschema:"TeamMembership identifier"`
}

func (teamMembershipDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// teamMembershipUpdateInput is the parameter shape for operation TeamMembershipUpdate (PUT /team_memberships/{id}).
type teamMembershipUpdateInput struct {
	// ID Team membership identifier
	ID int `json:"id" jsonschema:"Team membership identifier"`
	// Role Role for the user inside the team (1 for leader and 2 for regular member)
	Role int `json:"role" jsonschema:"Role for the user inside the team (1 for leader and 2 for regular member)"`
	// TeamID Team identifier
	TeamID int `json:"teamId" jsonschema:"Team identifier"`
	// UserID User identifier
	UserID int `json:"userId" jsonschema:"User identifier"`
}

func (teamMembershipUpdateInput) EnumParams() map[string][]any {
	return map[string][]any{
		"role": {1, 2},
	}
}

func (teamMembershipUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// teamMembershipsInput is the parameter shape for operation TeamMemberships (GET /teams/{id}/memberships).
type teamMembershipsInput struct {
	// ID Team Id
	ID int `json:"id" jsonschema:"Team Id"`
}

func (teamMembershipsInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}
