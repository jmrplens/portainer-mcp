// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package custom_templates

// customTemplateCreateRepositoryInput is the parameter shape for operation CustomTemplateCreateRepository (POST /custom_templates/create/repository).
type customTemplateCreateRepositoryInput struct {
	// ComposeFilePathInRepository Path to the Stack file inside the Git repository
	ComposeFilePathInRepository *string `json:"composeFilePathInRepository,omitempty" jsonschema:"Path to the Stack file inside the Git repository"`
	// Description Description of the template
	Description  string                                           `json:"description" jsonschema:"Description of the template"`
	EdgeSettings *customTemplateCreateRepositoryInputEdgeSettings `json:"edgeSettings,omitempty" edition:"EE"`
	// EdgeTemplate EdgeTemplate indicates if this template purpose for Edge Stack
	EdgeTemplate *bool `json:"edgeTemplate,omitempty" jsonschema:"EdgeTemplate indicates if this template purpose for Edge Stack"`
	// IsComposeFormat IsComposeFormat indicates if the Kubernetes template is created from a Docker Compose file
	IsComposeFormat *bool `json:"isComposeFormat,omitempty" jsonschema:"IsComposeFormat indicates if the Kubernetes template is created from a Docker Compose file"`
	// Logo URL of the template's logo
	Logo *string `json:"logo,omitempty" jsonschema:"URL of the template's logo"`
	// Note A note that will be displayed in the UI. Supports HTML content
	Note *string `json:"note,omitempty" jsonschema:"A note that will be displayed in the UI. Supports HTML content"`
	// Platform Platform associated to the template.
	// Valid values are: 1 - 'linux', 2 - 'windows'
	// Required for Docker stacks
	Platform *int `json:"platform,omitempty" jsonschema:"Platform associated to the template.\nValid values are: 1 - 'linux', 2 - 'windows'\nRequired for Docker stacks"`
	// RepositoryAuthentication Deprecated: use SourceID instead. Use basic authentication to clone the Git repository.
	RepositoryAuthentication *bool `json:"repositoryAuthentication,omitempty" jsonschema:"Deprecated: use SourceID instead. Use basic authentication to clone the Git repository."`
	// RepositoryAuthorizationType Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use
	RepositoryAuthorizationType *int `json:"repositoryAuthorizationType,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use" edition:"EE"`
	// RepositoryPassword Deprecated: use SourceID instead. Password used in basic authentication. Required when RepositoryAuthentication is true.
	RepositoryPassword *string `json:"repositoryPassword,omitempty" jsonschema:"Deprecated: use SourceID instead. Password used in basic authentication. Required when RepositoryAuthentication is true."`
	// RepositoryProvider Deprecated: use SourceID instead. RepositoryProvider is the provider to use
	RepositoryProvider *int `json:"repositoryProvider,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryProvider is the provider to use" edition:"EE"`
	// RepositoryReferenceName Reference name of a Git repository hosting the Stack file
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty" jsonschema:"Reference name of a Git repository hosting the Stack file"`
	// RepositoryURL Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file.
	RepositoryURL *string `json:"repositoryUrl,omitempty" jsonschema:"Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file."`
	// RepositoryUsername Deprecated: use SourceID instead. Username used in basic authentication. Required when RepositoryAuthentication is true.
	RepositoryUsername *string `json:"repositoryUsername,omitempty" jsonschema:"Deprecated: use SourceID instead. Username used in basic authentication. Required when RepositoryAuthentication is true."`
	// SourceID SourceID references an existing Source for git credentials/URL.
	// When set, the inline URL and authentication fields are ignored.
	SourceID int `json:"sourceId" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	// Title Title of the template
	Title string `json:"title" jsonschema:"Title of the template"`
	// TLSSkipVerify Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository.
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository."`
	// Type Type of created stack:
	// * 1 - swarm
	// * 2 - compose
	// * 3 - kubernetes
	Type int `json:"type" jsonschema:"Type of created stack:\n* 1 - swarm\n* 2 - compose\n* 3 - kubernetes"`
	// Variables Definitions of variables in the stack file
	Variables []customTemplateCreateRepositoryInputVariablesItem `json:"variables,omitempty" jsonschema:"Definitions of variables in the stack file"`
}

func (customTemplateCreateRepositoryInput) EnumParams() map[string][]any {
	return map[string][]any{
		"platform":                    {1, 2},
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
		"type":                        {1, 2},
	}
}

// customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings is a nested object property.
type customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings struct {
	// FilesystemPath Local filesystem path
	FilesystemPath *string `json:"filesystemPath,omitempty" jsonschema:"Local filesystem path" edition:"EE"`
	// PerDeviceConfigsGroupMatchType Per device configs group match type
	PerDeviceConfigsGroupMatchType *string `json:"perDeviceConfigsGroupMatchType,omitempty" jsonschema:"Per device configs group match type" edition:"EE"`
	// PerDeviceConfigsMatchType Per device configs match type
	PerDeviceConfigsMatchType *string `json:"perDeviceConfigsMatchType,omitempty" jsonschema:"Per device configs match type" edition:"EE"`
	// PerDeviceConfigsPath Per device configs path
	PerDeviceConfigsPath *string `json:"perDeviceConfigsPath,omitempty" jsonschema:"Per device configs path" edition:"EE"`
	// SupportPerDeviceConfigs Whether the edge stack supports per device configs
	SupportPerDeviceConfigs *bool `json:"supportPerDeviceConfigs,omitempty" jsonschema:"Whether the edge stack supports per device configs" edition:"EE"`
	// SupportRelativePath Whether the stack supports relative path volume
	SupportRelativePath *bool `json:"supportRelativePath,omitempty" jsonschema:"Whether the stack supports relative path volume" edition:"EE"`
}

func (customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings) EnumParams() map[string][]any {
	return map[string][]any{
		"perDeviceConfigsGroupMatchType": {"file", " dir"},
		"perDeviceConfigsMatchType":      {"file", " dir"},
	}
}

// customTemplateCreateRepositoryInputEdgeSettingsStaggerConfig: StaggerConfig is the configuration for staggered update
type customTemplateCreateRepositoryInputEdgeSettingsStaggerConfig struct {
	DeviceNumber            *int `json:"deviceNumber,omitempty" edition:"EE"`
	DeviceNumberIncrementBy *int `json:"deviceNumberIncrementBy,omitempty" edition:"EE"`
	DeviceNumberStartFrom   *int `json:"deviceNumberStartFrom,omitempty" edition:"EE"`
	StaggerOption           *int `json:"staggerOption,omitempty" edition:"EE"`
	StaggerParallelOption   *int `json:"staggerParallelOption,omitempty" edition:"EE"`
	// Timeout Timeout unit is minute
	Timeout *string `json:"timeout,omitempty" jsonschema:"Timeout unit is minute" edition:"EE"`
	// UpdateDelay UpdateDelay unit is minute
	UpdateDelay         *string `json:"updateDelay,omitempty" jsonschema:"UpdateDelay unit is minute" edition:"EE"`
	UpdateFailureAction *int    `json:"updateFailureAction,omitempty" edition:"EE"`
}

func (customTemplateCreateRepositoryInputEdgeSettingsStaggerConfig) EnumParams() map[string][]any {
	return map[string][]any{
		"staggerOption":         {0, 1, 2},
		"staggerParallelOption": {0, 1, 2},
		"updateFailureAction":   {0, 1, 2, 3},
	}
}

// customTemplateCreateRepositoryInputEdgeSettings is a nested object property.
type customTemplateCreateRepositoryInputEdgeSettings struct {
	// PrePullImage Pre Pull Image
	PrePullImage         *bool                                                                `json:"prePullImage,omitempty" jsonschema:"Pre Pull Image" edition:"EE"`
	PrivateRegistryID    *int                                                                 `json:"privateRegistryId,omitempty" edition:"EE"`
	RelativePathSettings *customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings `json:"relativePathSettings,omitempty" edition:"EE"`
	// RetryDeploy Retry deploy
	RetryDeploy *bool `json:"retryDeploy,omitempty" jsonschema:"Retry deploy" edition:"EE"`
	RetryPeriod *int  `json:"retryPeriod,omitempty" edition:"EE"`
	// StaggerConfig StaggerConfig is the configuration for staggered update
	StaggerConfig *customTemplateCreateRepositoryInputEdgeSettingsStaggerConfig `json:"staggerConfig,omitempty" jsonschema:"StaggerConfig is the configuration for staggered update" edition:"EE"`
}

// customTemplateCreateRepositoryInputVariablesItem is a nested object property.
type customTemplateCreateRepositoryInputVariablesItem struct {
	DefaultValue *string `json:"defaultValue,omitempty"`
	Description  *string `json:"description,omitempty"`
	Label        *string `json:"label,omitempty"`
	Name         *string `json:"name,omitempty"`
}

