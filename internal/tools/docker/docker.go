// Package docker exposes Portainer's Docker-environment endpoints as catalog
// actions.
//
// Two things about this domain surprise a reader coming from the routes.
//
// stacksImageStatusClear lives at /stacks/image_status/clear, not under
// /docker/, but the vendored specification tags it "docker" and
// cmd/gen_action_inputs routes by tags[0] (spec.go), so it belongs here. Its
// action name says "stacks" for the same reason a reader would expect it to:
// the operation is about stacks' image status, and hiding that behind the
// directory it happens to live in would be worse than the mild oddity of
// docker.stacks_image_status_clear.
//
// Every operation in this domain declares security: [{jwt: []}] in the
// vendored specification, which read literally would make all eight
// uncallable with the API key this server uses. That is a documentation
// defect, not a constraint: dashboard and images were probed against a live
// 2.44.0 server and answer 200 with X-API-Key, and the 404s the others
// return are missing-resource, not authentication. See
// docs/api-divergences.md. There is deliberately no JWT code path here.
package docker

import "github.com/jmrplens/portainer-mcp/internal/toolutil"

// Specs returns every action this domain contributes to the catalog.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), handWrittenSpecs()...)
}

// handWrittenSpecs returns the actions this generator refuses to produce:
// dockerContainerGpusInspect, containerImageStatus and ServiceImageStatus,
// whose path parameter the vendored specification types as an integer when
// it is really a string (a Docker hex container ID or a Docker Swarm service
// ID). Task 3 of the docker wave writes these by hand; until then this
// returns nothing so that Specs() stays correct for this task's five
// mechanical operations.
func handWrittenSpecs() []toolutil.ActionSpec {
	return nil
}

// narrative supplies the Title and Description overrides for operations
// whose own summary and description in the vendored specification do not
// say enough on their own. toolutil.WithNarrative applies it to every
// action in this domain, generated or hand-written, so that no action can
// acquire a literal Title/Description assignment that drifts from the spec
// unnoticed — see docs/domain-wave-checklist.md.
//
// DockerDashboard and DockerImagesList are here because
// cmd/gen_action_inputs's scaffold run warns about both: the vendored
// specification's own "description" for each is nothing but
// "**Access policy**: ..." boilerplate, which the generator strips, so
// without an override Description falls back to repeating Title verbatim
// (see docs/domain-wave-checklist.md's Step 2, which names exactly this
// situation as a candidate a wave must review before accepting). Both
// descriptions below are written from the generated response type each
// handler actually returns (DockerDashboardData, []ImagesImageResponse),
// not from the spec's stripped text. DockerContainerGpusInspect,
// ContainerImageStatus and ServiceImageStatus trigger the identical warning
// but are Task 3's hand-written operations, not this task's — narrative
// entries for them belong there, not here.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "DockerDashboard":
		return toolutil.ActionNarrative{
			Description: "Returns dashboard counters for one Docker environment: the number of stacks, services, networks and volumes, plus a summary of container states (running/stopped/etc.) and image counts and total size.",
		}
	case "DockerImagesList":
		return toolutil.ActionNarrative{
			Description: "Lists every Docker image present on one environment, with each image's id, tags, creation time and size. Pass withUsage to also report whether each image is used by at least one container.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
