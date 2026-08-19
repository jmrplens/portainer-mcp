// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package endpoint_groups

// endpointGroupAddEndpointInput is the parameter shape for operation EndpointGroupAddEndpoint (PUT /endpoint_groups/{id}/endpoints/{endpointId}).
type endpointGroupAddEndpointInput struct {
	// EndpointID Environment(Endpoint) identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment(Endpoint) identifier"`
	// ID EndpointGroup identifier
	ID int `json:"id" jsonschema:"EndpointGroup identifier"`
}

func (endpointGroupAddEndpointInput) MinimumParams() map[string]int {
	return map[string]int{
		"endpointId": 1,
		"id":         1,
	}
}

// endpointGroupCreateInput is the parameter shape for operation EndpointGroupCreate (POST /endpoint_groups).
type endpointGroupCreateInput struct {
	// AssociatedEndpoints List of environment(endpoint) identifiers that will be part of this group
	AssociatedEndpoints []int `json:"associatedEndpoints,omitempty" jsonschema:"List of environment(endpoint) identifiers that will be part of this group"`
	// Description Environment(Endpoint) group description
	Description *string `json:"description,omitempty" jsonschema:"Environment(Endpoint) group description"`
	// Name Environment(Endpoint) group name
	Name string `json:"name" jsonschema:"Environment(Endpoint) group name"`
	// TagIDs List of tag identifiers to which this environment(endpoint) group is associated
	TagIDs []int `json:"tagIdS,omitempty" jsonschema:"List of tag identifiers to which this environment(endpoint) group is associated"`
}

// endpointGroupDeleteInput is the parameter shape for operation EndpointGroupDelete (DELETE /endpoint_groups/{id}).
type endpointGroupDeleteInput struct {
	// ID EndpointGroup identifier
	ID int `json:"id" jsonschema:"EndpointGroup identifier"`
}

func (endpointGroupDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointGroupDeleteEndpointInput is the parameter shape for operation EndpointGroupDeleteEndpoint (DELETE /endpoint_groups/{id}/endpoints/{endpointId}).
type endpointGroupDeleteEndpointInput struct {
	// EndpointID Environment(Endpoint) identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment(Endpoint) identifier"`
	// ID EndpointGroup identifier
	ID int `json:"id" jsonschema:"EndpointGroup identifier"`
}

func (endpointGroupDeleteEndpointInput) MinimumParams() map[string]int {
	return map[string]int{
		"endpointId": 1,
		"id":         1,
	}
}

// endpointGroupInspectInput is the parameter shape for operation EndpointGroupInspect (GET /endpoint_groups/{id}).
//
// Hand-written, not scaffolded: cmd/gen_action_inputs keys every struct it
// emits off an operationId, and this route carries none in either vendored
// document — internal/specnaming's table is what names it EndpointGroupInspect
// (see handlers.go and this domain's package comment). The two parameters
// below are the two the documents do declare for GET /endpoint_groups/{id},
// with their own descriptions verbatim, so cmd/audit_spec_drift compares this
// shape against the specification exactly as it does the six generated ones.
type endpointGroupInspectInput struct {
	// ID Environment(Endpoint) group identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) group identifier"`
	// Size If true, include the number of environments and breakdown by type
	Size *bool `json:"size,omitempty" jsonschema:"If true, include the number of environments and breakdown by type"`
}

func (endpointGroupInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointGroupListInput is the parameter shape for operation EndpointGroupList (GET /endpoint_groups).
type endpointGroupListInput struct {
	// Size If true, each environment(endpoint) group will include the number of environments(endpoints) associated to it and breakdown by type
	Size *bool `json:"size,omitempty" jsonschema:"If true, each environment(endpoint) group will include the number of environments(endpoints) associated to it and breakdown by type"`
}

// endpointGroupUpdateInput is the parameter shape for operation EndpointGroupUpdate (PUT /endpoint_groups/{id}).
type endpointGroupUpdateInput struct {
	// AssociatedEndpoints List of environment(endpoint) identifiers that will be part of this group
	AssociatedEndpoints []int `json:"associatedEndpoints,omitempty" jsonschema:"List of environment(endpoint) identifiers that will be part of this group"`
	// Description Environment(Endpoint) group description
	Description *string `json:"description,omitempty" jsonschema:"Environment(Endpoint) group description"`
	// ID EndpointGroup identifier
	ID int `json:"id" jsonschema:"EndpointGroup identifier"`
	// Name Environment(Endpoint) group name
	Name *string `json:"name,omitempty" jsonschema:"Environment(Endpoint) group name"`
	// TagIDs List of tag identifiers associated to the environment(endpoint) group
	TagIDs             []int                                                      `json:"tagIdS,omitempty" jsonschema:"List of tag identifiers associated to the environment(endpoint) group"`
	TeamAccessPolicies map[string]endpointGroupUpdateInputTeamAccessPoliciesValue `json:"teamAccessPolicies,omitempty"`
	UserAccessPolicies map[string]endpointGroupUpdateInputUserAccessPoliciesValue `json:"userAccessPolicies,omitempty"`
}

func (endpointGroupUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointGroupUpdateInputTeamAccessPoliciesValue is a nested object property.
type endpointGroupUpdateInputTeamAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// endpointGroupUpdateInputUserAccessPoliciesValue is a nested object property.
type endpointGroupUpdateInputUserAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}
