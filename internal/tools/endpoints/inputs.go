// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package endpoints

// endpointAssociationDeleteInput is the parameter shape for operation EndpointAssociationDelete (PUT /endpoints/{id}/association).
type endpointAssociationDeleteInput struct {
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
}

func (endpointAssociationDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointDeleteInput is the parameter shape for operation EndpointDelete (DELETE /endpoints/{id}).
type endpointDeleteInput struct {
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
}

func (endpointDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointDeleteBatchInput is the parameter shape for operation EndpointDeleteBatch (POST /endpoints/delete).
type endpointDeleteBatchInput struct {
	Endpoints []endpointDeleteBatchInputEndpointsItem `json:"endpoints,omitempty"`
}

// endpointDeleteBatchInputEndpointsItem is a nested object property.
type endpointDeleteBatchInputEndpointsItem struct {
	DeleteCluster *bool `json:"deleteCluster,omitempty"`
	ID            *int  `json:"id,omitempty"`
}

// endpointDockerhubStatusInput is the parameter shape for operation EndpointDockerhubStatus (GET /endpoints/{id}/dockerhub/{registryId}).
type endpointDockerhubStatusInput struct {
	// ID endpoint ID
	ID int `json:"id" jsonschema:"endpoint ID"`
	// RegistryID registry ID
	RegistryID int `json:"registryId" jsonschema:"registry ID"`
}

func (endpointDockerhubStatusInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointForceUpdateServiceInput is the parameter shape for operation EndpointForceUpdateService (PUT /endpoints/{id}/forceupdateservice).
type endpointForceUpdateServiceInput struct {
	// ID endpoint identifier
	ID int `json:"id" jsonschema:"endpoint identifier"`
	// PullImage PullImage if true will pull the image
	PullImage *bool `json:"pullImage,omitempty" jsonschema:"PullImage if true will pull the image"`
	// ServiceID ServiceId to update
	ServiceID *string `json:"serviceId,omitempty" jsonschema:"ServiceId to update"`
}

func (endpointForceUpdateServiceInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointInspectInput is the parameter shape for operation EndpointInspect (GET /endpoints/{id}).
type endpointInspectInput struct {
	// ExcludeSnapshot if true, the snapshot data won't be retrieved
	ExcludeSnapshot *bool `json:"excludeSnapshot,omitempty" jsonschema:"if true, the snapshot data won't be retrieved"`
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
}

func (endpointInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointListInput is the parameter shape for operation EndpointList (GET /endpoints).
type endpointListInput struct {
	// AgentVersions will return only environments with on of these agent versions
	AgentVersions []string `json:"agentVersions,omitempty" jsonschema:"will return only environments with on of these agent versions"`
	// EdgeAsync if exists true show only edge async agents, false show only standard edge agents. if missing, will show both types (relevant only for edge agents)
	EdgeAsync *bool `json:"edgeAsync,omitempty" jsonschema:"if exists true show only edge async agents, false show only standard edge agents. if missing, will show both types (relevant only for edge agents)"`
	// EdgeCheckInPassedSeconds if bigger then zero, show only edge agents that checked-in in the last provided seconds (relevant only for edge agents)
	//
	// Hand-written, and the reason endpointList has no generated handler:
	// the specification declares this query parameter {"type": "number"},
	// which cmd/gen_action_inputs renders *float64, while the generated
	// client narrows it to *float32. checkWireWidth refuses to bind a
	// parameter it cannot prove round-trips, so the narrowing is decided by
	// a person instead -- see endpointList in handlers.go.
	EdgeCheckInPassedSeconds *float64 `json:"edgeCheckInPassedSeconds,omitempty" jsonschema:"if bigger then zero, show only edge agents that checked-in in the last provided seconds (relevant only for edge agents)"`
	// EdgeDeviceUntrusted if true, show only untrusted edge agents, if false show only trusted edge agents (relevant only for edge agents)
	EdgeDeviceUntrusted *bool `json:"edgeDeviceUntrusted,omitempty" jsonschema:"if true, show only untrusted edge agents, if false show only trusted edge agents (relevant only for edge agents)"`
	// EdgeGroupIDs List environments(endpoints) of these edge groups
	EdgeGroupIDs []int `json:"edgeGroupIds,omitempty" jsonschema:"List environments(endpoints) of these edge groups"`
	// EdgeStackID will return the environments of the specified edge stack
	EdgeStackID *int `json:"edgeStackId,omitempty" jsonschema:"will return the environments of the specified edge stack"`
	// EdgeStackStatus only applied when edgeStackId exists. Filter the returned environments based on their deployment status in the stack (not the environment status!)
	EdgeStackStatus *int `json:"edgeStackStatus,omitempty" jsonschema:"only applied when edgeStackId exists. Filter the returned environments based on their deployment status in the stack (not the environment status!)"`
	// EndpointIDs will return only these environments(endpoints)
	EndpointIDs []int `json:"endpointIds,omitempty" jsonschema:"will return only these environments(endpoints)"`
	// ExcludeEdgeGroupIDs Exclude environments(endpoints) of these edge groups
	ExcludeEdgeGroupIDs []int `json:"excludeEdgeGroupIds,omitempty" jsonschema:"Exclude environments(endpoints) of these edge groups"`
	// ExcludeGroupIDs Exclude environments of these groups
	ExcludeGroupIDs []int `json:"excludeGroupIds,omitempty" jsonschema:"Exclude environments of these groups"`
	// ExcludeIDs will exclude these environments(endpoints)
	ExcludeIDs []int `json:"excludeIds,omitempty" jsonschema:"will exclude these environments(endpoints)"`
	// ExcludeSnapshots if true, the snapshot data won't be retrieved
	ExcludeSnapshots *bool `json:"excludeSnapshots,omitempty" jsonschema:"if true, the snapshot data won't be retrieved"`
	// GroupIDs List environments(endpoints) of these groups
	GroupIDs []int `json:"groupIds,omitempty" jsonschema:"List environments(endpoints) of these groups"`
	// K8sEnvAdmin If true, an `X-K8S-Env-Admin` header will be added to the response to indicate if the user is a K8S environment admin for any of the returned environments
	K8sEnvAdmin *bool "json:\"k8sEnvAdmin,omitempty\" jsonschema:\"If true, an `X-K8S-Env-Admin` header will be added to the response to indicate if the user is a K8S environment admin for any of the returned environments\" edition:\"EE\""
	// Limit Limit results to this value
	Limit *int `json:"limit,omitempty" jsonschema:"Limit results to this value"`
	// Name will return only environments(endpoints) with this name
	Name *string `json:"name,omitempty" jsonschema:"will return only environments(endpoints) with this name"`
	// NameFilter Filter environments by partial name match (case-insensitive, searches name only)
	NameFilter *string `json:"nameFilter,omitempty" jsonschema:"Filter environments by partial name match (case-insensitive, searches name only)" edition:"EE"`
	// Order Order sorted results by desc/asc
	Order *string `json:"order,omitempty" jsonschema:"Order sorted results by desc/asc"`
	// Outdated If true, return only environments with an outdated agent
	Outdated *bool `json:"outdated,omitempty" jsonschema:"If true, return only environments with an outdated agent"`
	// PlatformTypes Filter environments by platform type
	PlatformTypes []int `json:"platformTypes,omitempty" jsonschema:"Filter environments by platform type"`
	// Policy If true, will apply policy data to the returned environments(endpoints)
	Policy *bool `json:"policy,omitempty" jsonschema:"If true, will apply policy data to the returned environments(endpoints)" edition:"EE"`
	// PolicyIDs List environments(endpoints) associated with these policies
	PolicyIDs []int `json:"policyIds,omitempty" jsonschema:"List environments(endpoints) associated with these policies" edition:"EE"`
	// PolicyStatus Filter environments by policy status (applied, failed, in_progress, warning, not_supported). Only applies when policyIds is specified.
	PolicyStatus []string `json:"policyStatus,omitempty" jsonschema:"Filter environments by policy status (applied, failed, in_progress, warning, not_supported). Only applies when policyIds is specified." edition:"EE"`
	// Provisioned If true, will return environment(endpoint) that were provisioned
	Provisioned *bool `json:"provisioned,omitempty" jsonschema:"If true, will return environment(endpoint) that were provisioned" edition:"EE"`
	// Search Search query
	Search *string `json:"search,omitempty" jsonschema:"Search query"`
	// Sort Sort results by this value
	Sort *string `json:"sort,omitempty" jsonschema:"Sort results by this value"`
	// Start Start searching from
	Start *int `json:"start,omitempty" jsonschema:"Start searching from"`
	// Status List environments(endpoints) by this status
	Status []int `json:"status,omitempty" jsonschema:"List environments(endpoints) by this status"`
	// TagIDs search environments(endpoints) with these tags (depends on tagsPartialMatch)
	TagIDs []int `json:"tagIds,omitempty" jsonschema:"search environments(endpoints) with these tags (depends on tagsPartialMatch)"`
	// TagsPartialMatch If true, will return environment(endpoint) which has one of tagIds, if false (or missing) will return only environments(endpoints) that has all the tags
	TagsPartialMatch *bool `json:"tagsPartialMatch,omitempty" jsonschema:"If true, will return environment(endpoint) which has one of tagIds, if false (or missing) will return only environments(endpoints) that has all the tags"`
	// Types List environments(endpoints) of this type
	Types []int `json:"types,omitempty" jsonschema:"List environments(endpoints) of this type"`
	// UpdateInformation If true, an `X-Update-Available` header will be added to the response to indicate if an update is available
	UpdateInformation *bool "json:\"updateInformation,omitempty\" jsonschema:\"If true, an `X-Update-Available` header will be added to the response to indicate if an update is available\" edition:\"EE\""
}

func (endpointListInput) EnumParams() map[string][]any {
	return map[string][]any{
		"edgeStackStatus": {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13},
		"sort":            {"Name", "Group", "Status", "LastCheckIn", "EdgeID", "PlatformType", "Health", "Id"},
	}
}

// endpointMTLSAgentCertificateErrorInput is the parameter shape for operation EndpointMTLSAgentCertificateError (GET /endpoints/{id}/mtls_certificate_error).
type endpointMTLSAgentCertificateErrorInput struct {
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
}

func (endpointMTLSAgentCertificateErrorInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointMTLSCertificateInput is the parameter shape for operation EndpointMTLSCertificate (GET /endpoints/{id}/mtls_certificate).
type endpointMTLSCertificateInput struct {
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
}

func (endpointMTLSCertificateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointRegistriesListInput is the parameter shape for operation EndpointRegistriesList (GET /endpoints/{id}/registries).
type endpointRegistriesListInput struct {
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
	// Namespace required if kubernetes environment, will show registries by namespace
	Namespace *string `json:"namespace,omitempty" jsonschema:"required if kubernetes environment, will show registries by namespace"`
}

func (endpointRegistriesListInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointRegistryAccessInput is the parameter shape for operation EndpointRegistryAccess (PUT /endpoints/{id}/registries/{registryId}).
type endpointRegistryAccessInput struct {
	// ID Environment(Endpoint) identifier
	ID         int      `json:"id" jsonschema:"Environment(Endpoint) identifier"`
	Namespaces []string `json:"namespaces,omitempty"`
	// RegistryID Registry identifier
	RegistryID         int                                                           `json:"registryId" jsonschema:"Registry identifier"`
	TeamAccessPolicies map[string]endpointRegistryAccessInputTeamAccessPoliciesValue `json:"teamAccessPolicies,omitempty"`
	UserAccessPolicies map[string]endpointRegistryAccessInputUserAccessPoliciesValue `json:"userAccessPolicies,omitempty"`
}

func (endpointRegistryAccessInput) MinimumParams() map[string]int {
	return map[string]int{
		"id":         1,
		"registryId": 1,
	}
}

// endpointRegistryAccessInputTeamAccessPoliciesValue is a nested object property.
type endpointRegistryAccessInputTeamAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// endpointRegistryAccessInputUserAccessPoliciesValue is a nested object property.
type endpointRegistryAccessInputUserAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// endpointSetPolicyStatusesInput is the parameter shape for operation EndpointSetPolicyStatuses (PUT /endpoints/{id}/edge/policies/statuses).
type endpointSetPolicyStatusesInput struct {
	// ID Environment(Endpoint) identifier
	ID       int                                          `json:"id" jsonschema:"Environment(Endpoint) identifier"`
	Statuses []endpointSetPolicyStatusesInputStatusesItem `json:"statuses,omitempty"`
}

func (endpointSetPolicyStatusesInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointSetPolicyStatusesInputStatusesItem is a nested object property.
type endpointSetPolicyStatusesInputStatusesItem struct {
	Fingerprint *string `json:"fingerprint,omitempty"`
	Message     *string `json:"message,omitempty"`
	PolicyID    *int    `json:"policyId,omitempty"`
	// Status applying|applied|failed|removing
	Status *string `json:"status,omitempty" jsonschema:"applying|applied|failed|removing"`
	Type   *string `json:"type,omitempty"`
}

// endpointSettingsUpdateInput is the parameter shape for operation EndpointSettingsUpdate (PUT /endpoints/{id}/settings).
type endpointSettingsUpdateInput struct {
	// ChangeWindow Whether GitOps update time restrictions are enabled
	ChangeWindow *endpointSettingsUpdateInputChangeWindow `json:"changeWindow,omitempty" jsonschema:"Whether GitOps update time restrictions are enabled" edition:"EE"`
	// DeploymentOptions Hide manual deployment forms for an environment
	DeploymentOptions       *endpointSettingsUpdateInputDeploymentOptions `json:"deploymentOptions,omitempty" jsonschema:"Hide manual deployment forms for an environment" edition:"EE"`
	EnableGPUManagement     *bool                                         `json:"enableGPUManagement,omitempty"`
	EnableImageNotification *bool                                         `json:"enableImageNotification,omitempty" edition:"EE"`
	Gpus                    []endpointSettingsUpdateInputGpusItem         `json:"gpus,omitempty"`
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
	// SecuritySettings Security settings updates
	// SecuritySettings Security settings updates
	//
	// Deliberately NOT tagged edition:"EE", although only the Business
	// Edition specification declares it. Both editions accept these ten
	// settings; they disagree only on where in the body they go, and
	// endpointSettingsUpdate in handlers.go sends both spellings so one
	// published field works on either server. Tagging it EE would prune it
	// from a Community catalog and leave a Community user unable to change
	// any per-environment security setting at all. Measured 2026-08-18
	// against a live 2.44.0 of each edition; see that handler's own comment
	// and docs/api-divergences.md.
	SecuritySettings *endpointSettingsUpdateInputSecuritySettings `json:"securitySettings,omitempty" jsonschema:"Security settings updates"`
}

func (endpointSettingsUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointSettingsUpdateInputChangeWindow: Whether GitOps update time restrictions are enabled
type endpointSettingsUpdateInputChangeWindow struct {
	Enabled   bool   `json:"enabled" edition:"EE"`
	EndTime   string `json:"endTime" edition:"EE"`
	StartTime string `json:"startTime" edition:"EE"`
}

// endpointSettingsUpdateInputDeploymentOptions: Hide manual deployment forms for an environment
type endpointSettingsUpdateInputDeploymentOptions struct {
	// HideAddWithForm Hide manual deploy forms in portainer
	HideAddWithForm bool `json:"hideAddWithForm" jsonschema:"Hide manual deploy forms in portainer" edition:"EE"`
	// HideFileUpload Hide the file upload option in the remaining visible forms
	HideFileUpload bool `json:"hideFileUpload" jsonschema:"Hide the file upload option in the remaining visible forms" edition:"EE"`
	// HideWebEditor Hide the webeditor in the remaining visible forms
	HideWebEditor         bool `json:"hideWebEditor" jsonschema:"Hide the webeditor in the remaining visible forms" edition:"EE"`
	OverrideGlobalOptions bool `json:"overrideGlobalOptions" edition:"EE"`
}

// endpointSettingsUpdateInputGpusItem is a nested object property.
type endpointSettingsUpdateInputGpusItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// endpointSettingsUpdateInputSecuritySettings: Security settings updates
type endpointSettingsUpdateInputSecuritySettings struct {
	// AllowBindMountsForRegularUsers Whether non-administrator should be able to use bind mounts when creating containers
	AllowBindMountsForRegularUsers *bool `json:"allowBindMountsForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use bind mounts when creating containers" edition:"EE"`
	// AllowContainerCapabilitiesForRegularUsers Whether non-administrator should be able to use container capabilities
	AllowContainerCapabilitiesForRegularUsers *bool `json:"allowContainerCapabilitiesForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use container capabilities" edition:"EE"`
	// AllowDeviceMappingForRegularUsers Whether non-administrator should be able to use device mapping
	AllowDeviceMappingForRegularUsers *bool `json:"allowDeviceMappingForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use device mapping" edition:"EE"`
	// AllowHostNamespaceForRegularUsers Whether non-administrator should be able to use the host pid
	AllowHostNamespaceForRegularUsers *bool `json:"allowHostNamespaceForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use the host pid" edition:"EE"`
	// AllowPrivilegedModeForRegularUsers Whether non-administrator should be able to use privileged mode when creating containers
	AllowPrivilegedModeForRegularUsers *bool `json:"allowPrivilegedModeForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use privileged mode when creating containers" edition:"EE"`
	// AllowSecurityOptForRegularUsers Whether non-administrator should be able to use security-opt settings
	AllowSecurityOptForRegularUsers *bool `json:"allowSecurityOptForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use security-opt settings" edition:"EE"`
	// AllowStackManagementForRegularUsers Whether non-administrator should be able to manage stacks
	AllowStackManagementForRegularUsers *bool `json:"allowStackManagementForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to manage stacks" edition:"EE"`
	// AllowSysctlSettingForRegularUsers Whether non-administrator should be able to use sysctl settings
	AllowSysctlSettingForRegularUsers *bool `json:"allowSysctlSettingForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to use sysctl settings" edition:"EE"`
	// AllowVolumeBrowserForRegularUsers Whether non-administrator should be able to browse volumes
	AllowVolumeBrowserForRegularUsers *bool `json:"allowVolumeBrowserForRegularUsers,omitempty" jsonschema:"Whether non-administrator should be able to browse volumes" edition:"EE"`
	// EnableHostManagementFeatures Whether host management features are enabled
	EnableHostManagementFeatures *bool `json:"enableHostManagementFeatures,omitempty" jsonschema:"Whether host management features are enabled" edition:"EE"`
}

// endpointSnapshotInput is the parameter shape for operation EndpointSnapshot (POST /endpoints/{id}/snapshot).
type endpointSnapshotInput struct {
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
}

func (endpointSnapshotInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointUpdateInput is the parameter shape for operation EndpointUpdate (PUT /endpoints/{id}).
type endpointUpdateInput struct {
	// AzureApplicationID Azure application ID
	AzureApplicationID *string `json:"azureApplicationId,omitempty" jsonschema:"Azure application ID"`
	// AzureAuthenticationKey Azure authentication key
	AzureAuthenticationKey *string `json:"azureAuthenticationKey,omitempty" jsonschema:"Azure authentication key"`
	// AzureTenantID Azure tenant ID
	AzureTenantID *string `json:"azureTenantId,omitempty" jsonschema:"Azure tenant ID"`
	// ChangeWindow Whether GitOps update time restrictions are enabled
	ChangeWindow *endpointUpdateInputChangeWindow `json:"changeWindow,omitempty" jsonschema:"Whether GitOps update time restrictions are enabled" edition:"EE"`
	// DeploymentOptions Hide manual deployment forms for an environment
	DeploymentOptions *endpointUpdateInputDeploymentOptions `json:"deploymentOptions,omitempty" jsonschema:"Hide manual deployment forms for an environment" edition:"EE"`
	Edge              *endpointUpdateInputEdge              `json:"edge,omitempty" edition:"EE"`
	// EdgeCheckinInterval The check in interval for edge agent (in seconds)
	EdgeCheckinInterval *int `json:"edgeCheckinInterval,omitempty" jsonschema:"The check in interval for edge agent (in seconds)"`
	// Gpus GPUs information
	Gpus []endpointUpdateInputGpusItem `json:"gpus,omitempty" jsonschema:"GPUs information"`
	// GroupID Group identifier
	GroupID *int `json:"groupId,omitempty" jsonschema:"Group identifier"`
	// ID Environment(Endpoint) identifier
	ID                 int   `json:"id" jsonschema:"Environment(Endpoint) identifier"`
	IsSetStatusMessage *bool `json:"isSetStatusMessage,omitempty" edition:"EE"`
	// Kubernetes Associated Kubernetes data
	Kubernetes *endpointUpdateInputKubernetes `json:"kubernetes,omitempty" jsonschema:"Associated Kubernetes data"`
	// Name Name that will be used to identify this environment(endpoint)
	Name *string `json:"name,omitempty" jsonschema:"Name that will be used to identify this environment(endpoint)"`
	// PublicURL URL or IP address where exposed containers will be reachable.\
	// Defaults to URL if not specified
	PublicURL *string `json:"publicUrl,omitempty" jsonschema:"URL or IP address where exposed containers will be reachable.\\\nDefaults to URL if not specified"`
	// Status The status of the environment(endpoint) (1 - up, 2 - down)
	Status        *int                              `json:"status,omitempty" jsonschema:"The status of the environment(endpoint) (1 - up, 2 - down)"`
	StatusMessage *endpointUpdateInputStatusMessage `json:"statusMessage,omitempty" edition:"EE"`
	// TagIDs List of tag identifiers to which this environment(endpoint) is associated
	TagIDs             []int                                                 `json:"tagIdS,omitempty" jsonschema:"List of tag identifiers to which this environment(endpoint) is associated"`
	TeamAccessPolicies map[string]endpointUpdateInputTeamAccessPoliciesValue `json:"teamAccessPolicies,omitempty"`
	// TLS Require TLS to connect against this environment(endpoint)
	TLS *bool `json:"tls,omitempty" jsonschema:"Require TLS to connect against this environment(endpoint)"`
	// TLSSkipClientVerify Skip client verification when using TLS
	TLSSkipClientVerify *bool `json:"tlsSkipClientVerify,omitempty" jsonschema:"Skip client verification when using TLS"`
	// TLSSkipVerify Skip server verification when using TLS
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Skip server verification when using TLS"`
	// URL URL or IP address of a Docker host
	URL                *string                                               `json:"url,omitempty" jsonschema:"URL or IP address of a Docker host"`
	UserAccessPolicies map[string]endpointUpdateInputUserAccessPoliciesValue `json:"userAccessPolicies,omitempty"`
}

func (endpointUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// endpointUpdateInputChangeWindow: Whether GitOps update time restrictions are enabled
type endpointUpdateInputChangeWindow struct {
	Enabled   bool   `json:"enabled" edition:"EE"`
	EndTime   string `json:"endTime" edition:"EE"`
	StartTime string `json:"startTime" edition:"EE"`
}

// endpointUpdateInputDeploymentOptions: Hide manual deployment forms for an environment
type endpointUpdateInputDeploymentOptions struct {
	// HideAddWithForm Hide manual deploy forms in portainer
	HideAddWithForm bool `json:"hideAddWithForm" jsonschema:"Hide manual deploy forms in portainer" edition:"EE"`
	// HideFileUpload Hide the file upload option in the remaining visible forms
	HideFileUpload bool `json:"hideFileUpload" jsonschema:"Hide the file upload option in the remaining visible forms" edition:"EE"`
	// HideWebEditor Hide the webeditor in the remaining visible forms
	HideWebEditor         bool `json:"hideWebEditor" jsonschema:"Hide the webeditor in the remaining visible forms" edition:"EE"`
	OverrideGlobalOptions bool `json:"overrideGlobalOptions" edition:"EE"`
}

// endpointUpdateInputEdge is a nested object property.
type endpointUpdateInputEdge struct {
	// CommandInterval The command list interval for edge agent - used in edge async mode (in seconds)
	CommandInterval *int `json:"commandInterval,omitempty" jsonschema:"The command list interval for edge agent - used in edge async mode (in seconds)" edition:"EE"`
	// PingInterval The ping interval for edge agent - used in edge async mode (in seconds)
	PingInterval *int `json:"pingInterval,omitempty" jsonschema:"The ping interval for edge agent - used in edge async mode (in seconds)" edition:"EE"`
	// SnapshotInterval The snapshot interval for edge agent - used in edge async mode (in seconds)
	SnapshotInterval *int `json:"snapshotInterval,omitempty" jsonschema:"The snapshot interval for edge agent - used in edge async mode (in seconds)" edition:"EE"`
}

// endpointUpdateInputGpusItem is a nested object property.
type endpointUpdateInputGpusItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// endpointUpdateInputKubernetesConfigurationIngressClassesItem is a nested object property.
type endpointUpdateInputKubernetesConfigurationIngressClassesItem struct {
	Blocked           *bool    `json:"blocked,omitempty"`
	BlockedNamespaces []string `json:"blockedNamespaces,omitempty"`
	Name              string   `json:"name"`
	Type              string   `json:"type"`
}

// endpointUpdateInputKubernetesConfigurationStorageClassesItem is a nested object property.
type endpointUpdateInputKubernetesConfigurationStorageClassesItem struct {
	AccessModes          []string `json:"accessModes,omitempty"`
	AllowVolumeExpansion bool     `json:"allowVolumeExpansion"`
	Name                 string   `json:"name"`
	Provisioner          string   `json:"provisioner"`
}

// endpointUpdateInputKubernetesConfiguration is a nested object property.
type endpointUpdateInputKubernetesConfiguration struct {
	AllowNoneIngressClass           bool                                                           `json:"allowNoneIngressClass"`
	EnableNodeShell                 *bool                                                          `json:"enableNodeShell,omitempty" edition:"EE"`
	EnableResourceOverCommit        *bool                                                          `json:"enableResourceOverCommit,omitempty"`
	IngressAvailabilityPerNamespace bool                                                           `json:"ingressAvailabilityPerNamespace"`
	IngressClasses                  []endpointUpdateInputKubernetesConfigurationIngressClassesItem `json:"ingressClasses,omitempty"`
	ResourceOverCommitPercentage    *int                                                           `json:"resourceOverCommitPercentage,omitempty"`
	RestrictDefaultNamespace        *bool                                                          `json:"restrictDefaultNamespace,omitempty"`
	RestrictSecrets                 *bool                                                          `json:"restrictSecrets,omitempty" edition:"EE"`
	RestrictStandardUserIngressW    *bool                                                          `json:"restrictStandardUserIngressW,omitempty" edition:"EE"`
	StorageClasses                  []endpointUpdateInputKubernetesConfigurationStorageClassesItem `json:"storageClasses,omitempty"`
	UseLoadBalancer                 *bool                                                          `json:"useLoadBalancer,omitempty"`
	UseServerMetrics                *bool                                                          `json:"useServerMetrics,omitempty"`
}

// endpointUpdateInputKubernetesFlags is a nested object property.
type endpointUpdateInputKubernetesFlags struct {
	GPUOperator                  *bool `json:"gPUOperator,omitempty"`
	IsServerIngressClassDetected bool  `json:"isServerIngressClassDetected"`
	IsServerMetricsDetected      bool  `json:"isServerMetricsDetected"`
	IsServerStorageDetected      bool  `json:"isServerStorageDetected"`
}

// endpointUpdateInputKubernetesSnapshotsItemDiagnosticsData is a nested object property.
type endpointUpdateInputKubernetesSnapshotsItemDiagnosticsData struct {
	DNS    map[string]string `json:"dns,omitempty"`
	Log    *string           `json:"log,omitempty"`
	Proxy  map[string]string `json:"proxy,omitempty"`
	Telnet map[string]string `json:"telnet,omitempty"`
}

// endpointUpdateInputKubernetesSnapshotsItemPerformanceMetrics is a nested object property.
//
// Hand-written, and the reason endpointUpdate has no generated handler. The
// specification declares all four of these {"type": "number"}, rendered
// *float64, where the generated client's own PortainerPerformanceMetrics
// narrows every one to *float32; checkWireWidth refuses the whole operation
// over it rather than bind a parameter it cannot prove round-trips. The
// declaration here stays faithful to the specification -- so
// audit_spec_drift sees no drift and no allow-list entry is needed -- and
// endpointUpdate in handlers.go performs the narrowing explicitly, refusing
// a value that does not survive it.
type endpointUpdateInputKubernetesSnapshotsItemPerformanceMetrics struct {
	CPUUsage     *float64 `json:"cpuUsage,omitempty"`
	DiskUsage    *float64 `json:"diskUsage,omitempty"`
	MemoryUsage  *float64 `json:"memoryUsage,omitempty"`
	NetworkUsage *float64 `json:"networkUsage,omitempty"`
}

// endpointUpdateInputKubernetesSnapshotsItem is a nested object property.
type endpointUpdateInputKubernetesSnapshotsItem struct {
	ClusterType        *string                                                       `json:"clusterType,omitempty"`
	DiagnosticsData    *endpointUpdateInputKubernetesSnapshotsItemDiagnosticsData    `json:"diagnosticsData,omitempty"`
	GPUNodeCount       *int                                                          `json:"gPUNodeCount,omitempty"`
	KubernetesVersion  string                                                        `json:"kubernetesVersion"`
	NodeCount          int                                                           `json:"nodeCount"`
	PerformanceMetrics *endpointUpdateInputKubernetesSnapshotsItemPerformanceMetrics `json:"performanceMetrics,omitempty"`
	Time               int                                                           `json:"time"`
	TotalCPU           int                                                           `json:"totalCpu"`
	TotalGPU           map[string]int                                                `json:"totalGPU,omitempty"`
	TotalMemory        int                                                           `json:"totalMemory"`
}

// endpointUpdateInputKubernetes: Associated Kubernetes data
type endpointUpdateInputKubernetes struct {
	Configuration endpointUpdateInputKubernetesConfiguration   `json:"configuration"`
	Flags         endpointUpdateInputKubernetesFlags           `json:"flags"`
	Snapshots     []endpointUpdateInputKubernetesSnapshotsItem `json:"snapshots,omitempty"`
}

// endpointUpdateInputStatusMessage is a nested object property.
type endpointUpdateInputStatusMessage struct {
	Detail *string `json:"detail,omitempty" edition:"EE"`
	// Operation 'scale', 'upgrade', 'addons'
	Operation *string `json:"operation,omitempty" jsonschema:"'scale', 'upgrade', 'addons'" edition:"EE"`
	// OperationStatus '', 'error', 'processing'
	OperationStatus *string `json:"operationStatus,omitempty" jsonschema:"'', 'error', 'processing'" edition:"EE"`
	Summary         *string `json:"summary,omitempty" edition:"EE"`
}

func (endpointUpdateInputStatusMessage) EnumParams() map[string][]any {
	return map[string][]any{
		"operationStatus": {"processing", "warning", "error", ""},
	}
}

// endpointUpdateInputTeamAccessPoliciesValue is a nested object property.
type endpointUpdateInputTeamAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// endpointUpdateInputUserAccessPoliciesValue is a nested object property.
type endpointUpdateInputUserAccessPoliciesValue struct {
	// Namespaces Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles
	Namespaces []string `json:"namespaces,omitempty" jsonschema:"Namespaces is a list of namespaces that this access policy applies to. Only used for namespaced level roles"`
	// RoleID Role identifier. Reference the role that will be associated to this access policy
	RoleID int `json:"roleId" jsonschema:"Role identifier. Reference the role that will be associated to this access policy"`
}

// endpointUpdateRelationsInput is the parameter shape for operation EndpointUpdateRelations (PUT /endpoints/relations).
type endpointUpdateRelationsInput struct {
	Relations map[string]endpointUpdateRelationsInputRelationsValue `json:"relations,omitempty"`
}

// endpointUpdateRelationsInputRelationsValue is a nested object property.
type endpointUpdateRelationsInputRelationsValue struct {
	EdgeGroups []int `json:"edgeGroups,omitempty"`
	Group      *int  `json:"group,omitempty"`
	Tags       []int `json:"tags,omitempty"`
}

// namespacesAccessUpdateInput is the parameter shape for operation NamespacesAccessUpdate (PUT /endpoints/{id}/pools/{rpn}/access).
type namespacesAccessUpdateInput struct {
	// ID Environment identifier
	ID int `json:"id" jsonschema:"Environment identifier"`
	// Rpn Namespace identifier
	Rpn           int   `json:"rpn" jsonschema:"Namespace identifier"`
	TeamsToAdd    []int `json:"teamsToAdd,omitempty"`
	TeamsToRemove []int `json:"teamsToRemove,omitempty"`
	UsersToAdd    []int `json:"usersToAdd,omitempty"`
	UsersToRemove []int `json:"usersToRemove,omitempty"`
}

func (namespacesAccessUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// snapshotContainersListInput is the parameter shape for operation SnapshotContainersList (GET /docker/{environmentId}/snapshot/containers).
type snapshotContainersListInput struct {
	// EdgeStackID Edge stack identifier, will return only containers for this edge stack
	EdgeStackID *int `json:"edgeStackId,omitempty" jsonschema:"Edge stack identifier, will return only containers for this edge stack"`
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
}

func (snapshotContainersListInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}

// snapshotInspectInput is the parameter shape for operation SnapshotInspect (GET /docker/{environmentId}/snapshot).
type snapshotInspectInput struct {
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
}

func (snapshotInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}

// trustEdgeEndpointsInput is the parameter shape for operation TrustEdgeEndpoints (POST /endpoints/edge/trust).
type trustEdgeEndpointsInput struct {
	EndpointIDs []int                                            `json:"endpointIdS,omitempty"`
	Relations   map[string]trustEdgeEndpointsInputRelationsValue `json:"relations,omitempty"`
}

// trustEdgeEndpointsInputRelationsValue is a nested object property.
type trustEdgeEndpointsInputRelationsValue struct {
	EdgeGroups []int `json:"edgeGroups,omitempty"`
	Group      *int  `json:"group,omitempty"`
	Tags       []int `json:"tags,omitempty"`
}

// The three types below are hand-written, not scaffolded, for the reasons
// their handlers in handlers.go record: cmd/gen_action_inputs refused
// EndpointCreate and EndpointDockerBrowsePut because oapi-codegen emitted
// only a ...WithBodyWithResponse method for each multipart-only request
// body, and SnapshotContainerInspect because this project's own
// pathParamTypeOverrides types containerId string against a generated client
// that still takes an int. A refused operation gets no Input struct, so
// every field name, type and required-ness below is transcribed from
// api/specs/ee-2.44.0.json by hand, using the identical names
// cmd/gen_action_inputs's own splitWords/goFieldName/bodyJSONTag would have
// derived — that is what cmd/audit_spec_drift compares against, and a
// plausible-looking hand-chosen name would gate.
//
// Unexported and with the domain's ordinary "ID" capitalisation, like every
// other Input struct in this file: golangci-lint's revive var-naming rule
// applies to struct fields regardless of whether the struct is exported.

// endpointCreateInput is the parameter shape for operation EndpointCreate (POST /endpoints).
//
// The vendored required array is [Name, EndpointCreationType] and is
// published verbatim. Everything else the specification's own descriptions
// call "required if ..." is conditionally required on a value of
// EndpointCreationType, which toolutil.ActionSpec.ValidateInput cannot
// express and this action does not attempt to enforce locally — Portainer
// rejects the combination itself, with a message that names the field.
//
// The three format:binary fields carry file *content*, not a path: a model
// has no way to name a path on the filesystem this process runs on, so the
// PEM text itself crosses the tool boundary and handlers.go writes it as the
// multipart file part. Same treatment, and same reasoning, as
// custom_templates' File field.
type endpointCreateInput struct {
	// AzureApplicationID Azure application ID. Required if environment(endpoint) type is set to 3
	AzureApplicationID *string `json:"azureApplicationId,omitempty" jsonschema:"Azure application ID. Required if environment(endpoint) type is set to 3"`
	// AzureAuthenticationKey Azure authentication key. Required if environment(endpoint) type is set to 3
	AzureAuthenticationKey *string `json:"azureAuthenticationKey,omitempty" jsonschema:"Azure authentication key. Required if environment(endpoint) type is set to 3"`
	// AzureTenantID Azure tenant ID. Required if environment(endpoint) type is set to 3
	AzureTenantID *string `json:"azureTenantId,omitempty" jsonschema:"Azure tenant ID. Required if environment(endpoint) type is set to 3"`
	// ContainerEngine Container engine used by the environment(endpoint). Value must be one of: 'docker' or 'podman'
	ContainerEngine *string `json:"containerEngine,omitempty" jsonschema:"Container engine used by the environment(endpoint). Value must be one of: 'docker' or 'podman'"`
	// CustomTemplateContent KaaS: custom template content to deploy on environment creation
	CustomTemplateContent *string `json:"customTemplateContent,omitempty" jsonschema:"KaaS: custom template content to deploy on environment creation" edition:"EE"`
	// CustomTemplateID KaaS: custom template identifier to deploy on environment creation
	CustomTemplateID *int `json:"customTemplateId,omitempty" jsonschema:"KaaS: custom template identifier to deploy on environment creation" edition:"EE"`
	// EdgeAsyncMode Enable async mode for edge agent
	EdgeAsyncMode *bool `json:"edgeAsyncMode,omitempty" jsonschema:"Enable async mode for edge agent" edition:"EE"`
	// EdgeCheckinInterval The check in interval for edge agent (in seconds)
	EdgeCheckinInterval *int `json:"edgeCheckinInterval,omitempty" jsonschema:"The check in interval for edge agent (in seconds)"`
	// EdgeCommandInterval The command interval for edge agent (in seconds)
	EdgeCommandInterval *int `json:"edgeCommandInterval,omitempty" jsonschema:"The command interval for edge agent (in seconds)" edition:"EE"`
	// EdgePingInterval The ping interval for edge agent (in seconds)
	EdgePingInterval *int `json:"edgePingInterval,omitempty" jsonschema:"The ping interval for edge agent (in seconds)" edition:"EE"`
	// EdgeSnapshotInterval The snapshot interval for edge agent (in seconds)
	EdgeSnapshotInterval *int `json:"edgeSnapshotInterval,omitempty" jsonschema:"The snapshot interval for edge agent (in seconds)" edition:"EE"`
	// EdgeTunnelServerAddress URL or IP address that will be used to establish a reverse tunnel. Required when settings.EnableEdgeComputeFeatures is set to false or when settings.Edge.TunnelServerAddress is not set
	EdgeTunnelServerAddress *string `json:"edgeTunnelServerAddress,omitempty" jsonschema:"URL or IP address that will be used to establish a reverse tunnel. Required when settings.EnableEdgeComputeFeatures is set to false or when settings.Edge.TunnelServerAddress is not set"`
	// EndpointCreationType Environment(Endpoint) type. Value must be one of: 1 (Local Docker environment), 2 (Agent environment), 3 (Azure environment), 4 (Edge agent environment) or 5 (Local Kubernetes Environment)
	EndpointCreationType int `json:"endpointCreationType" jsonschema:"Environment(Endpoint) type. Value must be one of: 1 (Local Docker environment), 2 (Agent environment), 3 (Azure environment), 4 (Edge agent environment) or 5 (Local Kubernetes Environment)"`
	// Gpus List of GPUs - json stringified array of {name, value} structs
	//
	// A string, not an array: this route's multipart schema types Gpus
	// "string" holding a JSON-encoded array, where endpoints.update's JSON
	// body takes a real array of objects. The caller supplies the encoded
	// document; marshalling a Go slice here would produce a part Portainer
	// then fails to unmarshal.
	Gpus *string `json:"gpus,omitempty" jsonschema:"List of GPUs - json stringified array of {name, value} structs"`
	// GroupID Environment(Endpoint) group identifier. If not specified will default to 1 (unassigned).
	GroupID *int `json:"groupId,omitempty" jsonschema:"Environment(Endpoint) group identifier. If not specified will default to 1 (unassigned)."`
	// KubeConfig Kubernetes configuration file content (base64 encoded). Required when EndpointCreationType is set to 6 (KubeConfigEnvironment)
	KubeConfig *string `json:"kubeConfig,omitempty" jsonschema:"Kubernetes configuration file content (base64 encoded). Required when EndpointCreationType is set to 6 (KubeConfigEnvironment)" edition:"EE"`
	// Name Name that will be used to identify this environment(endpoint) (example: my-environment)
	Name string `json:"name" jsonschema:"Name that will be used to identify this environment(endpoint) (example: my-environment)"`
	// PublicURL URL or IP address where exposed containers will be reachable. Defaults to URL if not specified (example: docker.mydomain.tld:2375)
	PublicURL *string `json:"publicUrl,omitempty" jsonschema:"URL or IP address where exposed containers will be reachable. Defaults to URL if not specified (example: docker.mydomain.tld:2375)"`
	// StackName KaaS: stack name for the custom template deployment
	StackName *string `json:"stackName,omitempty" jsonschema:"KaaS: stack name for the custom template deployment" edition:"EE"`
	// TLS Require TLS to connect against this environment(endpoint). Must be true if EndpointCreationType is set to 2 (Agent environment)
	TLS *bool `json:"tls,omitempty" jsonschema:"Require TLS to connect against this environment(endpoint). Must be true if EndpointCreationType is set to 2 (Agent environment)"`
	// TLSCACertFile TLS CA certificate file
	TLSCACertFile *string `json:"tlsCACertFile,omitempty" jsonschema:"Content of the TLS CA certificate, in PEM form"`
	// TLSCertFile TLS client certificate file
	TLSCertFile *string `json:"tlsCertFile,omitempty" jsonschema:"Content of the TLS client certificate, in PEM form"`
	// TLSKeyFile TLS client key file
	TLSKeyFile *string `json:"tlsKeyFile,omitempty" jsonschema:"Content of the TLS client key, in PEM form"`
	// TLSSkipClientVerify Skip client verification when using TLS. Must be true if EndpointCreationType is set to 2 (Agent environment)
	TLSSkipClientVerify *bool `json:"tlsSkipClientVerify,omitempty" jsonschema:"Skip client verification when using TLS. Must be true if EndpointCreationType is set to 2 (Agent environment)"`
	// TLSSkipVerify Skip server verification when using TLS. Must be true if EndpointCreationType is set to 2 (Agent environment)
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Skip server verification when using TLS. Must be true if EndpointCreationType is set to 2 (Agent environment)"`
	// TagIDs JSON-parsable array of tag identifiers to which this environment(endpoint) is associated
	//
	// A string for the same reason Gpus above is: the multipart schema types
	// it "string" holding a JSON array, where endpoints.update takes []int.
	TagIDs *string `json:"tagIds,omitempty" jsonschema:"JSON-parsable array of tag identifiers to which this environment(endpoint) is associated"`
	// URL URL or IP address of a Docker host (example: docker.mydomain.tld:2375). Defaults to local if not specified (Linux: /var/run/docker.sock, Windows: //./pipe/docker_engine). Cannot be empty if EndpointCreationType is set to 4 (Edge agent environment)
	URL *string `json:"url,omitempty" jsonschema:"URL or IP address of a Docker host (example: docker.mydomain.tld:2375). Defaults to local if not specified (Linux: /var/run/docker.sock, Windows: //./pipe/docker_engine). Cannot be empty if EndpointCreationType is set to 4 (Edge agent environment)"`
}

func (endpointCreateInput) EnumParams() map[string][]any {
	return map[string][]any{
		"endpointCreationType": {0, 1, 2, 3, 4, 5, 6},
	}
}

// endpointDockerBrowsePutInput is the parameter shape for operation EndpointDockerBrowsePut (POST /endpoints/{id}/docker/v2/browse/put).
type endpointDockerBrowsePutInput struct {
	// File The file to upload
	//
	// The uploaded file's content, not a path, for the reason
	// endpointCreateInput's TLS fields record. Text only in consequence: a
	// payload that is not valid UTF-8 cannot be expressed as a JSON string
	// and so cannot be uploaded through this action.
	File string `json:"file" jsonschema:"Content of the file to upload"`
	// ID Environment(Endpoint) identifier
	ID int `json:"id" jsonschema:"Environment(Endpoint) identifier"`
	// Path The destination path to upload the file to
	Path string `json:"path" jsonschema:"The destination path to upload the file to"`
	// VolumeID Optional volume identifier to upload the file
	//
	// "volumeID", not the "volumeId" a body property of this name would take:
	// it is a query parameter, and cmd/gen_action_inputs publishes a query
	// parameter under the specification's own spelling rather than
	// re-deriving one. Compare endpointListInput, every field of which is a
	// query parameter carrying its spec name verbatim.
	VolumeID *string `json:"volumeID,omitempty" jsonschema:"Optional volume identifier to upload the file"`
}

func (endpointDockerBrowsePutInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// snapshotContainerInspectInput is the parameter shape for operation SnapshotContainerInspect (GET /docker/{environmentId}/snapshot/containers/{containerId}).
//
// containerId is a string here and an integer in the vendored specification.
// That is this project's own correction, not a transcription slip: Portainer
// reads the segment as an opaque Docker container ID (64 hex characters) and
// passes it through, so the document is wrong about its own type. The same
// correction is declared in cmd/gen_action_inputs's pathParamTypeOverrides
// and is what makes the generator refuse this operation — it will not bind a
// string Input field to the generated client's int argument. It carries the
// domain's single api/spec-drift-allowlist.yaml entry; see
// docs/api-divergences.md §6.3.
//
// It correspondingly takes no "minimum": 1 bound, and needs no exception
// entry to avoid one: isIdentifierPathParam's rule only bounds integers.
type snapshotContainerInspectInput struct {
	// ContainerID Container identifier
	ContainerID string `json:"containerId" jsonschema:"Container identifier"`
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
}

func (snapshotContainerInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}
