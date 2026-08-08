// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package docker

// containersImageStatusClearInput is the parameter shape for operation ContainersImageStatusClear (POST /docker/{environmentId}/containers/image_status/clear).
type containersImageStatusClearInput struct {
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
}

func (containersImageStatusClearInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}

// dockerDashboardInput is the parameter shape for operation DockerDashboard (GET /docker/{environmentId}/dashboard).
type dockerDashboardInput struct {
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
}

func (dockerDashboardInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}

// dockerImagesListInput is the parameter shape for operation DockerImagesList (GET /docker/{environmentId}/images).
type dockerImagesListInput struct {
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
	// WithUsage Include image usage information
	WithUsage *bool `json:"withUsage,omitempty" jsonschema:"Include image usage information"`
}

func (dockerImagesListInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}

// serviceImageStatusClearInput is the parameter shape for operation ServiceImageStatusClear (POST /docker/{environmentId}/services/image_status/clear).
type serviceImageStatusClearInput struct {
	// EnvironmentID Environment identifier
	EnvironmentID int `json:"environmentId" jsonschema:"Environment identifier"`
}

func (serviceImageStatusClearInput) MinimumParams() map[string]int {
	return map[string]int{
		"environmentId": 1,
	}
}

// stacksImageStatusClearInput is the parameter shape for operation StacksImageStatusClear (POST /stacks/image_status/clear).
type stacksImageStatusClearInput struct {
	// EnvironmentID Identifier of the environment(endpoint) that will be used to filter the stacks to clear the image status cache for
	EnvironmentID *int `json:"environmentId,omitempty" jsonschema:"Identifier of the environment(endpoint) that will be used to filter the stacks to clear the image status cache for"`
	// SwarmID Identifier of the swarm cluster that will be used to filter the stacks to clear the image status cache for
	SwarmID *string `json:"swarmId,omitempty" jsonschema:"Identifier of the swarm cluster that will be used to filter the stacks to clear the image status cache for"`
}
