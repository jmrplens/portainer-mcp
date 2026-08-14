// Package custom_templates declares the Portainer custom-template actions.
//
// Nine operations ship here in both editions (List, Inspect, Update,
// CreateRepository, CreateString, CreateFile, Delete, File, GitFetch); a
// tenth, CustomTemplateCreate, does not, and two things about that absence
// and about CreateFile surprise a reader who only looks at the routes.
//
// CustomTemplateCreate (POST /custom_templates) is deprecated upstream — the
// generated client's CustomTemplateCreateWithResponse carries a
// `// Deprecated:` doc comment staticcheck's SA1019 flags — so
// cmd/gen_action_inputs skips it by default rather than expose a route
// Portainer itself is phasing out (main.go's deprecatedOps report). It
// already has an entry in api/coverage-allowlist.yaml. Its absence here is
// deliberate, not an oversight, and it needs no wrapper below: it is EE-only
// in the vendored specification, is the sole deprecated operation in this
// domain, and is skipped before checkCredentialRedaction ever runs.
//
// CustomTemplateCreateFile (POST /custom_templates/create/file) stays
// hand-written (a later task) because oapi-codegen emitted only a
// WithBody variant for its multipart-only request body: the generated
// client declares CustomTemplateCreateFileWithBodyWithResponse but no
// CustomTemplateCreateFileWithResponse — verified, zero hits:
//
//	grep "func (c \*ClientWithResponses) CustomTemplateCreateFileWithResponse(" internal/portainer/gen/client.gen.go
//
// clientMethodFor (handler.go) looks up the latter by the operation's own
// name and finds nothing, so this refusal is permanent regardless of how
// the domain is authored — it is not a redaction refusal, and declaring
// redactCustomTemplateCreateFile below does not make it go away.
//
// Six of these nine operations return a PortainereeCustomTemplate (or a
// slice of them) whose GitConfig field
// (*GithubComPortainerPortainerEeApiGitTypesRepoConfig) can carry a git
// credential in GitConfig.Authentication.Password. cmd/gen_action_inputs
// refuses to generate a bare handler for any of them —
// CustomTemplateList, CustomTemplateInspect, CustomTemplateUpdate,
// CustomTemplateCreateRepository, CustomTemplateCreateString and
// CustomTemplateCreateFile — until this domain declares a redaction
// wrapper of the exact name checkCredentialRedaction expects
// (redact<OperationID>). That check runs before the hand-written-override
// skip (main.go:471 precedes main.go:487), so CustomTemplateCreateFile
// needs its wrapper even though its handler will never be generated: the
// wrapper is what the hand-written handler in the later task is expected to
// call, the same way a hand-written handler elsewhere in this codebase
// states that it redacts. The six wrappers below all defer to
// redactCustomTemplate, which in turn defers to redact.RepoConfig
// (internal/tools/redact) — the same git-credential redactor
// PortainereeStack, StacksStackResponse and PortainereeEdgeStack's own
// domains use, since all four reach the identical RepoConfig type.
//
// generatedSpecs and handWrittenSpecs below are temporary stubs: this
// domain has no generated or hand-written code yet, only the scaffolding a
// later `make scaffold-domain` run and a hand-written CreateFile handler
// require to already exist. Because of that, narrative and the six redact*
// wrappers below have no caller yet either — every one of them carries a
// `//nolint:unused` for exactly that reason, each pointing at the task that
// wires it in. Task 4's `make scaffold-domain` run makes generatedSpecs
// call narrative and five of the six wrappers directly, and Task 5's
// hand-written handler calls redactCustomTemplateCreateFile; once both have
// landed, none of the six functions are unused any more and the
// nolint markers may be removed (they are harmless to leave, since this
// repository does not enable nolintlint).
package custom_templates

import (
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/tools/redact"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs returns every action this domain contributes to the catalog.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), handWrittenSpecs()...)
}

// generatedSpecs returns every ActionSpec cmd/gen_action_inputs derives for
// this domain.
//
// filled in by Task 4: today there is nothing to scaffold from, so this
// stub returns nil purely to let the package compile and let the
// scaffolder parse this file's declared symbols (the six redact* wrappers
// and narrative below) before it generates anything.
func generatedSpecs() []toolutil.ActionSpec {
	return nil
}