// customTemplateCreateStringInput is the parameter shape for operation CustomTemplateCreateString (POST /custom_templates/create/string).
type customTemplateCreateStringInput struct {
	// Description Description of the template
	Description  string                                       `json:"description" jsonschema:"Description of the template"`
	EdgeSettings *customTemplateCreateStringInputEdgeSettings `json:"edgeSettings,omitempty" edition:"EE"`
	// EdgeTemplate EdgeTemplate indicates if this template purpose for Edge Stack
	EdgeTemplate *bool `json:"edgeTemplate,omitempty" jsonschema:"EdgeTemplate indicates if this template purpose for Edge Stack"`
	// FileContent Content of stack file
	FileContent string `json:"fileContent" jsonschema:"Content of stack file"`
	// Logo URL of the template's logo
	Logo *string `json:"logo,omitempty" jsonschema:"URL of the template's logo"`
	// Note A note that will be displayed in the UI. Supports HTML content
	Note *string `json:"note,omitempty" jsonschema:"A note that will be displayed in the UI. Supports HTML content"`
	// Platform Platform associated to the template.
	// Valid values are: 1 - 'linux', 2 - 'windows'
	// Required for Docker stacks
	Platform *int `json:"platform,omitempty" jsonschema:"Platform associated to the template.\nValid values are: 1 - 'linux', 2 - 'windows'\nRequired for Docker stacks"`
	// Title Title of the template
	Title string `json:"title" jsonschema:"Title of the template"`
	// Type Type of created stack:
	// * 1 - swarm
	// * 2 - compose
	// * 3 - kubernetes
	Type int `json:"type" jsonschema:"Type of created stack:\n* 1 - swarm\n* 2 - compose\n* 3 - kubernetes"`
	// Variables Definitions of variables in the stack file
	Variables []customTemplateCreateStringInputVariablesItem `json:"variables,omitempty" jsonschema:"Definitions of variables in the stack file"`
}

func (customTemplateCreateStringInput) EnumParams() map[string][]any {
	return map[string][]any{
		"platform": {1, 2},
		"type":     {1, 2, 3},
	}
}

// customTemplateCreateStringInputEdgeSettingsRelativePathSettings is a nested object property.
type customTemplateCreateStringInputEdgeSettingsRelativePathSettings struct {
	// FilesystemPath Local filesystem path
	FilesystemPath *string `json:"filesystemPath,omitempty" jsonschema:"Local filesystem path" edition:"EE"`
	// PerDeviceConfigsGroupMatchType Per device configs group match type
	PerDeviceConfigsGroupMatchType *string `json:"perDeviceConfigsGroupMatchType,omitempty" jsonschema:"Per device configs group match type" edition:"EE"`
	// PerDeviceConfigsMatchType Per device configs match type
	PerDeviceConfigsMatchType *string `json:"perDeviceConfigsMatchType,omitempty" jsonschema:"Per device configs match type" edition:"EE"`
	// PerDeviceConfigsPath Per device configs path
	PerDeviceConfigsPath *string `json:"perDeviceConfigsPath,omitempty" jsonschema:"Per device configs path" edition:"EE"`
	// SupportPerDeviceConfigs Whether the edge stack supports per device configs
	SupportPerDeviceConfigs *bool `json:"supportPerDeviceConfigs,omitempty" jsonschema:"Whether the edge stack supports per device configs" edition:"EE"`
	// SupportRelativePath Whether the stack supports relative path volume
	SupportRelativePath *bool `json:"supportRelativePath,omitempty" jsonschema:"Whether the stack supports relative path volume" edition:"EE"`
}

func (customTemplateCreateStringInputEdgeSettingsRelativePathSettings) EnumParams() map[string][]any {
	return map[string][]any{
		"perDeviceConfigsGroupMatchType": {"file", " dir"},
		"perDeviceConfigsMatchType":      {"file", " dir"},
	}
}

// customTemplateCreateStringInputEdgeSettingsStaggerConfig: StaggerConfig is the configuration for staggered update
type customTemplateCreateStringInputEdgeSettingsStaggerConfig struct {
	DeviceNumber            *int `json:"deviceNumber,omitempty" edition:"EE"`
	DeviceNumberIncrementBy *int `json:"deviceNumberIncrementBy,omitempty" edition:"EE"`
	DeviceNumberStartFrom   *int `json:"deviceNumberStartFrom,omitempty" edition:"EE"`
	StaggerOption           *int `json:"staggerOption,omitempty" edition:"EE"`
	StaggerParallelOption   *int `json:"staggerParallelOption,omitempty" edition:"EE"`
	// Timeout Timeout unit is minute
	Timeout *string `json:"timeout,omitempty" jsonschema:"Timeout unit is minute" edition:"EE"`
	// UpdateDelay UpdateDelay unit is minute
	UpdateDelay         *string `json:"updateDelay,omitempty" jsonschema:"UpdateDelay unit is minute" edition:"EE"`
	UpdateFailureAction *int    `json:"updateFailureAction,omitempty" edition:"EE"`
}

