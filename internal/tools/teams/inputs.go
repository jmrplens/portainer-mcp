// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package teams

// teamCreateInput is the parameter shape for operation TeamCreate (POST /teams).
type teamCreateInput struct {
	// DenyPortainerAccess DenyPortainerAccess denies members of this team access to Portainer itself
	DenyPortainerAccess *bool `json:"denyPortainerAccess,omitempty" jsonschema:"DenyPortainerAccess denies members of this team access to Portainer itself" edition:"EE"`
	// Name Name
	Name string `json:"name" jsonschema:"Name"`
	// TeamLeaders TeamLeaders
	TeamLeaders []int `json:"teamLeaders,omitempty" jsonschema:"TeamLeaders"`
}

// teamDeleteInput is the parameter shape for operation TeamDelete (DELETE /teams/{id}).
type teamDeleteInput struct {
	// ID Team Id
	ID int `json:"id" jsonschema:"Team Id"`
}

func (teamDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// teamInspectInput is the parameter shape for operation TeamInspect (GET /teams/{id}).
type teamInspectInput struct {
	// ID Team identifier
	ID int `json:"id" jsonschema:"Team identifier"`
}

func (teamInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// teamListInput is the parameter shape for operation TeamList (GET /teams).
type teamListInput struct {
	// EnvironmentID Identifier of the environment(endpoint) that will be used to filter the authorized teams
	EnvironmentID *int `json:"environmentId,omitempty" jsonschema:"Identifier of the environment(endpoint) that will be used to filter the authorized teams"`
	// OnlyLedTeams Only list teams that the user is leader of
	OnlyLedTeams *bool `json:"onlyLedTeams,omitempty" jsonschema:"Only list teams that the user is leader of"`
}

// teamUpdateInput is the parameter shape for operation TeamUpdate (PUT /teams/{id}).
type teamUpdateInput struct {
	// DenyPortainerAccess DenyPortainerAccess denies members of this team access to Portainer itself
	DenyPortainerAccess *bool `json:"denyPortainerAccess,omitempty" jsonschema:"DenyPortainerAccess denies members of this team access to Portainer itself" edition:"EE"`
	// ID Team identifier
	ID int `json:"id" jsonschema:"Team identifier"`
	// Name Name
	Name *string `json:"name,omitempty" jsonschema:"Name"`
}

func (teamUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}
