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
	//
	// Published required, against the vendored specification, which omits it
	// from this operation's "required" array: the server rejects a Type 2
	// template created without it with 500 "Invalid custom template
	// platform", measured against a live 2.44.0 on both editions. The
	// field's own description ("Required for Docker stacks") is what is
	// right here and the required array is what is wrong; a caller filling
	// only the published required fields would otherwise take that 500 every
	// time, which is exactly what a model does.
	//
	// This is a gating divergence from the vendored specification, reported
	// by cmd/audit_spec_drift as
	// CustomTemplateCreateRepository.platform [requiredness] "false" ->
	// "true", and excused by a dated api/spec-drift-allowlist.yaml entry.
	// That entry could not be added before this domain was registered: the
	// audit walks wiring.AllSpecs(), and an entry excusing nothing a real
	// run finds is reported stale, which is itself a build error. Both
	// therefore landed in the same commit.
	Platform int `json:"platform" jsonschema:"Platform associated to the template.\nValid values are: 1 - 'linux', 2 - 'windows'\nRequired for Docker stacks"`
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
	//
	// Published optional, against the vendored specification, which lists it
	// in this operation's "required" array: the server does not require it.
	// Measured against a live 2.44.0 on both editions — RepositoryURL with
	// Platform 1 and no SourceID answers 200 and clones the repository, and
	// SourceID 0 sent explicitly answers 200 identically, so zero is
	// genuinely unset rather than a value the server looks up. It is read
	// when a real one is supplied (SourceID 99999 answers 500 "Source not
	// found", validated against /gitops/sources), so this is optional, not
	// ignored. Published required, it would have made ValidateInput refuse
	// every request that legitimately clones from the inline repository
	// fields. Excused by a dated api/spec-drift-allowlist.yaml entry
	// (CustomTemplateCreateRepository/sourceId), added in the same commit
	// that registered this domain, for the reason given on Platform above.
	SourceID *int `json:"sourceId,omitempty" jsonschema:"SourceID references an existing Source for git credentials/URL.\nWhen set, the inline URL and authentication fields are ignored."`
	// Title Title of the template
	Title string `json:"title" jsonschema:"Title of the template"`
	// TLSSkipVerify Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository.
	TLSSkipVerify *bool `json:"tlsSkipVerify,omitempty" jsonschema:"Deprecated: use SourceID instead. TLSSkipVerify skips SSL verification when cloning the Git repository."`
	// Type Type of created stack:
	// * 1 - swarm
	// * 2 - compose
	// * 3 - kubernetes
	//
	// Published with all three values, against the vendored specification,
	// which declares enum [1, 2] on this route alone while its own
	// description (above, verbatim) advertises 3 and both sibling routes
	// declare [1, 2, 3]. The catalog carried the narrow enum deliberately
	// until somebody measured the server rather than trusting a neighbouring
	// route's declaration; that measurement is now in: a Type 3 template
	// created from a git repository answers 200 on a live 2.44.0, Community
	// and Business Edition alike, and comes back stored as type 3. See
	// docs/api-divergences.md §6.5, which prescribed exactly this widening
	// on exactly this evidence, and the dated
	// api/spec-drift-allowlist.yaml entry
	// (CustomTemplateCreateRepository/type) that excuses the resulting
	// gating [enum] finding.
	Type int `json:"type" jsonschema:"Type of created stack:\n* 1 - swarm\n* 2 - compose\n* 3 - kubernetes"`
	// Variables Definitions of variables in the stack file
	Variables []customTemplateCreateRepositoryInputVariablesItem `json:"variables,omitempty" jsonschema:"Definitions of variables in the stack file"`
}