func (customTemplateCreateStringInputEdgeSettingsStaggerConfig) EnumParams() map[string][]any {
	return map[string][]any{
		"staggerOption":         {0, 1, 2},
		"staggerParallelOption": {0, 1, 2},
		"updateFailureAction":   {0, 1, 2, 3},
	}
}

// customTemplateCreateStringInputEdgeSettings is a nested object property.
type customTemplateCreateStringInputEdgeSettings struct {
	// PrePullImage Pre Pull Image
	PrePullImage         *bool                                                            `json:"prePullImage,omitempty" jsonschema:"Pre Pull Image" edition:"EE"`
	PrivateRegistryID    *int                                                             `json:"privateRegistryId,omitempty" edition:"EE"`
	RelativePathSettings *customTemplateCreateStringInputEdgeSettingsRelativePathSettings `json:"relativePathSettings,omitempty" edition:"EE"`
	// RetryDeploy Retry deploy
	RetryDeploy *bool `json:"retryDeploy,omitempty" jsonschema:"Retry deploy" edition:"EE"`
	RetryPeriod *int  `json:"retryPeriod,omitempty" edition:"EE"`
	// StaggerConfig StaggerConfig is the configuration for staggered update
	StaggerConfig *customTemplateCreateStringInputEdgeSettingsStaggerConfig `json:"staggerConfig,omitempty" jsonschema:"StaggerConfig is the configuration for staggered update" edition:"EE"`
}

// customTemplateCreateStringInputVariablesItem is a nested object property.
type customTemplateCreateStringInputVariablesItem struct {
	DefaultValue *string `json:"defaultValue,omitempty"`
	Description  *string `json:"description,omitempty"`
	Label        *string `json:"label,omitempty"`
	Name         *string `json:"name,omitempty"`
}

// customTemplateDeleteInput is the parameter shape for operation CustomTemplateDelete (DELETE /custom_templates/{id}).
type customTemplateDeleteInput struct {
	// ID Template identifier
	ID int `json:"id" jsonschema:"Template identifier"`
}

