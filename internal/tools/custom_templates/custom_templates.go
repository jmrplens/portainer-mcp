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
// (internal/redact) — the same git-credential redactor
// PortainereeStack, StacksStackResponse and PortainereeEdgeStack's own
// domains use, since all four reach the identical RepoConfig type.
//
// generatedSpecs now lives in actions.go, written by `make scaffold-domain`:
// eight ActionSpecs, each wrapped in toolutil.WithNarrative(…,
// narrative(operationID)), and five of them calling one of the redaction
// wrappers below. handWrittenSpecs is still a stub, and still holds the
// throwaway reference to redactCustomTemplateCreateFile that keeps
// golangci-lint's unused check meaningful until the hand-written CreateFile
// handler calls it for real — a linter quieted about this domain's own
// redaction wrappers is exactly the wrong thing to quiet, since an uncalled
// wrapper is a silent path for a credential to reach a model unredacted.
// Replacing that stub's body with the real handler's ActionSpec removes the
// reference as a side effect of doing the work.
package custom_templates

import (
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/redact"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs returns every action this domain contributes to the catalog.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), handWrittenSpecs()...)
}

// handWrittenSpecs returns the one action this generator can never produce:
// CustomTemplateCreateFile. See this file's package doc for why.
//
// filled in by Task 5: today there is no hand-written handler yet, so this
// stub returns nil purely to let the package compile. The assignment below
// is the same throwaway-reference device generatedSpecs uses above, here
// for redactCustomTemplateCreateFile specifically: Task 5's hand-written
// handler is what is actually expected to call it, and replacing this
// stub's body with that handler's real ActionSpec entry deletes this line
// along with the rest of the stub.
func handWrittenSpecs() []toolutil.ActionSpec {
	_ = redactCustomTemplateCreateFile
	return nil
}

