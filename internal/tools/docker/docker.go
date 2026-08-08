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

import (
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs returns every action this domain contributes to the catalog.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), handWrittenSpecs()...)
}

// handWrittenSpecs returns the three actions this generator refuses to
// produce: DockerContainerGpusInspect, ContainerImageStatus and
// ServiceImageStatus. Each names a path parameter (containerId or serviceId)
// the vendored specification types "integer" when Portainer actually reads
// it as a string — Docker's own 64-character hex container ID, or Docker
// Swarm's own alphanumeric service ID — so the generated client's typed
// method, which bakes in the wrong Go type, can never be called with a real
// value. See handlers.go's package doc comment and docs/api-divergences.md
// §6.3.
//
// DockerContainerGpusInspect is declared in both vendored specifications
// (edition.CE); ContainerImageStatus and ServiceImageStatus are declared in
// the Business Edition specification only (edition.EE).
func handWrittenSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.container_gpus_inspect", Domain: "docker", OperationID: "DockerContainerGpusInspect",
			Title:       "Fetch container gpus data",
			Description: "Fetch container gpus data",
			Edition:     edition.CE,
			Handler:     dockerContainerGpusInspect,
			Input:       dockerContainerGpusInspectInput{},
		}, narrative("DockerContainerGpusInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.container_image_status", Domain: "docker", OperationID: "ContainerImageStatus",
			Title:       "Fetch image status for container",
			Description: "Fetch image status for container",
			Edition:     edition.EE,
			Handler:     containerImageStatus,
			Input:       containerImageStatusInput{},
		}, narrative("ContainerImageStatus")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.service_image_status", Domain: "docker", OperationID: "ServiceImageStatus",
			Title:       "Fetch image status for service",
			Description: "Fetch image status for service",
			Edition:     edition.EE,
			Handler:     serviceImageStatus,
			Input:       serviceImageStatusInput{},
		}, narrative("ServiceImageStatus")),
	}
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
// not from the spec's stripped text.
//
// DockerContainerGpusInspect, ContainerImageStatus and ServiceImageStatus
// trigger the identical boilerplate-description warning, but their override
// exists for a second, more important reason than the boilerplate itself:
// the vendored specification's own summary never mentions that containerId/
// serviceId is a string despite the schema declaring it an integer, and that
// is exactly what a model calling one of these three needs to know before it
// tries to pass a number.
//
// ContainerImageStatus and ServiceImageStatus carry a third reason, found
// while building this domain's e2e coverage (docs/api-divergences.md §2.4):
// ServiceImageStatus was measured answering a stale cached success for a
// Swarm service that had already been deleted, and kept doing so after the
// node left Swarm entirely. Only the spec's own optional refresh parameter,
// which forces a live docker service inspect, made it tell the truth. The
// vendored specification declares the identical refresh parameter on
// ContainerImageStatus, but that endpoint was not itself probed for the same
// caching, so its narrative below flags the risk as unverified rather than
// asserting it.
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
	case "DockerContainerGpusInspect":
		return toolutil.ActionNarrative{
			Description: "Returns the GPU device requests attached to one container, as recorded in its HostConfig.DeviceRequests. containerId is Docker's own 64-character hexadecimal container ID — published here as a string even though the vendored specification incorrectly declares it an integer; see docs/api-divergences.md.",
		}
	case "ContainerImageStatus":
		return toolutil.ActionNarrative{
			Description: "Reports whether a newer version of one container's image is available. The sibling docker.service_image_status action has been measured returning a stale cached answer for a resource that had already stopped existing unless refresh is passed; this action declares the identical refresh parameter and may cache the same way, but that has not itself been verified — pass refresh if the answer needs to reflect the container's current state. containerId is Docker's own 64-character hexadecimal container ID — published here as a string even though the vendored specification incorrectly declares it an integer; see docs/api-divergences.md.",
		}
	case "ServiceImageStatus":
		return toolutil.ActionNarrative{
			Description: "Reports whether a newer version of one Swarm service's image is available. Without refresh, Portainer can answer from a stale cache describing a service that no longer exists — measured surviving both the service's own deletion and its node leaving Swarm entirely; pass refresh to force a live check and get the service's current state. serviceId is Docker Swarm's own alphanumeric service ID — published here as a string even though the vendored specification incorrectly declares it an integer; see docs/api-divergences.md.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