func (customTemplateDeleteInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// customTemplateFileInput is the parameter shape for operation CustomTemplateFile (GET /custom_templates/{id}/file).
type customTemplateFileInput struct {
	// ID Template identifier
	ID int `json:"id" jsonschema:"Template identifier"`
}

func (customTemplateFileInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// customTemplateGitFetchInput is the parameter shape for operation CustomTemplateGitFetch (PUT /custom_templates/{id}/git_fetch).
type customTemplateGitFetchInput struct {
	// ID Template identifier
	ID int `json:"id" jsonschema:"Template identifier"`
}

func (customTemplateGitFetchInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// customTemplateInspectInput is the parameter shape for operation CustomTemplateInspect (GET /custom_templates/{id}).
type customTemplateInspectInput struct {
	// ID Template identifier
	ID int `json:"id" jsonschema:"Template identifier"`
}

func (customTemplateInspectInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// customTemplateListInput is the parameter shape for operation CustomTemplateList (GET /custom_templates).
type customTemplateListInput struct {
	// Edge Filter by edge templates
	Edge *bool `json:"edge,omitempty" jsonschema:"Filter by edge templates"`
	// Type Template types
	Type []int `json:"type" jsonschema:"Template types"`
}

// customTemplateUpdateInput is the parameter shape for operation CustomTemplateUpdate (PUT /custom_templates/{id}).
type customTemplateUpdateInput struct {
	// ComposeFilePathInRepository Path to the Stack file inside the Git repository
	ComposeFilePathInRepository *string `json:"composeFilePathInRepository,omitempty" jsonschema:"Path to the Stack file inside the Git repository"`
	// Description Description of the template
	Description  string                                 `json:"description" jsonschema:"Description of the template"`
	EdgeSettings *customTemplateUpdateInputEdgeSettings `json:"edgeSettings,omitempty" edition:"EE"`
	// EdgeTemplate EdgeTemplate indicates if this template purpose for Edge Stack
	EdgeTemplate *bool `json:"edgeTemplate,omitempty" jsonschema:"EdgeTemplate indicates if this template purpose for Edge Stack"`
	// FileContent Content of stack file
	FileContent string `json:"fileContent" jsonschema:"Content of stack file"`
	// ID Template identifier
	ID int `json:"id" jsonschema:"Template identifier"`
	// IsComposeFormat IsComposeFormat indicates if the Kubernetes template is created from a Docker Compose file
	IsComposeFormat *bool `json:"isComposeFormat,omitempty" jsonschema:"IsComposeFormat indicates if the Kubernetes template is created from a Docker Compose file"`
	// Logo URL of the template's logo
	Logo *string `json:"logo,omitempty" jsonschema:"URL of the template's logo"`
	// Note A note that will be displayed in the UI. Supports HTML content
	Note *string `json:"note,omitempty" jsonschema:"A note that will be displayed in the UI. Supports HTML content"`
	// Platform Platform associated to the template.
	// Valid values are: 1 - 'linux', 2 - 'windows'
	// Required for Docker stacks
	Platform *int `json:"platform,omitempty" jsonschema:"Platform associated to the template.\nValid values are: 1 - 'linux', 2 - 'windows'\nRequired for Docker stacks"`
	// RepositoryAuthentication Deprecated: use SourceID instead. Use authentication to clone the Git repository.
	RepositoryAuthentication *bool `json:"repositoryAuthentication,omitempty" jsonschema:"Deprecated: use SourceID instead. Use authentication to clone the Git repository."`
	// RepositoryAuthorizationType Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use
	RepositoryAuthorizationType *int `json:"repositoryAuthorizationType,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryAuthorizationType is the authorization type to use" edition:"EE"`
	// RepositoryPassword Deprecated: use SourceID instead. Password used in basic authentication or token used in token authentication. Required when RepositoryAuthentication is true.
	RepositoryPassword *string `json:"repositoryPassword,omitempty" jsonschema:"Deprecated: use SourceID instead. Password used in basic authentication or token used in token authentication. Required when RepositoryAuthentication is true."`
	// RepositoryProvider Deprecated: use SourceID instead. RepositoryProvider is the provider to use
	RepositoryProvider *int `json:"repositoryProvider,omitempty" jsonschema:"Deprecated: use SourceID instead. RepositoryProvider is the provider to use" edition:"EE"`
	// RepositoryReferenceName Reference name of a Git repository hosting the Stack file
	RepositoryReferenceName *string `json:"repositoryReferenceName,omitempty" jsonschema:"Reference name of a Git repository hosting the Stack file"`
	// RepositoryURL Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file.
	RepositoryURL *string `json:"repositoryUrl,omitempty" jsonschema:"Deprecated: use SourceID instead. URL of a Git repository hosting the Stack file."`
	// RepositoryUsername Deprecated: use SourceID instead. Username used in basic authentication. Required when RepositoryAuthentication is true.
	RepositoryUsername *string `json:"repositoryUsername,omitempty" jsonschema:"Deprecated: use SourceID instead. Username used in basic authentication. Required when RepositoryAuthentication is true."`
	// SourceID SourceID references an existing Source for git credentials/URL.
	// When set, the inline URL and authentication fields are ignored.
	SourceID *int `json:"sourceId,omitempty" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	// Title Title of the template
	Title string `json:"title" jsonschema:"Title of the template"`
	// TLSSkipVerify Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository.
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository."`
	// Type Type of created stack (1 - swarm, 2 - compose, 3 - kubernetes)
	Type int `json:"type" jsonschema:"Type of created stack (1 - swarm, 2 - compose, 3 - kubernetes)"`
	// Variables Definitions of variables in the stack file
	Variables []customTemplateUpdateInputVariablesItem `json:"variables,omitempty" jsonschema:"Definitions of variables in the stack file"`
}

func (customTemplateUpdateInput) EnumParams() map[string][]any {
	return map[string][]any{
		"platform":                    {1, 2},
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
		"type":                        {1, 2, 3},
	}
}

func (customTemplateUpdateInput) MinimumParams() map[string]int {
	return map[string]int{
		"id": 1,
	}
}

// customTemplateUpdateInputEdgeSettingsRelativePathSettings is a nested object property.
type customTemplateUpdateInputEdgeSettingsRelativePathSettings struct {
	// FilesystemPath Local filesystem path
	FilesystemPath *string `json:"filesystemPath,omitempty" jsonschema:"Local filesystem path" edition:"EE"`
	// PerDeviceConfigsGroupMatchType Per device configs group match type
	PerDeviceConfigsGroupMatchType *string `json:"perDeviceConfigsGroupMatchType,omitempty" jsonschema:"Per device configs group match type" edition:"EE"`
	// PerDeviceConfigsMatchType Per device configs match type
	PerDeviceConfigsMatchType *string `json:"perDeviceConfigsMatchType,omitempty" jsonschema:"Per device configs match type" edition:"EE"`
	// PerDeviceConfigsPath Per device configs path
	PerDeviceConfigsPath *string `json:"perDeviceConfigsPath,omitempty" jsonschema:"Per device configs path" edition:"EE"`
	// SupportPerDeviceConfigs Whether the edge stack supports per device configs
	SupportPerDeviceConfigs *bool `json:"supportPerDeviceConfigs,omitempty" jsonschema:"Whether the edge stack supports per device configs" edition:"EE"`
	// SupportRelativePath Whether the stack supports relative path volume
	SupportRelativePath *bool `json:"supportRelativePath,omitempty" jsonschema:"Whether the stack supports relative path volume" edition:"EE"`
}

func (customTemplateUpdateInputEdgeSettingsRelativePathSettings) EnumParams() map[string][]any {
	return map[string][]any{
		"perDeviceConfigsGroupMatchType": {"file", " dir"},
		"perDeviceConfigsMatchType":      {"file", " dir"},
	}
}

// customTemplateUpdateInputEdgeSettingsStaggerConfig: StaggerConfig is the configuration for staggered update
type customTemplateUpdateInputEdgeSettingsStaggerConfig struct {
	DeviceNumber            *int `json:"deviceNumber,omitempty" edition:"EE"`
	DeviceNumberIncrementBy *int `json:"deviceNumberIncrementBy,omitempty" edition:"EE"`
	DeviceNumberStartFrom   *int `json:"deviceNumberStartFrom,omitempty" edition:"EE"`
	StaggerOption           *int `json:"staggerOption,omitempty" edition:"EE"`
	StaggerParallelOption   *int `json:"staggerParallelOption,omitempty" edition:"EE"`
	// Timeout Timeout unit is minute
	Timeout *string `json:"timeout,omitempty" jsonschema:"Timeout unit is minute" edition:"EE"`
	// UpdateDelay UpdateDelay unit is minute
	UpdateDelay         *string `json:"updateDelay,omitempty" jsonschema:"UpdateDelay unit is minute" edition:"EE"`
	UpdateFailureAction *int    `json:"updateFailureAction,omitempty" edition:"EE"`
}

func (customTemplateUpdateInputEdgeSettingsStaggerConfig) EnumParams() map[string][]any {
	return map[string][]any{
		"staggerOption":         {0, 1, 2},
		"staggerParallelOption": {0, 1, 2},
		"updateFailureAction":   {0, 1, 2, 3},
	}
}

// customTemplateUpdateInputEdgeSettings is a nested object property.
type customTemplateUpdateInputEdgeSettings struct {
	// PrePullImage Pre Pull Image
	PrePullImage         *bool                                                      `json:"prePullImage,omitempty" jsonschema:"Pre Pull Image" edition:"EE"`
	PrivateRegistryID    *int                                                       `json:"privateRegistryId,omitempty" edition:"EE"`
	RelativePathSettings *customTemplateUpdateInputEdgeSettingsRelativePathSettings `json:"relativePathSettings,omitempty" edition:"EE"`
	// RetryDeploy Retry deploy
	RetryDeploy *bool `json:"retryDeploy,omitempty" jsonschema:"Retry deploy" edition:"EE"`
	RetryPeriod *int  `json:"retryPeriod,omitempty" edition:"EE"`
	// StaggerConfig StaggerConfig is the configuration for staggered update
	StaggerConfig *customTemplateUpdateInputEdgeSettingsStaggerConfig `json:"staggerConfig,omitempty" jsonschema:"StaggerConfig is the configuration for staggered update" edition:"EE"`
}

// customTemplateUpdateInputVariablesItem is a nested object property.
type customTemplateUpdateInputVariablesItem struct {
	DefaultValue *string `json:"defaultValue,omitempty"`
	Description  *string `json:"description,omitempty"`
	Label        *string `json:"label,omitempty"`
	Name         *string `json:"name,omitempty"`
}
