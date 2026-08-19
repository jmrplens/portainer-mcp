// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package resource_controls

// resourceControlCreateInput is the parameter shape for operation ResourceControlCreate (POST /resource_controls).
type resourceControlCreateInput struct {
	// AdministratorsOnly Permit access to resource only to admins
	AdministratorsOnly *bool `json:"administratorsOnly,omitempty" jsonschema:"Permit access to resource only to admins"`
	// Public Permit access to the associated resource to any user
	Public     *bool  `json:"public,omitempty" jsonschema:"Permit access to the associated resource to any user"`
	ResourceID string `json:"resourceId"`
	// SubResourceIDs List of Docker resources that will inherit this access control
	SubResourceIDs []string `json:"subResourceIdS,omitempty" jsonschema:"List of Docker resources that will inherit this access control"`
	// Teams List of team identifiers with access to the associated resource
	Teams []int `json:"teams,omitempty" jsonschema:"List of team identifiers with access to the associated resource"`
	// Type Type of Resource. Valid values are: 1 - container, 2 - service
	// 3 - volume, 4 - network, 5 - secret, 6 - stack, 7 - config, 8 - custom template, 9 - azure-container-group
	Type int `json:"type" jsonschema:"Type of Resource. Valid values are: 1 - container, 2 - service\n3 - volume, 4 - network, 5 - secret, 6 - stack, 7 - config, 8 - custom template, 9 - azure-container-group"`
	// Users List of user identifiers with access to the associated resource
	Users []int `json:"users,omitempty" jsonschema:"List of user identifiers with access to the associated resource"`
}

func (resourceControlCreateInput) EnumParams() map[string][]any {
	return map[string][]any{
		"type": {1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
}

// resourceControlDeleteInput is the parameter shape for operation ResourceControlDelete (DELETE /resource_controls/{id}).
type resourceControlDeleteInput struct {
	// ID Resource control identifier
	ID int `json:"id" jsonschema:"Resource control identifier"`
}

func (resourceControlDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// resourceControlUpdateInput is the parameter shape for operation ResourceControlUpdate (PUT /resource_controls/{id}).
type resourceControlUpdateInput struct {
	// AdministratorsOnly Permit access to resource only to admins
	AdministratorsOnly *bool `json:"administratorsOnly,omitempty" jsonschema:"Permit access to resource only to admins"`
	// ID Resource control identifier
	ID int `json:"id" jsonschema:"Resource control identifier"`
	// Public Permit access to the associated resource to any user
	Public *bool `json:"public,omitempty" jsonschema:"Permit access to the associated resource to any user"`
	// Teams List of team identifiers with access to the associated resource
	Teams []int `json:"teams,omitempty" jsonschema:"List of team identifiers with access to the associated resource"`
	// Users List of user identifiers with access to the associated resource
	Users []int `json:"users,omitempty" jsonschema:"List of user identifiers with access to the associated resource"`
}

func (resourceControlUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}
