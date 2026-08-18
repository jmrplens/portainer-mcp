// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package stacks

// edgeStackWebhookInvokeInput is the parameter shape for operation EdgeStackWebhookInvoke (POST /edge_stacks/webhooks/{webhookID}).
type edgeStackWebhookInvokeInput struct {
	// WebhookID Stack identifier
	WebhookID string `json:"webhookID" jsonschema:"Stack identifier"`
}

// stackAssociateInput is the parameter shape for operation StackAssociate (PUT /stacks/{id}/associate).
type stackAssociateInput struct {
	// EndpointID Environment identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment identifier"`
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
	// OrphanedRunning Indicates whether the stack is orphaned
	OrphanedRunning bool `json:"orphanedRunning" jsonschema:"Indicates whether the stack is orphaned"`
	// SwarmID Swarm identifier
	SwarmID int `json:"swarmId" jsonschema:"Swarm identifier"`
}

func (stackAssociateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackConvertInput is the parameter shape for operation StackConvert (POST /stacks/{id}/convert).
type stackConvertInput struct {
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
	// Namespace Namespace for the Kubernetes stack
	Namespace *string `json:"namespace,omitempty" jsonschema:"Namespace for the Kubernetes stack"`
	// TargetFormat Target format: "kubernetes" for manifests or "helm" for helm chart
	TargetFormat string `json:"targetFormat" jsonschema:"Target format: \"kubernetes\" for manifests or \"helm\" for helm chart"`
}

func (stackConvertInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackCreateDockerStandaloneRepositoryInput is the parameter shape for operation StackCreateDockerStandaloneRepository (POST /stacks/create/standalone/repository).
type stackCreateDockerStandaloneRepositoryInput struct {
	// AdditionalFiles Applicable when deploying with multiple stack files
	AdditionalFiles []string `json:"additionalFiles,omitempty" jsonschema:"Applicable when deploying with multiple stack files"`
	// AutoUpdate Optional GitOps update configuration
	AutoUpdate *stackCreateDockerStandaloneRepositoryInputAutoUpdate `json:"autoUpdate,omitempty" jsonschema:"Optional GitOps update configuration"`
	// ComposeFile Path to the Stack file inside the Git repository
	ComposeFile *string `json:"composeFile,omitempty" jsonschema:"Path to the Stack file inside the Git repository"`
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID int `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	// Env A list of environment variables used during stack deployment
	Env []stackCreateDockerStandaloneRepositoryInputEnvItem `json:"env,omitempty" jsonschema:"A list of environment variables used during stack deployment"`
	// FilesystemPath Local filesystem path
	FilesystemPath *string `json:"filesystemPath,omitempty" jsonschema:"Local filesystem path" edition:"EE"`
	// FromAppTemplate Whether the stack is from a app template
	FromAppTemplate *bool `json:"fromAppTemplate,omitempty" jsonschema:"Whether the stack is from a app template"`
	// Name Name of the stack
	Name string `json:"name" jsonschema:"Name of the stack"`
	// Registries List of Registries to use for this stack
	Registries []int `json:"registries,omitempty" jsonschema:"List of Registries to use for this stack" edition:"EE"`
	// RepositoryAuthentication Deprecated: use SourceID instead. Use basic authentication to clone the Git repository.
	RepositoryAuthentication *bool `json:"repositoryAuthentication,omitempty" jsonschema:"Deprecated: use SourceID instead. Use basic authentication to clone the Git repository."`
	// RepositoryAuthorizationType Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use.
	RepositoryAuthorizationType *int `json:"repositoryAuthorizationType,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use." edition:"EE"`
	// RepositoryPassword Deprecated: use SourceID instead.
	// Password used in basic authentication. Required when RepositoryAuthentication is true
	RepositoryPassword *string `json:"repositoryPassword,omitempty" jsonschema:"Deprecated: use SourceID instead.\nPassword used in basic authentication. Required when RepositoryAuthentication is true"`
	// RepositoryProvider Deprecated: use SourceID instead. RepositoryProvider is the provider to use.
	RepositoryProvider *int `json:"repositoryProvider,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryProvider is the provider to use." edition:"EE"`
	// RepositoryReferenceName Reference name of a Git repository hosting the Stack file
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty" jsonschema:"Reference name of a Git repository hosting the Stack file"`
	// RepositoryURL Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file.
	RepositoryURL *string `json:"repositoryUrl,omitempty" jsonschema:"Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file."`
	// RepositoryUsername Deprecated: use SourceID instead.
	// Username used in basic authentication. Required when RepositoryAuthentication is true
	RepositoryUsername *string `json:"repositoryUsername,omitempty" jsonschema:"Deprecated: use SourceID instead.\nUsername used in basic authentication. Required when RepositoryAuthentication is true"`
	// SourceID SourceID references an existing Source for git credentials/URL.
	// When set, the inline URL and authentication fields are ignored.
	SourceID *int `json:"sourceId,omitempty" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	// SupportRelativePath Whether the stack supports relative path volume
	SupportRelativePath *bool `json:"supportRelativePath,omitempty" jsonschema:"Whether the stack supports relative path volume" edition:"EE"`
	// TLSSkipVerify Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository.
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository."`
}

func (stackCreateDockerStandaloneRepositoryInput) EnumParams() map[string][]any {
	return map[string][]any{
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
	}
}

// stackCreateDockerStandaloneRepositoryInputAutoUpdate: Optional GitOps update configuration
type stackCreateDockerStandaloneRepositoryInputAutoUpdate struct {
	// ForcePullImage Pull latest image
	ForcePullImage *bool `json:"forcePullImage,omitempty" jsonschema:"Pull latest image"`
	// ForceUpdate Force update ignores repo changes
	ForceUpdate *bool `json:"forceUpdate,omitempty" jsonschema:"Force update ignores repo changes"`
	// Interval Auto update interval
	// Deprecated: polling interval now lives on the associated Source (Source.Interval).
	// Kept for DB backwards-compatibility only; new code must not read or write this field.
	Interval *string `json:"interval,omitempty" jsonschema:"Auto update interval\nDeprecated: polling interval now lives on the associated Source (Source.Interval).\nKept for DB backwards-compatibility only; new code must not read or write this field."`
	// JobID Autoupdate job id
	JobID *string `json:"jobId,omitempty" jsonschema:"Autoupdate job id"`
	// Webhook A UUID generated from client
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID generated from client"`
}

// stackCreateDockerStandaloneRepositoryInputEnvItem is a nested object property.
type stackCreateDockerStandaloneRepositoryInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stackCreateDockerStandaloneStringInput is the parameter shape for operation StackCreateDockerStandaloneString (POST /stacks/create/standalone/string).
type stackCreateDockerStandaloneStringInput struct {
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID int `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	// Env A list of environment variables used during stack deployment
	Env []stackCreateDockerStandaloneStringInputEnvItem `json:"env,omitempty" jsonschema:"A list of environment variables used during stack deployment"`
	// FromAppTemplate Whether the stack is from a app template
	FromAppTemplate *bool `json:"fromAppTemplate,omitempty" jsonschema:"Whether the stack is from a app template"`
	// Name Name of the stack
	Name string `json:"name" jsonschema:"Name of the stack"`
	// Registries List of Registries to use for this stack
	Registries []int `json:"registries,omitempty" jsonschema:"List of Registries to use for this stack" edition:"EE"`
	// StackFileContent Content of the Stack file
	StackFileContent string `json:"stackFileContent" jsonschema:"Content of the Stack file"`
	// Webhook A UUID to identify a webhook. The stack will be force updated and pull the latest image when the webhook was invoked.
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID to identify a webhook. The stack will be force updated and pull the latest image when the webhook was invoked." edition:"EE"`
}

// stackCreateDockerStandaloneStringInputEnvItem is a nested object property.
type stackCreateDockerStandaloneStringInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stackCreateDockerSwarmRepositoryInput is the parameter shape for operation StackCreateDockerSwarmRepository (POST /stacks/create/swarm/repository).
type stackCreateDockerSwarmRepositoryInput struct {
	// AdditionalFiles Applicable when deploying with multiple stack files
	AdditionalFiles []string `json:"additionalFiles,omitempty" jsonschema:"Applicable when deploying with multiple stack files"`
	// AutoUpdate Optional GitOps update configuration
	AutoUpdate *stackCreateDockerSwarmRepositoryInputAutoUpdate `json:"autoUpdate,omitempty" jsonschema:"Optional GitOps update configuration"`
	// ComposeFile Path to the Stack file inside the Git repository
	ComposeFile *string `json:"composeFile,omitempty" jsonschema:"Path to the Stack file inside the Git repository"`
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID int `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	// Env A list of environment variables used during stack deployment
	Env []stackCreateDockerSwarmRepositoryInputEnvItem `json:"env,omitempty" jsonschema:"A list of environment variables used during stack deployment"`
	// FilesystemPath Network filesystem path
	FilesystemPath *string `json:"filesystemPath,omitempty" jsonschema:"Network filesystem path" edition:"EE"`
	// FromAppTemplate Whether the stack is from a app template
	FromAppTemplate *bool `json:"fromAppTemplate,omitempty" jsonschema:"Whether the stack is from a app template"`
	// Name Name of the stack
	Name string `json:"name" jsonschema:"Name of the stack"`
	// Registries List of Registries to use for this stack
	Registries []int `json:"registries,omitempty" jsonschema:"List of Registries to use for this stack" edition:"EE"`
	// RepositoryAuthentication Deprecated: use SourceID instead. Use basic authentication to clone the Git repository.
	RepositoryAuthentication *bool `json:"repositoryAuthentication,omitempty" jsonschema:"Deprecated: use SourceID instead. Use basic authentication to clone the Git repository."`
	// RepositoryAuthorizationType Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use.
	RepositoryAuthorizationType *int `json:"repositoryAuthorizationType,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use." edition:"EE"`
	// RepositoryPassword Deprecated: use SourceID instead.
	// Password used in basic authentication. Required when RepositoryAuthentication is true
	RepositoryPassword *string `json:"repositoryPassword,omitempty" jsonschema:"Deprecated: use SourceID instead.\nPassword used in basic authentication. Required when RepositoryAuthentication is true"`
	// RepositoryProvider Deprecated: use SourceID instead. RepositoryProvider is the provider to use.
	RepositoryProvider *int `json:"repositoryProvider,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryProvider is the provider to use." edition:"EE"`
	// RepositoryReferenceName Reference name of a Git repository hosting the Stack file
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty" jsonschema:"Reference name of a Git repository hosting the Stack file"`
	// RepositoryURL Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file.
	RepositoryURL *string `json:"repositoryUrl,omitempty" jsonschema:"Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file."`
	// RepositoryUsername Deprecated: use SourceID instead.
	// Username used in basic authentication. Required when RepositoryAuthentication is true
	RepositoryUsername *string `json:"repositoryUsername,omitempty" jsonschema:"Deprecated: use SourceID instead.\nUsername used in basic authentication. Required when RepositoryAuthentication is true"`
	// SourceID SourceID references an existing Source for git credentials/URL.
	// When set, the inline URL and authentication fields are ignored.
	SourceID *int `json:"sourceId,omitempty" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	// SupportRelativePath Whether the stack suppors relative path volume
	SupportRelativePath *bool `json:"supportRelativePath,omitempty" jsonschema:"Whether the stack suppors relative path volume" edition:"EE"`
	// SwarmID Swarm cluster identifier
	SwarmID string `json:"swarmId" jsonschema:"Swarm cluster identifier"`
	// TLSSkipVerify Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository.
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository."`
}

func (stackCreateDockerSwarmRepositoryInput) EnumParams() map[string][]any {
	return map[string][]any{
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
	}
}

// stackCreateDockerSwarmRepositoryInputAutoUpdate: Optional GitOps update configuration
type stackCreateDockerSwarmRepositoryInputAutoUpdate struct {
	// ForcePullImage Pull latest image
	ForcePullImage *bool `json:"forcePullImage,omitempty" jsonschema:"Pull latest image"`
	// ForceUpdate Force update ignores repo changes
	ForceUpdate *bool `json:"forceUpdate,omitempty" jsonschema:"Force update ignores repo changes"`
	// Interval Auto update interval
	// Deprecated: polling interval now lives on the associated Source (Source.Interval).
	// Kept for DB backwards-compatibility only; new code must not read or write this field.
	Interval *string `json:"interval,omitempty" jsonschema:"Auto update interval\nDeprecated: polling interval now lives on the associated Source (Source.Interval).\nKept for DB backwards-compatibility only; new code must not read or write this field."`
	// JobID Autoupdate job id
	JobID *string `json:"jobId,omitempty" jsonschema:"Autoupdate job id"`
	// Webhook A UUID generated from client
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID generated from client"`
}

// stackCreateDockerSwarmRepositoryInputEnvItem is a nested object property.
type stackCreateDockerSwarmRepositoryInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stackCreateDockerSwarmStringInput is the parameter shape for operation StackCreateDockerSwarmString (POST /stacks/create/swarm/string).
type stackCreateDockerSwarmStringInput struct {
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID int `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	// Env A list of environment variables used during stack deployment
	Env []stackCreateDockerSwarmStringInputEnvItem `json:"env,omitempty" jsonschema:"A list of environment variables used during stack deployment"`
	// FromAppTemplate Whether the stack is from a app template
	FromAppTemplate *bool `json:"fromAppTemplate,omitempty" jsonschema:"Whether the stack is from a app template"`
	// Name Name of the stack
	Name string `json:"name" jsonschema:"Name of the stack"`
	// Registries List of Registries to use for this stack
	Registries []int `json:"registries,omitempty" jsonschema:"List of Registries to use for this stack" edition:"EE"`
	// StackFileContent Content of the Stack file
	StackFileContent string `json:"stackFileContent" jsonschema:"Content of the Stack file"`
	// SwarmID Swarm cluster identifier
	SwarmID string `json:"swarmId" jsonschema:"Swarm cluster identifier"`
	// Webhook A UUID to identify a webhook. The stack will be force updated and pull the latest image when the webhook was invoked.
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID to identify a webhook. The stack will be force updated and pull the latest image when the webhook was invoked." edition:"EE"`
}

// stackCreateDockerSwarmStringInputEnvItem is a nested object property.
type stackCreateDockerSwarmStringInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stackCreateKubernetesFileInput is the parameter shape for operation StackCreateKubernetesFile (POST /stacks/create/kubernetes/string).
type stackCreateKubernetesFileInput struct {
	ComposeFormat *bool `json:"composeFormat,omitempty"`
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID int `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	// FromAppTemplate Whether the stack is from a app template
	FromAppTemplate  *bool   `json:"fromAppTemplate,omitempty" jsonschema:"Whether the stack is from a app template"`
	Namespace        *string `json:"namespace,omitempty"`
	StackFileContent *string `json:"stackFileContent,omitempty"`
	StackName        *string `json:"stackName,omitempty"`
}

// stackCreateKubernetesGitInput is the parameter shape for operation StackCreateKubernetesGit (POST /stacks/create/kubernetes/repository).
type stackCreateKubernetesGitInput struct {
	AdditionalFiles []string                                 `json:"additionalFiles,omitempty"`
	AutoUpdate      *stackCreateKubernetesGitInputAutoUpdate `json:"autoUpdate,omitempty"`
	ComposeFormat   *bool                                    `json:"composeFormat,omitempty"`
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID int `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	// HelmChartPath Helm-specific fields
	HelmChartPath *string `json:"helmChartPath,omitempty" jsonschema:"Helm-specific fields" edition:"EE"`
	// HelmValuesFiles Array of paths to values YAML files in Git repo
	HelmValuesFiles []string `json:"helmValuesFiles,omitempty" jsonschema:"Array of paths to values YAML files in Git repo" edition:"EE"`
	ManifestFile    *string  `json:"manifestFile,omitempty"`
	Namespace       *string  `json:"namespace,omitempty"`
	// RepositoryAuthentication Deprecated: use SourceID instead. Use authentication to clone the Git repository.
	RepositoryAuthentication *bool `json:"repositoryAuthentication,omitempty" jsonschema:"Deprecated: use SourceID instead. Use authentication to clone the Git repository."`
	// RepositoryAuthorizationType Deprecated: use SourceID instead. Authorization type used to clone the Git repository.
	RepositoryAuthorizationType *int `json:"repositoryAuthorizationType,omitempty" jsonschema:"Deprecated: use SourceID instead. Authorization type used to clone the Git repository." edition:"EE"`
	// RepositoryPassword Deprecated: use SourceID instead. Password used in authentication.
	RepositoryPassword *string `json:"repositoryPassword,omitempty" jsonschema:"Deprecated: use SourceID instead. Password used in authentication."`
	// RepositoryProvider Deprecated: use SourceID instead. Provider used to clone the Git repository.
	RepositoryProvider *int `json:"repositoryProvider,omitempty" jsonschema:"Deprecated: use SourceID instead. Provider used to clone the Git repository." edition:"EE"`
	// RepositoryReferenceName Deprecated: use SourceID instead. Reference name of a Git repository hosting the Stack file.
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty" jsonschema:"Deprecated: use SourceID instead. Reference name of a Git repository hosting the Stack file."`
	// RepositoryURL Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file.
	RepositoryURL *string `json:"repositoryUrl,omitempty" jsonschema:"Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file."`
	// RepositoryUsername Deprecated: use SourceID instead. Username used in authentication.
	RepositoryUsername *string `json:"repositoryUsername,omitempty" jsonschema:"Deprecated: use SourceID instead. Username used in authentication."`
	// SourceID SourceID references an existing Source for git credentials/URL.
	// When set, the inline URL and authentication fields are ignored.
	SourceID  *int    `json:"sourceId,omitempty" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	StackName *string `json:"stackName,omitempty"`
	// TLSSkipVerify Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository.
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository."`
}

func (stackCreateKubernetesGitInput) EnumParams() map[string][]any {
	return map[string][]any{
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
	}
}

// stackCreateKubernetesGitInputAutoUpdate is a nested object property.
type stackCreateKubernetesGitInputAutoUpdate struct {
	// ForcePullImage Pull latest image
	ForcePullImage *bool `json:"forcePullImage,omitempty" jsonschema:"Pull latest image"`
	// ForceUpdate Force update ignores repo changes
	ForceUpdate *bool `json:"forceUpdate,omitempty" jsonschema:"Force update ignores repo changes"`
	// Interval Auto update interval
	// Deprecated: polling interval now lives on the associated Source (Source.Interval).
	// Kept for DB backwards-compatibility only; new code must not read or write this field.
	Interval *string `json:"interval,omitempty" jsonschema:"Auto update interval\nDeprecated: polling interval now lives on the associated Source (Source.Interval).\nKept for DB backwards-compatibility only; new code must not read or write this field."`
	// JobID Autoupdate job id
	JobID *string `json:"jobId,omitempty" jsonschema:"Autoupdate job id"`
	// Webhook A UUID generated from client
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID generated from client"`
}

// stackCreateKubernetesURLInput is the parameter shape for operation StackCreateKubernetesUrl (POST /stacks/create/kubernetes/url).
type stackCreateKubernetesURLInput struct {
	ComposeFormat *bool `json:"composeFormat,omitempty"`
	// EndpointID Identifier of the environment that will be used to deploy the stack
	EndpointID  int     `json:"endpointId" jsonschema:"Identifier of the environment that will be used to deploy the stack"`
	ManifestURL *string `json:"manifestUrl,omitempty"`
	Namespace   *string `json:"namespace,omitempty"`
	StackName   *string `json:"stackName,omitempty"`
}

// stackDeleteInput is the parameter shape for operation StackDelete (DELETE /stacks/{id}).
type stackDeleteInput struct {
	// EndpointID Environment identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment identifier"`
	// External Set to true to delete an external stack. Only external Swarm stacks are supported
	External *bool `json:"external,omitempty" jsonschema:"Set to true to delete an external stack. Only external Swarm stacks are supported"`
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
	// RemoveVolumes Set to true to delete named volumes declared in the stack file and anonymous volumes attached to containers. Only affects Docker Standalone stacks.
	RemoveVolumes *bool `json:"removeVolumes,omitempty" jsonschema:"Set to true to delete named volumes declared in the stack file and anonymous volumes attached to containers. Only affects Docker Standalone stacks." edition:"EE"`
}

func (stackDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackDeleteKubernetesByNameInput is the parameter shape for operation StackDeleteKubernetesByName (DELETE /stacks/name/{name}).
type stackDeleteKubernetesByNameInput struct {
	// EndpointID Environment identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment identifier"`
	// External Set to true to delete an external stack. Only external Swarm stacks are supported
	External *bool `json:"external,omitempty" jsonschema:"Set to true to delete an external stack. Only external Swarm stacks are supported"`
	// Name Stack name
	Name string `json:"name" jsonschema:"Stack name"`
}

// stackFileInspectInput is the parameter shape for operation StackFileInspect (GET /stacks/{id}/file).
type stackFileInspectInput struct {
	// CommitHash Git repository commit hash. If both version and commitHash are provided, the commitHash will be used
	CommitHash *string `json:"commitHash,omitempty" jsonschema:"Git repository commit hash. If both version and commitHash are provided, the commitHash will be used" edition:"EE"`
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
	// Version Stack file version maintained by Portainer. If both version and commitHash are provided, the commitHash will be used
	Version *int `json:"version,omitempty" jsonschema:"Stack file version maintained by Portainer. If both version and commitHash are provided, the commitHash will be used" edition:"EE"`
}

func (stackFileInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackGitRedeployInput is the parameter shape for operation StackGitRedeploy (PUT /stacks/{id}/git/redeploy).
type stackGitRedeployInput struct {
	// EndpointID Stacks created before version 1.18.0 might not have an associated environment(endpoint) identifier. Use this optional parameter to set the environment(endpoint) identifier used by the stack.
	EndpointID *int                           `json:"endpointId,omitempty" jsonschema:"Stacks created before version 1.18.0 might not have an associated environment(endpoint) identifier. Use this optional parameter to set the environment(endpoint) identifier used by the stack."`
	Env        []stackGitRedeployInputEnvItem `json:"env,omitempty"`
	// ID Stack identifier
	ID    int   `json:"id" jsonschema:"Stack identifier"`
	Prune *bool `json:"prune,omitempty"`
	// PullImage Deprecated(2.36): use RepullImageAndRedeploy instead for cleaner responsibility
	// Force a pulling to current image with the original tag though the image is already the latest
	PullImage *bool `json:"pullImage,omitempty" jsonschema:"Deprecated(2.36): use RepullImageAndRedeploy instead for cleaner responsibility\nForce a pulling to current image with the original tag though the image is already the latest"`
	// RepositoryAuthentication When true and RepositoryPassword is non-empty, stored credentials are replaced.
	RepositoryAuthentication    *bool `json:"repositoryAuthentication,omitempty" jsonschema:"When true and RepositoryPassword is non-empty, stored credentials are replaced."`
	RepositoryAuthorizationType *int  `json:"repositoryAuthorizationType,omitempty" edition:"EE"`
	// RepositoryPassword Non-empty value (with RepositoryAuthentication=true) replaces stored credentials; leave blank to keep them.
	RepositoryPassword      *string `json:"repositoryPassword,omitempty" jsonschema:"Non-empty value (with RepositoryAuthentication=true) replaces stored credentials; leave blank to keep them."`
	RepositoryProvider      *int    `json:"repositoryProvider,omitempty" edition:"EE"`
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty"`
	RepositoryUsername      *string `json:"repositoryUsername,omitempty"`
	// RepullImageAndRedeploy RepullImageAndRedeploy indicates whether to force repulling images and redeploying the stack
	RepullImageAndRedeploy *bool   `json:"repullImageAndRedeploy,omitempty" jsonschema:"RepullImageAndRedeploy indicates whether to force repulling images and redeploying the stack"`
	StackName              *string `json:"stackName,omitempty"`
}

func (stackGitRedeployInput) EnumParams() map[string][]any {
	return map[string][]any{
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
	}
}

func (stackGitRedeployInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackGitRedeployInputEnvItem is a nested object property.
type stackGitRedeployInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stackImagesStatusInput is the parameter shape for operation StackImagesStatus (GET /stacks/{id}/images_status).
type stackImagesStatusInput struct {
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
	// Refresh Refresh will force a refresh of the image status cache
	Refresh *bool `json:"refresh,omitempty" jsonschema:"Refresh will force a refresh of the image status cache"`
}

func (stackImagesStatusInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackInspectInput is the parameter shape for operation StackInspect (GET /stacks/{id}).
type stackInspectInput struct {
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
}

func (stackInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackListInput is the parameter shape for operation StackList (GET /stacks).
type stackListInput struct {
	// Filters Filters to process on the stack list. Encoded as JSON (a map[string]string). For example, {'SwarmID': 'jpofkc0i9uo9wtx1zesuk649w'} will only return stacks that are part of the specified Swarm cluster. Available filters: EndpointID, SwarmID.
	Filters *string `json:"filters,omitempty" jsonschema:"Filters to process on the stack list. Encoded as JSON (a map[string]string). For example, {'SwarmID': 'jpofkc0i9uo9wtx1zesuk649w'} will only return stacks that are part of the specified Swarm cluster. Available filters: EndpointID, SwarmID."`
}

// stackStartInput is the parameter shape for operation StackStart (POST /stacks/{id}/start).
type stackStartInput struct {
	// EndpointID Environment identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment identifier"`
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
}

func (stackStartInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackStopInput is the parameter shape for operation StackStop (POST /stacks/{id}/stop).
type stackStopInput struct {
	// EndpointID Environment identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment identifier"`
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
}

func (stackStopInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackUpdateInput is the parameter shape for operation StackUpdate (PUT /stacks/{id}).
type stackUpdateInput struct {
	// EndpointID Environment identifier
	EndpointID int `json:"endpointId" jsonschema:"Environment identifier"`
	// Env A list of environment(endpoint) variables used during stack deployment
	Env []stackUpdateInputEnvItem `json:"env,omitempty" jsonschema:"A list of environment(endpoint) variables used during stack deployment"`
	// ID Stack identifier
	ID int `json:"id" jsonschema:"Stack identifier"`
	// Prune Prune services that are no longer referenced (only available for Swarm stacks)
	Prune *bool `json:"prune,omitempty" jsonschema:"Prune services that are no longer referenced (only available for Swarm stacks)"`
	// PullImage Deprecated(2.36): use RepullImageAndRedeploy instead for cleaner responsibility
	// Force a pulling to current image with the original tag though the image is already the latest
	PullImage *bool `json:"pullImage,omitempty" jsonschema:"Deprecated(2.36): use RepullImageAndRedeploy instead for cleaner responsibility\nForce a pulling to current image with the original tag though the image is already the latest"`
	// Registries List of Registries to use for this stack
	Registries []int `json:"registries,omitempty" jsonschema:"List of Registries to use for this stack" edition:"EE"`
	// RepullImageAndRedeploy RepullImageAndRedeploy indicates whether to force repulling images and redeploying the stack
	RepullImageAndRedeploy *bool `json:"repullImageAndRedeploy,omitempty" jsonschema:"RepullImageAndRedeploy indicates whether to force repulling images and redeploying the stack"`
	// RollbackTo RollbackTo specifies the stack file version to rollback to (only support to rollback to the last version currently)
	RollbackTo *int `json:"rollbackTo,omitempty" jsonschema:"RollbackTo specifies the stack file version to rollback to (only support to rollback to the last version currently)" edition:"EE"`
	// StackFileContent New content of the Stack file
	StackFileContent *string `json:"stackFileContent,omitempty" jsonschema:"New content of the Stack file"`
	// Webhook A UUID to identify a webhook. The stack will be force updated and pull the latest image when the webhook was invoked.
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID to identify a webhook. The stack will be force updated and pull the latest image when the webhook was invoked." edition:"EE"`
}

func (stackUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackUpdateInputEnvItem is a nested object property.
type stackUpdateInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stackUpdateGitInput is the parameter shape for operation StackUpdateGit (POST /stacks/{id}/git).
type stackUpdateGitInput struct {
	AdditionalFiles []string `json:"additionalFiles,omitempty"`
	// Atomic Enable atomic rollback on failure (Helm --atomic flag)
	Atomic         *bool                          `json:"atomic,omitempty" jsonschema:"Enable atomic rollback on failure (Helm --atomic flag)" edition:"EE"`
	AutoUpdate     *stackUpdateGitInputAutoUpdate `json:"autoUpdate,omitempty"`
	ConfigFilePath *string                        `json:"configFilePath,omitempty"`
	// EndpointID Stacks created before version 1.18.0 might not have an associated environment(endpoint) identifier. Use this optional parameter to set the environment(endpoint) identifier used by the stack.
	EndpointID *int                         `json:"endpointId,omitempty" jsonschema:"Stacks created before version 1.18.0 might not have an associated environment(endpoint) identifier. Use this optional parameter to set the environment(endpoint) identifier used by the stack."`
	Env        []stackUpdateGitInputEnvItem `json:"env,omitempty"`
	// HelmChartPath Helm chart folder path in Git repo (for Helm stacks)
	HelmChartPath *string `json:"helmChartPath,omitempty" jsonschema:"Helm chart folder path in Git repo (for Helm stacks)" edition:"EE"`
	// HelmValuesFiles Helm values files paths in Git repo (for Helm stacks)
	HelmValuesFiles []string `json:"helmValuesFiles,omitempty" jsonschema:"Helm values files paths in Git repo (for Helm stacks)" edition:"EE"`
	// ID Stack identifier
	ID         int   `json:"id" jsonschema:"Stack identifier"`
	Prune      *bool `json:"prune,omitempty"`
	Registries []int `json:"registries,omitempty" edition:"EE"`
	// RepositoryAuthentication Deprecated: use SourceID instead. Use basic authentication to clone the Git repository.
	RepositoryAuthentication *bool `json:"repositoryAuthentication,omitempty" jsonschema:"Deprecated: use SourceID instead. Use basic authentication to clone the Git repository."`
	// RepositoryAuthorizationType Deprecated: use SourceID instead. Authorization type for git authentication.
	RepositoryAuthorizationType *int `json:"repositoryAuthorizationType,omitempty" jsonschema:"Deprecated: use SourceID instead. Authorization type for git authentication." edition:"EE"`
	// RepositoryPassword Deprecated: use SourceID instead. Password used in basic authentication.
	RepositoryPassword *string `json:"repositoryPassword,omitempty" jsonschema:"Deprecated: use SourceID instead. Password used in basic authentication."`
	// RepositoryProvider Deprecated: use SourceID instead. Git provider for OAuth-based authentication.
	RepositoryProvider      *int    `json:"repositoryProvider,omitempty" jsonschema:"Deprecated: use SourceID instead. Git provider for OAuth-based authentication." edition:"EE"`
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty"`
	// RepositoryURL Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file.
	RepositoryURL *string `json:"repositoryUrl,omitempty" jsonschema:"Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file."`
	// RepositoryUsername Deprecated: use SourceID instead. Username used in basic authentication.
	RepositoryUsername *string `json:"repositoryUsername,omitempty" jsonschema:"Deprecated: use SourceID instead. Username used in basic authentication."`
	// SourceID SourceID references an existing Source for git credentials/URL.
	// When set, the inline URL and authentication fields are ignored.
	SourceID      *int  `json:"sourceId,omitempty" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty"`
}

func (stackUpdateGitInput) EnumParams() map[string][]any {
	return map[string][]any{
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
	}
}

func (stackUpdateGitInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// stackUpdateGitInputAutoUpdate is a nested object property.
type stackUpdateGitInputAutoUpdate struct {
	// ForcePullImage Pull latest image
	ForcePullImage *bool `json:"forcePullImage,omitempty" jsonschema:"Pull latest image"`
	// ForceUpdate Force update ignores repo changes
	ForceUpdate *bool `json:"forceUpdate,omitempty" jsonschema:"Force update ignores repo changes"`
	// Interval Auto update interval
	// Deprecated: polling interval now lives on the associated Source (Source.Interval).
	// Kept for DB backwards-compatibility only; new code must not read or write this field.
	Interval *string `json:"interval,omitempty" jsonschema:"Auto update interval\nDeprecated: polling interval now lives on the associated Source (Source.Interval).\nKept for DB backwards-compatibility only; new code must not read or write this field."`
	// JobID Autoupdate job id
	JobID *string `json:"jobId,omitempty" jsonschema:"Autoupdate job id"`
	// Webhook A UUID generated from client
	Webhook *string `json:"webhook,omitempty" jsonschema:"A UUID generated from client"`
}

// stackUpdateGitInputEnvItem is a nested object property.
type stackUpdateGitInputEnvItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// stacksWebhookInvokeInput is the parameter shape for operation StacksWebhookInvoke (POST /stacks/webhooks/{webhookID}).
type stacksWebhookInvokeInput struct {
	// WebhookID Stack identifier
	WebhookID string `json:"webhookID" jsonschema:"Stack identifier"`
}
