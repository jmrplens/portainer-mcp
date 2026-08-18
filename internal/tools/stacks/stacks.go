// Package stacks declares the Portainer stack actions.
//
// Twenty-five operations carry the "stacks" tag in the vendored Business
// Edition specification, and internal/toolutil.DomainTags maps this domain
// to that one tag, so this package owns all twenty-five. Three things about
// that set cannot be read off the routes, and one of them contradicts this
// wave's reconnaissance.
//
// First, one of the twenty-five is not under /stacks at all.
// EdgeStackWebhookInvoke (POST /edge_stacks/webhooks/{webhookID}) is tagged
// "stacks", not "edge_stacks", and cmd/gen_action_inputs groups operations
// by their first tag and never by path segment, so this domain owns that
// single /edge_stacks/ route and the future edge_stacks domain — which
// covers the tag of the same name, and the other fourteen /edge_stacks/
// paths the Business Edition document declares — will not.
//
// The reconnaissance for this wave records that route as tagged "stacks" in
// *both* vendored documents. Verified directly against the two committed
// specifications, that is wrong in a way worth stating rather than quietly
// correcting: /edge_stacks/webhooks/{webhookID} does not exist in
// api/specs/ce-2.44.0.json at all (that document declares seven
// /edge_stacks/ paths to the Business Edition document's fifteen), so the
// operation is Business Edition only and its action must be declared
// edition.EE. Only api/specs/ee-2.44.0.json carries it, and there it is
// tagged "stacks".
//
// A second edition asymmetry sits on the sibling route POST
// /stacks/webhooks/{webhookID}: both documents declare it, but the Business
// Edition calls it StacksWebhookInvoke and Community Edition calls it
// WebhookInvoke. cmd/gen_action_inputs enumerates from the Business Edition
// document alone and cmd/audit_1to1 keys coverage by operationId, so the
// Community Edition spelling will read as an uncovered operation the moment
// this domain lands. That is a wave-level decision (an allow-list entry, a
// second hand-declared ActionSpec, or an accepted gap recorded in
// docs/api-divergences.md), not something this file can settle.
//
// Second, three of the twenty-five can never be generated, and each refuses
// for its own reason. Two are multipart: StackCreateDockerStandaloneFile
// (POST /stacks/create/standalone/file) and StackCreateDockerSwarmFile (POST
// /stacks/create/swarm/file) have a multipart/form-data-only request body,
// so oapi-codegen emitted only a WithBody variant for each — verified, zero
// hits for either:
//
//	grep "func (c \*ClientWithResponses) StackCreateDockerStandaloneFileWithResponse(" internal/portainer/gen/client.gen.go
//	grep "func (c \*ClientWithResponses) StackCreateDockerSwarmFileWithResponse(" internal/portainer/gen/client.gen.go
//
// clientMethodFor (handler.go in cmd/gen_action_inputs) looks the latter
// name up and finds nothing, so the refusal stands however the domain is
// authored. The third is StackMigrate (POST /stacks/{id}/migrate), whose
// query parameter "endpointId" is published as "endpointIdQuery" because
// the request body contributes an EndpointID property that renders to the
// same wire name and, per internal/specnaming, the body keeps the plain
// name. A generated handler distributes query parameters by unmarshalling
// the caller's raw input straight into apigen.StackMigrateParams, whose
// field is tagged `json:"endpointId,omitempty"` — so it would read the body
// property's value into the query parameter and silently drop the caller's
// own. buildHandlerSpec refuses exactly that, naming both names; the
// handler is hand-written instead.
//
// Third, fifteen of the twenty-five return a success response that can
// carry a git credential, all through one field:
// GitConfig.Authentication.Password, reached from apigen.PortainereeStack
// (thirteen of them), from a slice of it (StackList) and from
// apigen.StacksStackResponse (StackUpdateGit — the only success type in
// this domain that is not PortainereeStack, verified against
// StackUpdateGitResponse.JSON200 in the generated client). The remaining
// ten return a Kubernetes-create, convert, file or image-status response,
// or no body at all, and none of those reaches a RepoConfig.
//
// cmd/gen_action_inputs refuses to emit a handler for any of the fifteen
// until this file declares a function named exactly redact<OperationID>
// (checkCredentialRedaction, credential.go). That check runs before the
// hand-written-override skip, deliberately — it is the one guard a
// hand-written handler cannot walk around — so all three hand-written
// operations above need their wrapper too even though no handler will ever
// be generated for them. For those three the wrapper is not dead weight:
// the hand-written handler is expected to call it, which is how a
// hand-written handler in this codebase states that it redacts.
//
// Every wrapper below carries its real concrete parameter type. A wrapper
// declared func redactX(r any) any { return r } satisfies both the
// generated reflective guard and `make audit-spec-drift` while redacting
// nothing at all — docs/api-divergences.md §9.4 documents that hazard, and
// the previous stage demonstrated it. All fifteen defer to redactStack,
// redactStackResponse or the loop in redactStackList, which in turn defer to
// redact.RepoConfig (internal/redact) — the same git-credential redactor
// custom_templates uses, since PortainereeStack, StacksStackResponse,
// PortainereeCustomTemplate and PortainereeEdgeStack all reach the identical
// RepoConfig type.
//
// generatedSpecs and handWrittenSpecs are stubs today, and both are meant to
// be replaced wholesale rather than edited. `make scaffold-domain` writes
// its own generatedSpecs into actions.go, so the stub of that name below
// MUST be deleted before the domain is scaffolded — two declarations of one
// name in one package do not compile, and the scaffolder additionally reads
// this package's declared function names to decide what is already
// hand-written. The keep-alive references inside the two stub bodies exist
// so that no redaction wrapper is held live by a lint directive: `unused` is
// the one linter that reports a redaction wrapper losing its call site, and
// a lost call site is a silent credential exposure, so it must never be
// switched off here.
package stacks