// narrative supplies the Title and Description overrides for operations
// whose own summary and description in the vendored specification do not
// say enough on their own. toolutil.WithNarrative applies it to every
// action in this domain, generated or hand-written, so that no action can
// acquire a literal Title/Description assignment that drifts from the spec
// unnoticed — see docs/domain-wave-checklist.md.
//
// Every shippable operation in this domain gets a case, which is unusual
// and deliberate. The scaffold run flagged seven of this domain's ten
// operations under "description merely restates its summary" — this domain
// and cloud are the joint second-worst in the specification, behind
// kubernetes and settings — and the collision is worse than that number
// suggests: CustomTemplateCreateFile, CustomTemplateCreateRepository and
// CustomTemplateCreateString share the byte-identical summary "Create a
// custom template", so all three arrive with the identical Title *and* the
// identical Description. A model asked to create a template from a git
// repository cannot pick between them from the generated prose, and
// portainer_find_action ranks them identically. Each Description below
// therefore names the one thing that distinguishes its action from its
// siblings — for the three creates, where the stack file body comes from.
//
// Four of the sentences below contradict the vendored specification on
// purpose. The first three were measured against a live Portainer 2.44.0,
// both editions; the fourth is a defect visible in the document itself:
//
//  1. SourceID is listed in CustomTemplateCreateRepository's "required"
//     array but the server does not require it: RepositoryURL with
//     Platform: 1 and no SourceID answers 200 and clones the repository,
//     and SourceID 0 sent explicitly answers 200 identically, so zero is
//     genuinely unset. It is read when a real one is supplied, though —
//     SourceID 99999 answers 500 "Source not found", validated against
//     /gitops/sources.
//  2. Platform is absent from the "required" array of both JSON create
//     routes, but the server requires it for Docker stacks: creating a
//     Type 2 template without Platform answers 500 "Invalid custom
//     template platform". The field's own description ("Required for
//     Docker stacks") is right and the required list is wrong.
//  3. Every inline repository field (RepositoryURL, RepositoryUsername,
//     RepositoryPassword, RepositoryAuthentication,
//     RepositoryAuthorizationType, RepositoryProvider) is marked
//     "Deprecated: use SourceID instead", yet that deprecated path is the
//     one measured working end to end.
//  4. CustomTemplateCreateRepository's Type declares enum [1, 2] while its
//     own description advertises "3 - kubernetes", which the two sibling
//     routes do declare. See docs/api-divergences.md §6.5: the enum is
//     published as declared, because nobody has measured whether that
//     route accepts Type 3, and the narrative sends a caller who needs a
//     Kubernetes template from git to create_string instead.
//
// Items 1 and 2 are not merely described here, they are corrected in
// inputs.go: toolutil.ActionSpec.ValidateInput enforces required-ness
// locally, before the handler runs, so publishing the vendored arrays would
// have let a model that fills exactly the required fields — which is what a
// model does — omit Platform and take that 500 every time, while refusing
// outright any caller cloning from the inline repository fields without a
// SourceID the server never wanted. Prose cannot mitigate a validator. Each
// correction needs a dated api/spec-drift-allowlist.yaml entry, which lands
// with this domain's registration — see inputs.go's own comment on Platform
// for why an entry added before then would itself fail the build.
//
// No case mentions what the Authentication object in a response contains,
// and that is deliberate. Portainer already blanks the git password itself
// — create/repository, GET custom_templates/{id} and GET custom_templates
// were all measured answering Authentication: {"Username":"…","Password":""}
// on both editions — while redactCustomTemplate below drops the whole
// Authentication object, Username included. These actions therefore return
// strictly less than Portainer does, so no narrative may promise a caller
// that a username will be there.
//
// CustomTemplateCreate (POST /custom_templates) has no case: it is
// deprecated upstream, cmd/gen_action_inputs skips it, and it never becomes
// an action. CustomTemplateCreateFile has one even though its ActionSpec is
// still hand-written elsewhere — the hook is keyed by operationId, so the
// case is ready for the handler that will call it.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "CustomTemplateList":
		return toolutil.ActionNarrative{
			Title: "List custom templates",
			Description: "Returns the custom templates visible to the caller: for each one its identifier, title, description, stack type, platform, ownership and — for a git-backed template — its repository configuration. " +
				"The stack file body is not included; custom_templates.file returns it for one template at a time. " +
				"type is required and selects which stack types to return (1 swarm, 2 compose, 3 kubernetes); edge returns Edge stack templates instead. " +
				"Any git credential a template stores is stripped before the list reaches you.",
		}
	case "CustomTemplateInspect":
		return toolutil.ActionNarrative{
			Title: "Inspect a custom template",
			Description: "Returns one custom template's definition by identifier — the same shape as one entry of custom_templates.list, and likewise without the stack file body, which custom_templates.file returns. " +
				"Any git credential the template stores is stripped before the result reaches you.",
		}
	case "CustomTemplateFile":
		return toolutil.ActionNarrative{
			Title: "Read a custom template's stack file",
			Description: "Returns the stack file body Portainer has stored for one custom template, as a single FileContent string and nothing else (measured against 2.44.0). " +
				"This is a read, despite the noun-like action name: it returns the stored copy and changes nothing. " +
				"Refreshing that copy from a git-backed template's remote is custom_templates.git_fetch, which overwrites it.",
		}
	case "CustomTemplateGitFetch":
		return toolutil.ActionNarrative{
			Title: "Overwrite a custom template from its git repository",
			Description: "Pulls the stack file from a git-backed template's repository and REPLACES the template's stored content with what the remote holds now, discarding whatever was stored — including edits made through custom_templates.update. " +
				"This is a write with no undo: Portainer keeps no previous version, and the request says nothing about what is being discarded. " +
				"Both the action name and the specification's own description (\"Retrieve details about a template created from git repository method\") read like a query; they are wrong, which is why this action is flagged destructive. " +
				"It answers with the new content only, as a single FileContent string (measured against 2.44.0). " +
				"Only meaningful for templates created by custom_templates.create_repository. " +
				"To read the stored content without replacing it, use custom_templates.file.",
		}
	case "CustomTemplateUpdate":
		return toolutil.ActionNarrative{
			Title: "Replace a custom template's definition",
			Description: "Replaces the whole definition of one custom template: every field sent is stored and every optional field omitted is cleared, so send the template's current values plus the change, not the change alone — read them first with custom_templates.inspect and custom_templates.file. " +
				"title, description, fileContent and type are all required, so an update meaning to change only the title still rewrites the stack file body with whatever fileContent it sends. " +
				"platform is optional here and required on the two create actions, and that difference is deliberate rather than an oversight: the create routes were measured rejecting a type 2 template without it (500 \"Invalid custom template platform\") and this route was not probed, so send platform for a Docker stack anyway — its \"Required for Docker stacks\" note applies here too. " +
				"The inline repository fields are marked deprecated in favour of sourceId, but are the path measured working. " +
				"Any git credential in the result is stripped before it reaches you.",
		}
	case "CustomTemplateCreateString":
		return toolutil.ActionNarrative{
			Title: "Create a custom template from inline content",
			Description: "Creates a custom template whose stack file body is the fileContent string passed in this call. " +
				"This is the one of the three create actions that needs no external source: custom_templates.create_repository clones the body from a git repository, and custom_templates.create_file takes it from an uploaded file. " +
				"title, description, fileContent, type and platform are all required here. " +
				"platform is required although the vendored specification's own required list omits it: the server enforces it for Docker stacks, and a type 2 (compose) template created without it was measured answering 500 \"Invalid custom template platform\" (1 linux, 2 windows). " +
				"Unlike custom_templates.create_repository, this route accepts type 3 (kubernetes) as well as 1 (swarm) and 2 (compose).",
		}
	case "CustomTemplateCreateRepository":
		return toolutil.ActionNarrative{
			Title: "Create a custom template from a git repository",
			Description: "Creates a custom template whose stack file body is cloned from a git repository, and re-cloned on demand by custom_templates.git_fetch. " +
				"The siblings take the body from an inline string (custom_templates.create_string) or an uploaded file (custom_templates.create_file). " +
				"The vendored specification's own required list is wrong in both directions here, and this action publishes the corrected one, measured against 2.44.0. " +
				"sourceId is optional despite being listed required: omit it, or pass 0, and the repository is cloned from repositoryUrl alone — pass a real identifier only if it exists under /gitops/sources, since an unknown one answers 500 \"Source not found\". " +
				"platform is required despite not being listed: without it a type 2 (compose) template answers 500 \"Invalid custom template platform\" (1 linux, 2 windows). " +
				"The inline repository fields (repositoryUrl, repositoryUsername, repositoryPassword, repositoryAuthentication, repositoryAuthorizationType, repositoryProvider) are all marked \"Deprecated: use SourceID instead\", yet that deprecated path is the one measured working end to end. " +
				"type accepts only 1 (swarm) or 2 (compose) on this route, even though the field's description advertises 3 (kubernetes) and the sibling routes accept it — for a Kubernetes template use custom_templates.create_string. " +
				"A credential sent here is stored by Portainer and stripped from this action's result before it reaches you.",
		}
	case "CustomTemplateCreateFile":
		return toolutil.ActionNarrative{
			Title: "Create a custom template from an uploaded file",
			Description: "Creates a custom template whose stack file body is an uploaded file, sent as multipart/form-data. " +
				"The siblings take the body from an inline string (custom_templates.create_string) or a git repository (custom_templates.create_repository). " +
				"title, description, note, platform, type and the file itself are all required. " +
				"This is the one create route whose vendored required list already names platform; the two JSON creates omit it although the server enforces it, so this catalog publishes it required on all three.",
		}
	case "CustomTemplateDelete":
		return toolutil.ActionNarrative{
			Title: "Delete a custom template",
			Description: "Permanently removes one custom template and the stack file Portainer stored for it. This cannot be undone. " +
				"It addresses the template only: anything previously deployed from it is a separate object this route does not reach.",
		}
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
func redactCustomTemplate(t *apigen.PortainereeCustomTemplate) *apigen.PortainereeCustomTemplate {
	if t == nil {
		return nil
	}
	scrubbed := *t
	scrubbed.GitConfig = redact.RepoConfig(scrubbed.GitConfig)
	return &scrubbed
}

// redactCustomTemplateList applies redactCustomTemplate to every element of
// a custom-template list response.
func redactCustomTemplateList(ts *[]apigen.PortainereeCustomTemplate) any {
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
func redactCustomTemplateInspect(t *apigen.PortainereeCustomTemplate) any {
	return redactCustomTemplate(t)
}

func redactCustomTemplateUpdate(t *apigen.PortainereeCustomTemplate) any {
	return redactCustomTemplate(t)
}

func redactCustomTemplateCreateRepository(t *apigen.PortainereeCustomTemplate) any {
	return redactCustomTemplate(t)
}

func redactCustomTemplateCreateString(t *apigen.PortainereeCustomTemplate) any {
	return redactCustomTemplate(t)
}

func redactCustomTemplateCreateFile(t *apigen.PortainereeCustomTemplate) any {
	return redactCustomTemplate(t)
}
