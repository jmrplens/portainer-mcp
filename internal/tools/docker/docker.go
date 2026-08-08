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
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	default:
		return toolutil.ActionNarrative{}
	}
}
