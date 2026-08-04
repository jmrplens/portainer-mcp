// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package registries

// ecrDeleteRepositoryInput is the parameter shape for operation EcrDeleteRepository (DELETE /registries/{id}/ecr/repositories/{repositoryName}).
type ecrDeleteRepositoryInput struct {
	// ID Registry identifier
	ID int `json:"id" jsonschema:"Registry identifier"`
	// RepositoryName Repository name
	RepositoryName string `json:"repositoryName" jsonschema:"Repository name"`
}

func (ecrDeleteRepositoryInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// ecrDeleteTagsInput is the parameter shape for operation EcrDeleteTags (DELETE /registries/{id}/ecr/repositories/{repositoryName}/tags).
type ecrDeleteTagsInput struct {
	// ID Registry identifier
	ID int `json:"id" jsonschema:"Registry identifier"`
	// RepositoryName Repository name
	RepositoryName int      `json:"repositoryName" jsonschema:"Repository name"`
	Tags           []string `json:"tags,omitempty"`
}

func (ecrDeleteTagsInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// registryConfigureInput is the parameter shape for operation RegistryConfigure (POST /registries/{id}/configure).
type registryConfigureInput struct {
	// Authentication Is authentication against this registry enabled
	Authentication bool `json:"authentication" jsonschema:"Is authentication against this registry enabled"`
	// ID Registry identifier
	ID int `json:"id" jsonschema:"Registry identifier"`
	// Password Password used to authenticate against this registry. required when Authentication is true
	Password *string `json:"password,omitempty" jsonschema:"Password used to authenticate against this registry. required when Authentication is true"`
	// Region ECR region
	Region *string `json:"region,omitempty" jsonschema:"ECR region"`
	// TLS Use TLS
	TLS *bool `json:"tls,omitempty" jsonschema:"Use TLS"`
	// TLSCACertFile The TLS CA certificate file
	TLSCACertFile []int `json:"tlsCACertFile,omitempty" jsonschema:"The TLS CA certificate file"`
	// TLSCertFile The TLS client certificate file
	TLSCertFile []int `json:"tlsCertFile,omitempty" jsonschema:"The TLS client certificate file"`
	// TLSKeyFile The TLS client key file
	TLSKeyFile []int `json:"tlsKeyFile,omitempty" jsonschema:"The TLS client key file"`
	// TLSSkipVerify Skip the verification of the server TLS certificate
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Skip the verification of the server TLS certificate"`
	// Username Username used to authenticate against this registry. Required when Authentication is true
	Username *string `json:"username,omitempty" jsonschema:"Username used to authenticate against this registry. Required when Authentication is true"`
}

func (registryConfigureInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// registryCreateInput is the parameter shape for operation RegistryCreate (POST /registries).
type registryCreateInput struct {
	// Authentication Is authentication against this registry enabled
	Authentication bool `json:"authentication" jsonschema:"Is authentication against this registry enabled"`
	// BaseURL BaseURL required for ProGet registry
	BaseURL *string `json:"baseUrl,omitempty" jsonschema:"BaseURL required for ProGet registry"`
	// Ecr ECR specific details, required when type = 7
	Ecr *registryCreateInputEcr `json:"ecr,omitempty" jsonschema:"ECR specific details, required when type = 7"`
	// Github Github specific details, required when type = 8
	//
	// Business Edition only: absent from the Community Edition operation's
	// resolved schema (checked directly against api/specs/ce-2.44.0.json — a
	// Community server has never heard of a GitHub registry provider), so it
	// carries the `edition:"EE"` tag by hand. registries was frozen before
	// P3.3 task 2 added edition-gating (cmd/gen_action_inputs/fields.go's
	// ceEEFieldDiff/applyFieldEditionGate), so the generator will never
	// revisit this file to add the tag mechanically — see this domain's own
	// P3.2 freeze note above.
	Github *registryCreateInputGithub `json:"github,omitempty" jsonschema:"Github specific details, required when type = 8" edition:"EE"`
	// Gitlab Gitlab specific details, required when type = 4
	Gitlab *registryCreateInputGitlab `json:"gitlab,omitempty" jsonschema:"Gitlab specific details, required when type = 4"`
	// Name Name that will be used to identify this registry
	Name string `json:"name" jsonschema:"Name that will be used to identify this registry"`
	// Password Password used to authenticate against this registry. required when Authentication is true
	Password *string `json:"password,omitempty" jsonschema:"Password used to authenticate against this registry. required when Authentication is true"`
	// Quay Quay specific details, required when type = 1
	Quay *registryCreateInputQuay `json:"quay,omitempty" jsonschema:"Quay specific details, required when type = 1"`
	// TLS Use TLS
	TLS *bool `json:"tls,omitempty" jsonschema:"Use TLS"`
	// Type Registry Type. Valid values are:
	// 	1 (Quay.io),
	// 	2 (Azure container registry),
	// 	3 (custom registry),
	// 	4 (Gitlab registry),
	// 	5 (ProGet registry),
	// 	6 (DockerHub)
	// 	7 (ECR)
	// 	8 (Github registry)
	Type int `json:"type" jsonschema:"Registry Type. Valid values are:\n\t1 (Quay.io),\n\t2 (Azure container registry),\n\t3 (custom registry),\n\t4 (Gitlab registry),\n\t5 (ProGet registry),\n\t6 (DockerHub)\n\t7 (ECR)\n\t8 (Github registry)"`
	// URL URL or IP address of the Docker registry
	URL string `json:"url" jsonschema:"URL or IP address of the Docker registry"`
	// Username Username used to authenticate against this registry. Required when Authentication is true
	Username *string `json:"username,omitempty" jsonschema:"Username used to authenticate against this registry. Required when Authentication is true"`
}

func (registryCreateInput) EnumParams() map[string][]any {
	return map[string][]any{
		"type": {1, 2, 3, 4, 5, 6, 7, 8},
	}
}

// registryCreateInputEcr: ECR specific details, required when type = 7
type registryCreateInputEcr struct {
	Region *string `json:"region,omitempty"`
}

// registryCreateInputGithub: Github specific details, required when type = 8
type registryCreateInputGithub struct {
	OrganisationName *string `json:"organisationName,omitempty"`
	UseOrganisation  *bool   `json:"useOrganisation,omitempty"`
}

// registryCreateInputGitlab: Gitlab specific details, required when type = 4
type registryCreateInputGitlab struct {
	InstanceURL *string `json:"instanceUrl,omitempty"`
	ProjectID   *int    `json:"projectId,omitempty"`
	ProjectPath *string `json:"projectPath,omitempty"`
}

// registryCreateInputQuay: Quay specific details, required when type = 1
type registryCreateInputQuay struct {
	OrganisationName *string `json:"organisationName,omitempty"`
	UseOrganisation  *bool   `json:"useOrganisation,omitempty"`
}

// registryDeleteInput is the parameter shape for operation RegistryDelete (DELETE /registries/{id}).
type registryDeleteInput struct {
	// ID Registry identifier
	ID int `json:"id" jsonschema:"Registry identifier"`
}

func (registryDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// registryInspectInput is the parameter shape for operation RegistryInspect (GET /registries/{id}).
type registryInspectInput struct {
	// EndpointID Environment identifier (applies policy overrides if provided)
	//
	// Business Edition only: absent from the Community Edition operation's
	// resolved schema (api/specs/ce-2.44.0.json's registryInspect declares no
	// endpointId query parameter at all) — see registryCreateInput's Github
	// field for the full reasoning on why this is hand-tagged rather than
	// generator-applied.
	EndpointID *int `json:"endpointId,omitempty" jsonschema:"Environment identifier (applies policy overrides if provided)" edition:"EE"`
	// ID Registry identifier
	ID int `json:"id" jsonschema:"Registry identifier"`
}

func (registryInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// registryPingInput is the parameter shape for operation RegistryPing (POST /registries/ping).
type registryPingInput struct {
	// Password Password used to authenticate against this registry
	Password *string `json:"password,omitempty" jsonschema:"Password used to authenticate against this registry"`
	// TLS Use TLS
	TLS *bool `json:"tls,omitempty" jsonschema:"Use TLS"`
	// Type Registry Type. Valid values are:
	// 	1 (Quay.io),
	// 	2 (Azure container registry),
	// 	3 (custom registry),
	// 	4 (Gitlab registry),
	// 	5 (ProGet registry),
	// 	6 (DockerHub)
	// 	7 (ECR)
	// 	8 (Github registry)
	Type int `json:"type" jsonschema:"Registry Type. Valid values are:\n\t1 (Quay.io),\n\t2 (Azure container registry),\n\t3 (custom registry),\n\t4 (Gitlab registry),\n\t5 (ProGet registry),\n\t6 (DockerHub)\n\t7 (ECR)\n\t8 (Github registry)"`
	// URL URL or IP address of the Docker registry
	URL string `json:"url" jsonschema:"URL or IP address of the Docker registry"`
	// Username Username used to authenticate against this registry
	Username *string `json:"username,omitempty" jsonschema:"Username used to authenticate against this registry"`
}

func (registryPingInput) EnumParams() map[string][]any {
	return map[string][]any{
		"type": {1, 2, 3, 4, 5, 6, 7, 8},
	}
}

// registryUpdateInput is the parameter shape for operation RegistryUpdate (PUT /registries/{id}).
type registryUpdateInput struct {
	Authentication bool                    `json:"authentication"`
	BaseURL        *string                 `json:"baseUrl,omitempty"`
	Ecr            *registryUpdateInputEcr `json:"ecr,omitempty"`
	// Github is Business Edition only: absent from the Community Edition
	// operation's resolved schema (api/specs/ce-2.44.0.json) — see
	// registryCreateInput's identical Github field for the full reasoning.
	Github *registryUpdateInputGithub `json:"github,omitempty" edition:"EE"`
	// ID Registry identifier
	ID               int                                                 `json:"id" jsonschema:"Registry identifier"`
	Name             string                                              `json:"name"`
	Password         *string                                             `json:"password,omitempty"`
	Quay             *registryUpdateInputQuay                            `json:"quay,omitempty"`
	RegistryAccesses map[string]registryUpdateInputRegistryAccessesValue `json:"registryAccesses,omitempty"`
	URL              string                                              `json:"url"`
	Username         *string                                             `json:"username,omitempty"`
}

func (registryUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// registryUpdateInputEcr is a nested object property.
type registryUpdateInputEcr struct {
	Region *string `json:"region,omitempty"`
}

// registryUpdateInputGithub is a nested object property.
type registryUpdateInputGithub struct {
	OrganisationName *string `json:"organisationName,omitempty"`
	UseOrganisation  *bool   `json:"useOrganisation,omitempty"`
}

// registryUpdateInputQuay is a nested object property.
type registryUpdateInputQuay struct {
	OrganisationName *string `json:"organisationName,omitempty"`
	UseOrganisation  *bool   `json:"useOrganisation,omitempty"`
}

// registryUpdateInputRegistryAccessesValueTeamAccessPoliciesValue is a nested object property.
type registryUpdateInputRegistryAccessesValueTeamAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// registryUpdateInputRegistryAccessesValueUserAccessPoliciesValue is a nested object property.
type registryUpdateInputRegistryAccessesValueUserAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// registryUpdateInputRegistryAccessesValue is a nested object property.
type registryUpdateInputRegistryAccessesValue struct {
	// Namespaces Kubernetes specific fields (with kubernetes, namespaces have access to a registry, if users/teams have access to the same namespace, they have access to the registry)
	Namespaces         []string                                                                   `json:"namespaces,omitempty" jsonschema:"Kubernetes specific fields (with kubernetes, namespaces have access to a registry, if users/teams have access to the same namespace, they have access to the registry)"`
	TeamAccessPolicies map[string]registryUpdateInputRegistryAccessesValueTeamAccessPoliciesValue `json:"teamAccessPolicies,omitempty"`
	// UserAccessPolicies Docker specific fields (with docker, users/teams have access to a registry)
	UserAccessPolicies map[string]registryUpdateInputRegistryAccessesValueUserAccessPoliciesValue `json:"userAccessPolicies,omitempty" jsonschema:"Docker specific fields (with docker, users/teams have access to a registry)"`
}

// repositoryTagsDeleteInput is the parameter shape for operation RepositoryTagsDelete (DELETE /registries/{id}/repositories/{repositoryName}/tags).
type repositoryTagsDeleteInput struct {
	// ID Registry identifier
	ID int `json:"id" jsonschema:"Registry identifier"`
	// RepositoryName Repository name
	RepositoryName string   `json:"repositoryName" jsonschema:"Repository name"`
	Tags           []string `json:"tags,omitempty"`
}

func (repositoryTagsDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}