import (
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/redact"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs returns every action this domain contributes to the catalog.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), handWrittenSpecs()...)
}

// generatedSpecs returns every ActionSpec cmd/gen_action_inputs derives for
// this domain — twenty-two of the twenty-five operations.
//
// Written by the scaffold task into actions.go, which is where the real
// declaration will live: this stub is deleted by that task before it runs
// the scaffolder, not merged with what the scaffolder writes.
//
// It returns nil today so the package compiles and so the scaffolder can
// parse this file's declared symbols — narrative and the fifteen redaction
// wrappers — before it generates anything. The assignments below are what
// hold those symbols live in the meantime. They name every wrapper whose
// call site will be a generated handler, plus narrative, which every
// generated ActionSpec calls through toolutil.WithNarrative; they disappear
// with this stub at exactly the moment those real call sites arrive.
func generatedSpecs() []toolutil.ActionSpec {
	_ = narrative
	_ = redactStackList
	_ = redactStackInspect
	_ = redactStackCreateDockerStandaloneRepository
	_ = redactStackCreateDockerStandaloneString
	_ = redactStackCreateDockerSwarmRepository
	_ = redactStackCreateDockerSwarmString
	_ = redactStackUpdate
	_ = redactStackUpdateGit
	_ = redactStackAssociate
	_ = redactStackGitRedeploy
	_ = redactStackStart
	_ = redactStackStop
	return nil
}

// handWrittenSpecs returns the three actions cmd/gen_action_inputs can never
// produce — StackCreateDockerStandaloneFile, StackCreateDockerSwarmFile and
// StackMigrate. See this file's package doc for why each refuses.
//
// Filled in by the hand-written-handler tasks; nil today so the package
// compiles. The three assignments below hold live the wrappers whose call
// site will be one of those hand-written handlers, and disappear with this
// stub when the handlers land. Deliberately no ActionSpec literal and no
// function named stackMigrate yet: scanHandOverrides reads both an
// OperationID literal and a mechanically-named handler function as "already
// hand-written" and would skip the operation, and the scaffold task expects
// all three refusals to be reported.
func handWrittenSpecs() []toolutil.ActionSpec {
	_ = redactStackCreateDockerStandaloneFile
	_ = redactStackCreateDockerSwarmFile
	_ = redactStackMigrate
	return nil
}

// narrative supplies the Title and Description overrides for operations
// whose own summary and description in the vendored specification do not say
// enough on their own. toolutil.WithNarrative applies it to every action in
// this domain, generated or hand-written, so that no action can acquire a
// literal Title/Description assignment that drifts from the spec unnoticed —
// see docs/domain-wave-checklist.md.
//
// Every case returns the zero value today. The generated titles this hook
// has to react to are not visible until cmd/gen_action_inputs has actually
// scaffolded this domain and printed its description-quality report, which
// is a later task. The hook must exist now regardless: emit.go wraps a
// generated spec in toolutil.WithNarrative only when this file already
// declares a function named exactly narrative, and it checks that at
// scaffold time. Scaffolding before the hook exists emits every description
// bare, with no error and no warning.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	default:
		return toolutil.ActionNarrative{}
	}
}