// handWrittenSpecs returns the one action this generator can never produce:
// CustomTemplateCreateFile. See this file's package doc for why.
//
// filled in by Task 5: today there is no hand-written handler yet, so this
// stub returns nil purely to let the package compile.
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
// Every case below returns the zero value: the generated titles this hook
// would need to react to are not visible until cmd/gen_action_inputs has
// actually scaffolded this domain, which is a later task. The hook must
// exist now regardless — emit.go only wraps a generated spec in
// toolutil.WithNarrative when the hand file already declares a function
// named exactly narrative, and that check runs at scaffold time, not
// afterward.
func narrative(operationID string) toolutil.ActionNarrative { //nolint:unused // called by generatedSpecs once Task 4 scaffolds this domain
	switch operationID {
	default:
		return toolutil.ActionNarrative{}
	}
}

// redactCustomTemplate returns a copy of t with its GitConfig's git
// credentials removed.
//
// Dropped through redact.RepoConfig, not field by field here: GitConfig's
// Authentication sub-object carries Password, but also Username and
// GitCredentialID, and RepoConfig already exists precisely so every
// git-backed domain shares one place that knows to drop all of it. Whether
// Portainer actually populates Authentication on a given response is not
// something this code should have to know, so every wrapper below redacts
// unconditionally, the same as registries' redact/redactList.
func redactCustomTemplate(t *apigen.PortainereeCustomTemplate) *apigen.PortainereeCustomTemplate { //nolint:unused // called by the wrappers below, themselves wired in by Task 4/5
	if t == nil {
		return nil
	}
	scrubbed := *t
	scrubbed.GitConfig = redact.RepoConfig(scrubbed.GitConfig)
	return &scrubbed
}

// redactCustomTemplateList applies redactCustomTemplate to every element of
// a custom-template list response.
func redactCustomTemplateList(ts *[]apigen.PortainereeCustomTemplate) any { //nolint:unused // called by generatedSpecs's CustomTemplateList handler, wired in by Task 4
	if ts == nil {
		return nil
	}
	out := make([]apigen.PortainereeCustomTemplate, len(*ts))
	for i := range *ts {
		out[i] = *redactCustomTemplate(&(*ts)[i])
	}
	return &out
}

// redactCustomTemplateInspect, redactCustomTemplateUpdate,
// redactCustomTemplateCreateRepository, redactCustomTemplateCreateString and
// redactCustomTemplateCreateFile (together with redactCustomTemplateList
// above) are the redaction wrappers cmd/gen_action_inputs's generator
// requires before it will generate a handler for
// CustomTemplateList/CustomTemplateInspect/CustomTemplateUpdate/
// CustomTemplateCreateRepository/CustomTemplateCreateString/
// CustomTemplateCreateFile at all: every one of their success responses can
// carry GitConfig.Authentication.Password (per
// toolutil.IsCredentialShapedName, resolved through the vendored spec's own
// $refs — see credential.go in that package), so the generator refuses to
// emit a bare handler for any of them without a function named exactly this
// way already declared here, checked before it even looks at whether the
// operation is otherwise hand-written. Each is a thin rename over
// redactCustomTemplate/redactCustomTemplateList this file already declares:
// the generator's contract only requires a function of this name and shape
// to exist, not that it be a new implementation — see registries.go's
// identical redactRegistryList/redactRegistryCreate/redactRegistryInspect/
// redactRegistryUpdate for the precedent this follows. Each wrapper keeps
// its parameter typed as the real concrete response type
// (*apigen.PortainereeCustomTemplate, or *[]apigen.PortainereeCustomTemplate
// for List) rather than any: a wrapper typed any would satisfy both this
// guard and `make audit-spec-drift` vacuously, without redacting anything —
// see docs/api-divergences.md §9.4.
func redactCustomTemplateInspect(t *apigen.PortainereeCustomTemplate) any { //nolint:unused // called by generatedSpecs's CustomTemplateInspect handler, wired in by Task 4
	return redactCustomTemplate(t)
}

func redactCustomTemplateUpdate(t *apigen.PortainereeCustomTemplate) any { //nolint:unused // called by generatedSpecs's CustomTemplateUpdate handler, wired in by Task 4
	return redactCustomTemplate(t)
}

func redactCustomTemplateCreateRepository(t *apigen.PortainereeCustomTemplate) any { //nolint:unused // called by generatedSpecs's CustomTemplateCreateRepository handler, wired in by Task 4
	return redactCustomTemplate(t)
}

func redactCustomTemplateCreateString(t *apigen.PortainereeCustomTemplate) any { //nolint:unused // called by generatedSpecs's CustomTemplateCreateString handler, wired in by Task 4
	return redactCustomTemplate(t)
}

func redactCustomTemplateCreateFile(t *apigen.PortainereeCustomTemplate) any { //nolint:unused // called by the hand-written CreateFile handler, wired in by Task 5
	return redactCustomTemplate(t)
}