func (customTemplateCreateRepositoryInput) EnumParams() map[string][]any {
	return map[string][]any{
		"platform":                    {1, 2},
		"repositoryAuthorizationType": {0, 1},
		"repositoryProvider":          {0, 1, 2, 3, 4, 5, 6},
		// [1, 2, 3], not the vendored [1, 2]: measured, see the Type field's
		// own comment above.
		"type": {1, 2, 3},
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

// EnumParams publishes "dir", not the vendored " dir".
//
// portaineree.CustomTemplateRelativePathSettings declares both these fields
// as an allOf $ref to portainer.PerDevConfigsFilterType — whose own enum is
// the clean ["file", "dir"] — while attaching an inline ["file", " dir"] of
// its own beside that $ref. A sibling keyword beats the $ref it sits next to,
// deliberately, in cmd/gen_action_inputs's resolver, so the leading space was
// what this method returned when the file was first scaffolded. Portainer's
// " dir" is a typo in the document, contradicted by the very component the
// same node references. The server was measured accepting "dir", " dir" and
// even "zzz-not-a-value" here, storing each verbatim, so it validates this
// field not at all: the published enum is a client-side constraint only, and
// therefore the only thing steering a model away from the typo.
//
// The generator no longer emits it: normaliseEnumValues
// (cmd/gen_action_inputs/schema.go) trims every enum value as it is read out
// of the document, so a regeneration produces exactly what is written here.
// See docs/api-divergences.md §6.6, and the identically-corrected methods on
// customTemplateCreateStringInputEdgeSettingsRelativePathSettings and
// customTemplateUpdateInputEdgeSettingsRelativePathSettings below, which the
// same schema feeds.
func (customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings) EnumParams() map[string][]any {
	return map[string][]any{
		"perDeviceConfigsGroupMatchType": {"file", "dir"},
		"perDeviceConfigsMatchType":      {"file", "dir"},
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
	//
	// Published required, against the vendored specification, for the same
	// measured reason as customTemplateCreateRepositoryInput.Platform above:
	// a Type 2 template created without it answers 500 "Invalid custom
	// template platform". This route also accepts Type 3 (kubernetes), for
	// which the server's own requirement was not separately measured;
	// requiring the field of every caller costs a Kubernetes template one
	// metadata value it can set to 1, while leaving it optional costs a
	// Docker one a 500. Excused by its own dated
	// api/spec-drift-allowlist.yaml entry (CustomTemplateCreateString/
	// platform — that file keys on (operation_id, field), so the sibling
	// route's entry does not cover this one), added in the same commit that
	// registered this domain, for the reason given on
	// customTemplateCreateRepositoryInput.Platform above.
	Platform int `json:"platform" jsonschema:"Platform associated to the template.\nValid values are: 1 - 'linux', 2 - 'windows'\nRequired for Docker stacks"`
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

// EnumParams publishes "dir", not the vendored " dir"; see
// customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings's own
// EnumParams above for why, and docs/api-divergences.md §6.6.
func (customTemplateCreateStringInputEdgeSettingsRelativePathSettings) EnumParams() map[string][]any {
	return map[string][]any{
		"perDeviceConfigsGroupMatchType": {"file", "dir"},
		"perDeviceConfigsMatchType":      {"file", "dir"},
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

// EnumParams publishes "dir", not the vendored " dir"; see
// customTemplateCreateRepositoryInputEdgeSettingsRelativePathSettings's own
// EnumParams above for why, and docs/api-divergences.md §6.6.
func (customTemplateUpdateInputEdgeSettingsRelativePathSettings) EnumParams() map[string][]any {
	return map[string][]any{
		"perDeviceConfigsGroupMatchType": {"file", "dir"},
		"perDeviceConfigsMatchType":      {"file", "dir"},
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

// The type below is hand-written, not scaffolded: cmd/gen_action_inputs
// never reaches CustomTemplateCreateFile, because oapi-codegen emitted only
// CustomTemplateCreateFileWithBodyWithResponse for its multipart-only
// request body and clientMethodFor looks up the plain
// CustomTemplateCreateFileWithResponse that does not exist (see
// custom_templates.go's package doc). Its handler lives in handlers.go and
// renders these fields into a multipart body through
// portainer.MultipartForm; every field name, type and required-ness below is
// transcribed from the "multipart/form-data" schema of POST
// /custom_templates/create/file in api/specs/ee-2.44.0.json.
//
// Unexported and with the domain's ordinary "ID" capitalisation, like every
// other Input struct in this file, for the reason docker/inputs.go states:
// golangci-lint's revive var-naming rule applies to struct fields regardless
// of whether the struct itself is exported.

// customTemplateCreateFileInput is the parameter shape for operation CustomTemplateCreateFile (POST /custom_templates/create/file).
//
// Unlike its two JSON siblings above, this route's vendored required array
// is self-consistent and is published verbatim: it already lists Platform
// (which the JSON creates omit although the server enforces it, the
// divergence recorded in docs/api-divergences.md §3.7) and its Type enum
// already admits 3 (which CustomTemplateCreateRepository's omits, §6.5). So
// there is no required-ness override here — the two corrections carried by
// customTemplateCreateRepositoryInput and customTemplateCreateStringInput
// must not be copied onto this route, since on this one they would be
// inventing a divergence rather than recording one.
//
// This route does carry three api/spec-drift-allowlist.yaml entries all the
// same, of an entirely different kind: EdgeSettings, File and Variables
// each publish a description that elaborates on the vendored one, because
// the vendored text is copied from the JSON create routes and is wrong
// about this route's own types (see each field below). A field-level
// description is not covered by toolutil.WithNarrative: cmd/audit_spec_drift's
// isGating fires on any ChangeDescription with a non-empty Before and,
// unlike the $title/$description kinds, does not consult AfterOverridden.
//
// Note is required here and optional on both JSON creates. That is what the
// vendored specification says, and it was later measured wrong: the server
// accepts the route with the Note part omitted (docs/api-divergences.md
// §3.7). The shape is published verbatim all the same — relaxing it changes
// this hand-written multipart handler's input and deserves its own change,
// not a side effect of the wave that happened to measure it.
type customTemplateCreateFileInput struct {
	// Description Description of the template
	Description string `json:"description" jsonschema:"Description of the template"`
	// EdgeSettings A json object of edge config
	//
	// A string, not a nested object, and deliberately so: this route's
	// multipart schema types EdgeSettings "string" holding JSON, where the
	// two JSON create routes take a real nested object. Sending an object
	// here would marshal a Go struct into the part and Portainer would fail
	// to unmarshal the part's own JSON. The caller supplies the encoded
	// document.
	EdgeSettings *string `json:"edgeSettings,omitempty" jsonschema:"A json object of edge config, passed as a JSON-encoded string" edition:"EE"`
	// EdgeTemplate Indicates if this template purpose for Edge Stack
	EdgeTemplate *bool `json:"edgeTemplate,omitempty" jsonschema:"Indicates if this template purpose for Edge Stack" edition:"EE"`
	// File File
	//
	// The uploaded stack file's content. The specification types it "string"
	// with format "binary" — an upload — and a model has no way to name a
	// path on the server this process runs on, so the content itself is what
	// crosses the tool boundary and handlers.go writes it as the multipart
	// file part. Text only, in consequence: a stack file, a compose file or
	// a Kubernetes manifest all are, but a payload that is not valid UTF-8
	// cannot be expressed as a JSON string and so cannot be uploaded through
	// this action.
	File string `json:"file" jsonschema:"Content of the stack file to upload"`
	// Logo URL of the template's logo
	Logo *string `json:"logo,omitempty" jsonschema:"URL of the template's logo"`
	// Note A note that will be displayed in the UI. Supports HTML content
	//
	// Required on this route, optional on both JSON creates. Published as
	// the specification declares it; see this type's own doc comment.
	Note string `json:"note" jsonschema:"A note that will be displayed in the UI. Supports HTML content"`
	// Platform Platform associated to the template (1 - 'linux', 2 - 'windows')
	Platform int `json:"platform" jsonschema:"Platform associated to the template (1 - 'linux', 2 - 'windows')"`
	// Title Title of the template
	Title string `json:"title" jsonschema:"Title of the template"`
	// Type Type of created stack (1 - swarm, 2 - compose, 3 - kubernetes)
	Type int `json:"type" jsonschema:"Type of created stack (1 - swarm, 2 - compose, 3 - kubernetes)"`
	// Variables A json array of variables definitions
	//
	// A string for the same reason EdgeSettings above is: this route's
	// multipart schema types it "string" holding a JSON array, where the
	// JSON creates take a real array of objects.
	Variables *string `json:"variables,omitempty" jsonschema:"A json array of variables definitions, passed as a JSON-encoded string"`
}

func (customTemplateCreateFileInput) EnumParams() map[string][]any {
	return map[string][]any{
		"platform": {1, 2},
		"type":     {1, 2, 3},
	}
}