// redactStack returns a copy of s with its GitConfig's git credentials
// removed.
//
// Dropped through redact.RepoConfig, not field by field here: GitConfig's
// Authentication sub-object carries Password, but also Username and
// GitCredentialID, and RepoConfig already exists precisely so every
// git-backed domain shares one place that knows to drop all of it. Whether
// Portainer actually populates Authentication on a given response is not
// something this code should have to know, so every wrapper below redacts
// unconditionally, the same as registries' redact/redactList.
func redactStack(s *apigen.PortainereeStack) *apigen.PortainereeStack {
	if s == nil {
		return nil
	}
	scrubbed := *s
	scrubbed.GitConfig = redact.RepoConfig(scrubbed.GitConfig)
	return &scrubbed
}

// redactStackResponse is redactStack for apigen.StacksStackResponse, the
// success type of StackUpdateGit alone. It is a separate function rather
// than a generic one because the two structs are unrelated Go types that
// merely happen to share a GitConfig field of the same RepoConfig type: the
// only thing to share is redact.RepoConfig itself, and both do.
func redactStackResponse(s *apigen.StacksStackResponse) *apigen.StacksStackResponse {
	if s == nil {
		return nil
	}
	scrubbed := *s
	scrubbed.GitConfig = redact.RepoConfig(scrubbed.GitConfig)
	return &scrubbed
}

// redactStackList applies redactStack to every element of a stack list
// response.
func redactStackList(ss *[]apigen.PortainereeStack) any {
	if ss == nil {
		return nil
	}
	out := make([]apigen.PortainereeStack, len(*ss))
	for i := range *ss {
		out[i] = *redactStack(&(*ss)[i])
	}
	return &out
}

// The thirteen wrappers below, together with redactStackList above, are the
// redaction wrappers cmd/gen_action_inputs requires before it will generate
// a handler for the fifteen stacks operations whose success response can
// carry GitConfig.Authentication.Password (per
// toolutil.IsCredentialShapedName, resolved through the vendored spec's own
// $refs — see credential.go in that package). The generator refuses to emit
// a bare handler for any of them without a function named exactly this way
// already declared here, checked before it even looks at whether the
// operation is otherwise hand-written — which is why
// redactStackCreateDockerStandaloneFile, redactStackCreateDockerSwarmFile
// and redactStackMigrate are here even though their handlers are
// hand-written and will call these wrappers themselves.
//
// Each is a thin rename over redactStack (or, for StackUpdateGit alone,
// redactStackResponse): the generator's contract only requires a function of
// this name and shape to exist, not that it be a new implementation — see
// registries.go's redactRegistryList/redactRegistryCreate/
// redactRegistryInspect/redactRegistryUpdate and custom_templates.go's six
// for the precedent this follows.
//
// Each keeps its parameter typed as the real concrete response type
// (*apigen.PortainereeStack, *[]apigen.PortainereeStack for List,
// *apigen.StacksStackResponse for UpdateGit) rather than any: a wrapper
// typed any would satisfy both this guard and `make audit-spec-drift`
// vacuously, without redacting anything — see docs/api-divergences.md §9.4.
func redactStackInspect(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackCreateDockerStandaloneFile(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackCreateDockerStandaloneRepository(s *apigen.PortainereeStack) any {
	return redactStack(s)
}

func redactStackCreateDockerStandaloneString(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackCreateDockerSwarmFile(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackCreateDockerSwarmRepository(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackCreateDockerSwarmString(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackUpdate(s *apigen.PortainereeStack) any { return redactStack(s) }

// redactStackUpdateGit is the one wrapper in this domain whose parameter is
// not a PortainereeStack: POST /stacks/{id}/git answers with
// stacks.stackResponse, which the generated client renders as
// StacksStackResponse.
func redactStackUpdateGit(s *apigen.StacksStackResponse) any { return redactStackResponse(s) }

func redactStackAssociate(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackGitRedeploy(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackStart(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackStop(s *apigen.PortainereeStack) any { return redactStack(s) }

func redactStackMigrate(s *apigen.PortainereeStack) any { return redactStack(s) }
