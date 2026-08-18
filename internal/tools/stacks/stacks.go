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
// document alone and cmd/audit_1to1 keyed coverage by operationId, so the
// Community Edition spelling would have read as an uncovered operation the
// moment this domain landed. That is settled outside this package and not by
// anything this domain declares: cmd/audit_1to1 now carries an alias table
// (alias.go) recording the two operationIds as one route, so covering either
// covers both. A second ActionSpec was rejected because actioncatalog.Build
// refuses two specs sharing a Name before it filters by edition, which would
// have forced a Community user to call stacks.webhook_invoke_ce for the route
// an EE user calls stacks.webhook_invoke; a coverage allow-list entry was
// rejected because that file is for operations that will never be exposed as
// an MCP action, and this one is exposed, under the other name. The action
// stays edition.EE, because the catalog is still built from the Business
// Edition document and StacksWebhookInvoke is the name that resolves there.
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
// generatedSpecs now lives in actions.go, written by `make scaffold-domain`,
// and the stub of that name that used to sit in this file is gone: two
// declarations of one name in one package do not compile, so it had to be
// deleted before the scaffolder ran. Its keep-alive block went with it —
// every wrapper it held live now has a real call site in a generated
// handler, which is the only reason it existed. handWrittenSpecs is no
// longer a stub either: all three of its actions are declared, their
// handlers are in handlers.go, and the keep-alive references that stood in
// for those call sites are gone — the last of them, for redactStackMigrate,
// left with that handler. No redaction wrapper in this file may
// ever be held live by a lint directive instead: `unused` is the one linter
// that reports a redaction wrapper losing its call site, and a lost call
// site is a silent credential exposure.
package stacks

import (
	"github.com/jmrplens/portainer-mcp/internal/edition"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/redact"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs returns every action this domain contributes to the catalog.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), handWrittenSpecs()...)
}

// handWrittenSpecs returns the three actions cmd/gen_action_inputs can never
// produce — StackCreateDockerStandaloneFile, StackCreateDockerSwarmFile and
// StackMigrate. See this file's package doc for why each refuses.
//
// All three are declared below, over the handlers in handlers.go. Those
// handlers are what call redactStackCreateDockerStandaloneFile,
// redactStackCreateDockerSwarmFile and redactStackMigrate: every one of the
// three wrappers was declared before a call site existed, held live only by a
// throwaway reference in this function's earlier stub, and the last of those
// references is gone with this commit now that real call sites do the work.
// Nothing mechanical would have noticed their absence — golangci-lint's
// unused check would simply have gone quiet — which is why the calls are
// pinned by tests of their own (handlers_test.go) rather than left to review:
// an uncalled redaction wrapper is a silent path for a credential to reach a
// model.
//
// The first two specs are Mutating and NOT Destructive, matching the four JSON
// creates: each adds a stack at a new identifier and removes nothing. Neither
// is Idempotent. That agrees with the generator's own POST rule (dangerFlags
// in cmd/gen_action_inputs treats POST as mutating, not destructive, not
// idempotent), but nothing here inherits it — these specs are written by
// hand, so the flag was decided rather than defaulted, and the decision is the
// one the four JSON creates already ship: repeating either call does not
// converge on the state the first produced, it asks Portainer for a second
// stack. The Swarm route's own vendored responses say as much, declaring 409
// "Stack name or webhook id is not unique" — a refusal, not a no-op — and the
// standalone route does not even declare that. Idempotent reaches clients as
// IdempotentHint, which invites unattended retry; a create must not.
//
// stacks.migrate is the one Destructive spec in this function, and the flag is
// a hand ruling the verb rule does not produce: POST alone gives Mutating and
// nothing else. The vendored description is what decides it — the route "will
// re-create the stack inside the target environment(endpoint) before removing
// the original stack" — so the original is gone and no field of
// stacks.stackMigratePayload describes what it held. That is the same
// criterion stacks.git_redeploy is flagged under.
//
// Its Idempotent was decided rather than left at the verb rule's false, which
// is a distinction this domain has already paid for once: git_redeploy shipped
// a hand-set Destructive next to a verb-derived Idempotent that contradicted
// it, and the two had to be reconciled in a later fix. Here they agree, and
// they agree for a reason worth stating. A repeat of this call is not a no-op
// converging on the first one's state: the stack is already in the target
// environment by then, and the route's own vendored responses declare 409 "A
// stack with the same name is already running on the target environment
// (endpoint)" — a refusal. Worse, Idempotent reaches a client as
// IdempotentHint, an invitation to retry unattended, and the one act this
// route performs that cannot be taken back is removing the original. A
// destructive action must not carry it.
//
// The rulings on Destructive were made by the scaffold task while the whole
// domain was in view and were carried here by pendingRulings in stacks_test.go,
// which starts asserting the moment an ActionSpec exists;
// TestUnit_DangerFlags_MatchThisDomainsRulings now pins all three flags for all
// three actions, and the last pendingRulings entry moved out with this commit.
//
// Declared through toolutil.WithNarrative with the vendored summary and
// description as the literal Title and Description, exactly like the
// twenty-two in actions.go. The literals are what a regeneration would write
// and what cmd/audit_spec_drift compares against; the narrative case is what a
// model actually reads, and WithNarrative is what records the difference as a
// deliberate override rather than as drift. The two create routes arrive
// carrying the seven-way-identical description their JSON siblings carry
// ("Deploy a new stack into a Docker environment specified via the environment
// identifier."), so without a case they would collide outright —
// TestUnit_Narrative_GivesEveryActionADistinctTitleAndDescription is what says
// so.
//
// Edition CE for all three: /stacks/create/standalone/file,
// /stacks/create/swarm/file and /stacks/{id}/migrate are declared in
// api/specs/ce-2.44.0.json as well as in the Business Edition document. The
// two create routes' multipart schemas are byte-identical across the two
// (internal/apiversion/applicability_gen.go carries both routes for both
// editions, from 2.27.9 with no upper bound); migrate's are not — the Business
// Edition copy of stacks.stackMigratePayload adds IsHelm and Namespace, which
// is a difference in shape rather than in whether the route exists, and
// inputs.go carries it as an `edition:"EE"` tag on those two fields.
func handWrittenSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_docker_standalone_file", Domain: "stacks", OperationID: "StackCreateDockerStandaloneFile",
			Title:       "Deploy a new compose stack from a file",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateDockerStandaloneFile,
			Input:       stackCreateDockerStandaloneFileInput{},
		}, narrative("StackCreateDockerStandaloneFile")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.create_docker_swarm_file", Domain: "stacks", OperationID: "StackCreateDockerSwarmFile",
			Title:       "Deploy a new swarm stack from a file",
			Description: "Deploy a new stack into a Docker environment specified via the environment identifier.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     stackCreateDockerSwarmFile,
			Input:       stackCreateDockerSwarmFileInput{},
		}, narrative("StackCreateDockerSwarmFile")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "stacks.migrate", Domain: "stacks", OperationID: "StackMigrate",
			Title:       "Migrate a stack to another environment(endpoint)",
			Description: "Migrate a stack from an environment(endpoint) to another environment(endpoint). It will re-create the stack inside the target environment(endpoint) before removing the original stack.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Handler:     stackMigrate,
			Input:       stackMigrateInput{},
		}, narrative("StackMigrate")),
	}
}

// narrative supplies the Title and Description overrides for operations
// whose own summary and description in the vendored specification do not say
// enough on their own. toolutil.WithNarrative applies it to every action in
// this domain, generated or hand-written, so that no action can acquire a
// literal Title/Description assignment that drifts from the spec unnoticed —
// see docs/domain-wave-checklist.md.
//
// Every one of the twenty-two generated operations gets a case, which is
// more than the scaffold run asked for and is deliberate. The run named
// seven of them — StackImagesStatus, EdgeStackWebhookInvoke,
// StacksWebhookInvoke and StackAssociate have no description beyond stripped
// boilerplate, so Description falls back to repeating Title, and StackDelete,
// StackStart and StackStop merely restate their summary — but
// descriptionQualityWarnings compares each operation against itself and
// cannot see the worse defect in this domain, which is collision between
// operations:
//
//   - Seven creates (both Docker Standalone routes, both Docker Swarm
//     routes and all three Kubernetes routes) carry the byte-identical
//     description "Deploy a new stack into a Docker environment specified
//     via the environment identifier." A model choosing between them has
//     nothing to choose on, and portainer_find_action ranks all seven the
//     same.
//   - That sentence is additionally wrong for the three Kubernetes routes:
//     they deploy into a Kubernetes environment, not a Docker one.
//   - StackDelete and StackDeleteKubernetesByName both describe themselves
//     as "Remove a stack.", though one takes an identifier and removes one
//     stack and the other takes a name.
//   - StackFileInspect has its summary and description the wrong way round:
//     the summary is the sentence ("Retrieve the content of the Stack file
//     for the specified stack") and the description is the label ("Get Stack
//     file content."), so the generated Title is longer than the generated
//     Description.
//
// Each Description below therefore states the one thing that distinguishes
// its action from its neighbours — for the creates, which orchestrator the
// stack is deployed to and where the stack file comes from.
//
// Two further conventions this hook follows, both learned from
// custom_templates:
//
//   - Where an action redacts, the Description says so. All fifteen
//     credential-bearing operations in this domain return strictly less than
//     Portainer does: redactStack drops GitConfig.Authentication whole,
//     Username and GitCredentialID included, so no sentence here may promise
//     a caller that a git username will be present.
//   - Where a danger flag is a hand ruling rather than the verb-derived
//     default, the Description says why in the model's own terms. The flags
//     themselves are set in actions.go, which carries the reasoning for a
//     reader of the code; these sentences are what a model actually reads
//     before calling.
//
// Nothing in this hook is a measurement. Unlike custom_templates', whose
// narratives correct the vendored document against a live 2.44.0 server,
// every sentence below is derived from the vendored specification, the
// generated client and this package's own code. The one live fact that does
// appear — the API-key probe recorded under StackImagesStatus — is quoted as
// this wave's reconnaissance recorded it and labelled as such.
//
// All three operations cmd/gen_action_inputs refused now have their case here
// too — StackCreateDockerStandaloneFile, StackCreateDockerSwarmFile and
// StackMigrate, whose ActionSpecs handWrittenSpecs declares — written in this
// same hook rather than as Title and Description literals on those specs,
// exactly like the twenty-two.
//
// StackMigrate's case carries a third kind of correction the two above do not.
// Its two endpoint fields are indistinguishable by name to a model reading the
// schema alone: the vendored Business Edition document gives its body
// properties no descriptions at all, so endpointId — the migration target, the
// one required field this action turns on — reaches a model as a bare integer
// with an example. Naming which of endpointId and endpointIdQuery is the
// destination is therefore not commentary but the only place that fact is
// stated to a caller.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "StackList":
		return toolutil.ActionNarrative{
			Title: "List stacks",
			Description: "Returns the stacks the caller may see: for each one its identifier, name, stack type (1 swarm, 2 compose, 3 kubernetes), the environment it is deployed to, its ownership and — for a git-backed stack — its repository configuration. " +
				"The stack file body is not included; stacks.file_inspect returns it for one stack at a time. " +
				"An administrator gets every stack, any other user only the stacks their authorizations reach, and stacks the server marks limited are never returned by this route at all. " +
				"filters narrows the result and is a JSON object encoded into a single string rather than an object parameter. EndpointID and SwarmID are the only keys this route supports, and although the specification calls the document a map of string to string, EndpointID is a NUMBER and only SwarmID is a string: send \"{\\\"EndpointID\\\":3}\" or \"{\\\"SwarmID\\\":\\\"jpofkc0i9uo9wtx1zesuk649w\\\"}\", because a quoted EndpointID is refused with 400 \"cannot unmarshal ... of type int\". " +
				"Each key also matches ONE stack type, measured: EndpointID returns only compose stacks in that environment and never a swarm stack deployed there, SwarmID returns only swarm stacks, and sending both keys returns the union of the two rather than their intersection. Omit filters entirely to see every stack. " +
				"Any git credential a stack stores is stripped before the list reaches you.",
		}
	case "StackInspect":
		return toolutil.ActionNarrative{
			Title: "Inspect a stack",
			Description: "Returns one stack's definition by identifier — the same shape as one entry of stacks.list, and likewise without the stack file body, which stacks.file_inspect returns. " +
				"Any git credential the stack stores is stripped before the result reaches you.",
		}
	case "StackFileInspect":
		return toolutil.ActionNarrative{
			Title: "Read a stack's stack file",
			Description: "Returns the file Portainer has stored for one stack — a Compose file, a Swarm stack file or a Kubernetes manifest, depending on the stack — as a single StackFileContent string. " +
				"This is a read: it returns the stored copy and changes nothing, despite sitting among actions that deploy. " +
				"On Business Edition, version selects an earlier revision Portainer kept and commitHash selects a revision by git commit; sending both uses commitHash, and sending neither returns the current content. " +
				"Refreshing a git-backed stack's stored copy from its remote is stacks.git_redeploy, which also redeploys it.",
		}
	case "StackImagesStatus":
		return toolutil.ActionNarrative{
			Title: "Check whether a stack's images are outdated",
			Description: "Reports whether the images the stack's containers run are still current for the tags they were deployed from — the data behind Portainer's out-of-date indicator — as an overall status plus per-image detail. " +
				"refresh: true recomputes that status instead of reading the cached one, which costs a registry round trip per image. " +
				"Read-only: it pulls nothing and changes nothing about the stack. " +
				"The vendored specification declares only jwt under this route's security and omits ApiKeyAuth, alone among this domain's twenty-five operations. That is a defect in the document, not a constraint: this wave's reconnaissance probed the route with an API key and got 404 for a stack that did not exist rather than 401, so the key authenticated. " +
				"Business Edition only.",
		}
	case "StackCreateDockerStandaloneString":
		return toolutil.ActionNarrative{
			Title: "Deploy a Compose stack from inline content",
			Description: "Creates and deploys a Docker Compose stack on a standalone Docker environment, with the Compose file passed inline in this call as the stackFileContent string. Nothing is uploaded: the two actions that take a real file upload are stacks.create_docker_standalone_file and stacks.create_docker_swarm_file. " +
				"endpointId names the environment, name names the stack, and env carries deployment-time environment variables as name/value pairs. " +
				"The sibling that clones the same file from a git repository instead is stacks.create_docker_standalone_repository. " +
				"Use the Swarm actions instead for a Swarm cluster and the Kubernetes ones for a Kubernetes environment: the routes are not interchangeable, and this one deploys through the standalone Docker engine. " +
				"Business Edition also accepts registries, naming the registries to pull from, and webhook, a UUID that later lets stacks.webhook_invoke redeploy this stack. " +
				"Answers with the created stack, git credentials stripped.",
		}
	case "StackCreateDockerStandaloneRepository":
		return toolutil.ActionNarrative{
			Title: "Deploy a Compose stack from a git repository",
			Description: "Creates and deploys a Docker Compose stack on a standalone Docker environment, cloning the Compose file from a git repository; stacks.git_redeploy is what later re-pulls and redeploys it. " +
				"endpointId names the environment and name the stack. composeFile is the path to the file inside the repository, and additionalFiles names further stack files when the deployment uses more than one. " +
				"autoUpdate configures GitOps polling or a webhook so Portainer redeploys the stack when the repository moves. " +
				"sourceId references a stored git source and, when set, supersedes the inline repositoryUrl/repositoryUsername/repositoryPassword fields, which the specification marks deprecated in its favour. " +
				"The siblings take the file inline (stacks.create_docker_standalone_string) or from an upload. " +
				"Answers with the created stack; the git credential you send here is stripped from that answer and cannot be read back through any action in this domain.",
		}
	case "StackCreateDockerStandaloneFile":
		return toolutil.ActionNarrative{
			Title: "Deploy a Compose stack from an uploaded file",
			Description: "Creates and deploys a Docker Compose stack on a standalone Docker environment from a Compose file UPLOADED as multipart/form-data — the file's content travels in this call as the file string, and Portainer stores it as the stack's own file. " +
				"This and stacks.create_docker_swarm_file are the only two actions in this domain that upload anything; the siblings pass the same content inline as JSON (stacks.create_docker_standalone_string) or clone it from a git repository (stacks.create_docker_standalone_repository), and a stack created here has no repository, so stacks.git_redeploy cannot refresh it. " +
				"endpointId names the environment and name the stack. env carries deployment-time variables and, unlike the JSON siblings' list of name/value objects, is a JSON-encoded STRING on this route: send \"[{\\\"name\\\":\\\"KEY\\\",\\\"value\\\":\\\"v\\\"}]\", not an array. " +
				"The vendored multipart schema lists only Name as required, so file is published optional although this is the upload route; send it. " +
				"This route has none of the Business Edition extras its JSON siblings take — no registries and no webhook — so a stack that later needs stacks.webhook_invoke must be created by one of those instead. " +
				"Answers with the created stack, git credentials stripped.",
		}
	case "StackCreateDockerSwarmString":
		return toolutil.ActionNarrative{
			Title: "Deploy a Swarm stack from inline content",
			Description: "Creates and deploys a Docker Swarm stack, with the stack file passed inline in this call as the stackFileContent string. Nothing is uploaded: stacks.create_docker_swarm_file is the action that takes a real file upload. " +
				"Differs from stacks.create_docker_standalone_string in the orchestrator and in one required field: swarmId, the Swarm cluster identifier, which the standalone routes have no equivalent of. " +
				"endpointId names the environment, name the stack, env the deployment-time variables. " +
				"Business Edition also accepts registries and a webhook UUID for later redeployment through stacks.webhook_invoke. " +
				"Answers with the created stack, git credentials stripped.",
		}
	case "StackCreateDockerSwarmRepository":
		return toolutil.ActionNarrative{
			Title: "Deploy a Swarm stack from a git repository",
			Description: "Creates and deploys a Docker Swarm stack, cloning the stack file from a git repository; stacks.git_redeploy is what later re-pulls and redeploys it. " +
				"Differs from stacks.create_docker_standalone_repository in the orchestrator and in requiring swarmId, the Swarm cluster identifier. " +
				"composeFile is the path to the stack file inside the repository, additionalFiles names further files, and autoUpdate configures GitOps polling or a webhook. " +
				"sourceId references a stored git source and supersedes the inline repository fields, which the specification marks deprecated in its favour. " +
				"Answers with the created stack; the git credential you send here is stripped from that answer.",
		}
	case "StackCreateDockerSwarmFile":
		return toolutil.ActionNarrative{
			Title: "Deploy a Swarm stack from an uploaded file",
			Description: "Creates and deploys a Docker Swarm stack from a stack file UPLOADED as multipart/form-data — the file's content travels in this call as the file string. " +
				"Differs from stacks.create_docker_standalone_file in the orchestrator and in accepting swarmId, the Swarm cluster identifier, as a string. " +
				"endpointId names the environment, name the stack, and env carries deployment-time variables as a JSON-encoded STRING rather than the list of name/value objects the JSON siblings take. " +
				"This route's vendored multipart schema declares NO required fields at all — not even name — so nothing but endpointId is enforced before the call; that is the document's doing, not a statement that Portainer will accept a nameless stack, so send name anyway. A name already in use is answered 409 \"Stack name or webhook id is not unique\", which the standalone route does not even declare. " +
				"A stack created here has no repository, so stacks.git_redeploy cannot refresh it, and the route takes none of the Business Edition extras (registries, webhook) its JSON siblings do. " +
				"Answers with the created stack, git credentials stripped.",
		}
	case "StackCreateKubernetesFile":
		return toolutil.ActionNarrative{
			Title: "Deploy a Kubernetes stack from an inline manifest",
			Description: "Creates and deploys a Kubernetes stack from a manifest passed inline in this call as stackFileContent — a string, not an uploaded file, whatever the operationId StackCreateKubernetesFile suggests: the route is POST /stacks/create/kubernetes/string and it has no multipart form at all. " +
				"This action is named stacks.create_kubernetes_string for that reason, so that it reads as the sibling of stacks.create_docker_standalone_string rather than of the two genuine file-upload actions this domain also has. " +
				"endpointId names the Kubernetes environment, namespace the target namespace and stackName the stack. composeFormat: true declares the content to be Docker Compose for Portainer to convert rather than a Kubernetes manifest. " +
				"The vendored description for this route claims it deploys \"into a Docker environment\", which it shares byte for byte with six other create routes; it deploys into a Kubernetes environment. " +
				"The siblings clone the manifest from a git repository (stacks.create_kubernetes_git) or fetch it from a URL (stacks.create_kubernetes_url).",
		}
	case "StackCreateKubernetesGit":
		return toolutil.ActionNarrative{
			Title: "Deploy a Kubernetes stack from a git repository",
			Description: "Creates and deploys a Kubernetes stack, cloning the manifest — or a Helm chart — from a git repository; stacks.git_redeploy is what later re-pulls and redeploys it. " +
				"endpointId names the Kubernetes environment, namespace the target namespace, stackName the stack, and manifestFile the path to the manifest inside the repository. " +
				"helmChartPath and helmValuesFiles (Business Edition) switch the deployment to a Helm chart held in the same repository. autoUpdate configures GitOps polling or a webhook. " +
				"sourceId references a stored git source and supersedes the inline repository fields, which the specification marks deprecated in its favour. " +
				"The vendored description for this route claims it deploys \"into a Docker environment\"; it deploys into a Kubernetes environment. " +
				"Answers with the created stack; the git credential you send here is stripped from that answer.",
		}
	case "StackCreateKubernetesUrl":
		return toolutil.ActionNarrative{
			Title: "Deploy a Kubernetes stack from a manifest URL",
			Description: "Creates and deploys a Kubernetes stack from a manifest Portainer fetches over HTTP at manifestUrl — a plain URL fetch with no git clone, no credential and nothing stored to re-pull from, so a stack created this way cannot be refreshed by stacks.git_redeploy. " +
				"endpointId names the Kubernetes environment, namespace the target namespace and stackName the stack. composeFormat: true declares the fetched content to be Docker Compose rather than a Kubernetes manifest. " +
				"The vendored description for this route claims it deploys \"into a Docker environment\"; it deploys into a Kubernetes environment. " +
				"Use stacks.create_kubernetes_git when the manifest lives in a repository and should stay in sync with it.",
		}
	case "StackUpdate":
		return toolutil.ActionNarrative{
			Title: "Replace a stack's file and redeploy it",
			Description: "Replaces one stack's stored stack file with stackFileContent and its variables with env, then redeploys the stack in the environment named by the required endpointId. " +
				"The specification's \"only for file based stacks\" matters: for a git-backed stack the stored file belongs to the remote, and stacks.git_redeploy is the route that refreshes it. " +
				"env replaces the whole variable list rather than merging into it, so send the variables the stack should end up with, not only the ones that change — read the current set with stacks.inspect first. " +
				"prune removes services the new file no longer names (Swarm only). repullImageAndRedeploy forces a fresh image pull; pullImage is its spelling before 2.36 and is deprecated. " +
				"rollbackTo (Business Edition) restores an earlier stack file version, and Portainer supports rolling back to the previous one only. " +
				"Answers with the updated stack, git credentials stripped.",
		}
	case "StackUpdateGit":
		return toolutil.ActionNarrative{
			Title: "Change a stack's git configuration",
			Description: "Changes where and how a git-backed stack pulls from — reference name, config file path, the autoUpdate GitOps schedule or webhook, Helm chart path and values files — without pulling or redeploying anything; stacks.git_redeploy is what does that. " +
				"sourceId references a stored git source and, when set, the URL, authentication and TLS settings come from that source and the inline repository fields are ignored. " +
				"repositoryAuthentication with a non-empty repositoryPassword REPLACES the stored credential; leaving repositoryPassword blank keeps whatever is already stored. " +
				"Answers with the stack's git configuration, and the credential is stripped from that answer — this action cannot be used to read back a password, including one it has just set. " +
				"It is the one action in this domain whose answer is a git-configuration summary rather than a full stack object, so do not expect the fields stacks.inspect returns.",
		}
	case "StackGitRedeploy":
		return toolutil.ActionNarrative{
			Title: "Pull from git and redeploy a stack, discarding what is deployed",
			Description: "Pulls a git-backed stack's file from its repository and REDEPLOYS the stack from what the remote holds now, replacing what is deployed. " +
				"This is a write with no undo, which is why it is flagged destructive although its summary (\"Redeploy a stack\") and its HTTP verb both read as an ordinary update: the request names no revision, so what arrives is whatever the configured reference points at when the call is made, and Portainer keeps no copy of the replaced deployment to restore. " +
				"For the same reason it is not marked idempotent and must not be retried automatically: repeating the call is not a no-op, it is a second deployment of whatever the remote holds by then. " +
				"prune additionally removes services the new file no longer names. repullImageAndRedeploy forces a fresh image pull; pullImage is its pre-2.36 spelling and is deprecated. " +
				"repositoryAuthentication with a non-empty repositoryPassword also replaces the stack's stored git credential as a side effect of redeploying. " +
				"endpointId is only for a stack created before Portainer 1.18.0 that has no environment recorded against it. " +
				"To read the stored stack file without redeploying, use stacks.file_inspect. Answers with the redeployed stack, git credentials stripped.",
		}
	case "StackStart":
		return toolutil.ActionNarrative{
			Title: "Start a stopped stack",
			Description: "Starts a stack that stacks.stop previously stopped, recreating its services in the environment named by the required endpointId from the stack file already stored. " +
				"It deploys nothing new and pulls nothing: creating a stack is one of the stacks.create_* actions and refreshing one from its repository is stacks.git_redeploy. " +
				"Answers with the started stack, git credentials stripped.",
		}
	case "StackStop":
		return toolutil.ActionNarrative{
			Title: "Stop a running stack without deleting it",
			Description: "Stops a running stack's services while keeping the stack itself: its identifier, definition and stored stack file all survive, and stacks.start brings it back. " +
				"endpointId is required. This is the reversible counterpart of stacks.delete, which removes the stack outright. " +
				"Answers with the stopped stack, git credentials stripped.",
		}
	case "StackDelete":
		return toolutil.ActionNarrative{
			Title: "Delete a stack and undeploy it",
			Description: "Removes one stack by identifier and undeploys what it created in the environment named by the required endpointId. " +
				"There is no undo, and recreating the stack later redeploys the stack file, not the state its containers held. Use stacks.stop to halt a stack you intend to start again. " +
				"external: true is for an external Swarm stack Portainer did not itself create; the specification limits that flag to Swarm. " +
				"removeVolumes (Business Edition, Docker Standalone only) also deletes the named volumes the stack file declares and the anonymous volumes attached to its containers — without it, the data in those volumes outlives the stack. " +
				"stacks.delete_kubernetes_by_name is the by-name route and can match more than one stack.",
		}
	case "StackDeleteKubernetesByName":
		return toolutil.ActionNarrative{
			Title: "Delete Kubernetes stacks by name",
			Description: "Removes Kubernetes stacks by NAME rather than by identifier, in the environment named by the required endpointId. " +
				"The specification's own summary is plural (\"Remove Kubernetes stacks by name\"), so treat this as capable of removing every stack carrying that name in that environment, not exactly one — stacks.delete is the route that removes exactly the one stack an identifier names. " +
				"No undo, and the same 'the containers' state does not come back' caveat as stacks.delete applies. " +
				"This route has no removeVolumes flag.",
		}
	case "StackAssociate":
		return toolutil.ActionNarrative{
			Title: "Re-attach an orphaned stack to an environment",
			Description: "Points a stack record that has lost its environment — an orphan, typically left behind when the environment it was deployed to was removed from Portainer — at a new environment, so Portainer manages it again. " +
				"It re-parents the record only: the stack file is untouched, nothing is deployed and no container is created or destroyed. " +
				"All three query parameters are required and none has a default. endpointId is the environment to attach to; orphanedRunning states whether the stack is in fact still running out there; swarmId is the Swarm cluster identifier. " +
				"Note that swarmId is declared an integer on this route alone — everywhere else in this API a Swarm cluster identifier is a string, as in stacks.create_docker_swarm_string's swarmId — and this action publishes it as the vendored document declares it. " +
				"Administrator only. Answers with the re-parented stack, git credentials stripped.",
		}
	case "StackConvert":
		return toolutil.ActionNarrative{
			Title: "Preview a Docker stack converted to Kubernetes or Helm",
			Description: "Converts an existing Docker Compose or Docker Swarm stack to Kubernetes manifests or a Helm chart and RETURNS THE RESULT FOR PREVIEW. " +
				"It deploys nothing, creates no stack and leaves the source stack exactly as it was — which is why it is not flagged destructive despite being a POST that the word \"convert\" makes sound irreversible. " +
				"targetFormat is required: \"kubernetes\" for manifests, \"helm\" for a chart. namespace names the Kubernetes namespace the generated manifests should target. " +
				"The answer is a map of generated file name to file content. Deploying it is a separate call to stacks.create_kubernetes_string or stacks.create_kubernetes_git. " +
				"Business Edition only.",
		}
	case "StacksWebhookInvoke":
		return toolutil.ActionNarrative{
			Title: "Trigger a stack's git auto-update webhook",
			Description: "Invokes one stack's GitOps webhook by its UUID, which makes Portainer pull from that stack's repository and redeploy it — the same replacement stacks.git_redeploy performs, reached through the webhook door instead. " +
				"Flagged destructive for the same reason as stacks.git_redeploy, and more so: this request names neither a revision nor even a stack, only the webhook UUID, so the caller can see nothing of what is about to be discarded. " +
				"Only a stack whose autoUpdate webhook UUID has been configured (by one of the stacks.create_* actions, stacks.update or stacks.update_git) has one; a stack without it answers 409, as does a stack whose deployment is already running. " +
				"The route declares no security requirement at all — it is public — so that UUID is the only thing protecting it: treat it as a secret and never place it where the answer to another call could carry it. " +
				"Answers with no body, only a status. " +
				"Registered as a Business Edition action because the two vendored documents give this one route two different operationIds — StacksWebhookInvoke in Business Edition, WebhookInvoke in Community Edition — and the catalog is built from the Business Edition document; the route itself exists in both.",
		}
	case "StackMigrate":
		return toolutil.ActionNarrative{
			Title: "Move a stack to another environment, destroying the original",
			Description: "Re-creates one stack in a different environment and then REMOVES the original: after this call the stack no longer exists where it was, and nothing here records what it held. " +
				"There is no undo and no reverse action — migrating back is another call to this action, which re-creates and destroys again. " +
				"id names the stack to move. endpointId is the environment to move it TO and is required. " +
				"endpointIdQuery is a different field for a different job and is almost never what you want: it repairs a stack created before Portainer 1.18.0 that has no environment recorded against it, by stating which environment it is currently deployed to. Both are called endpointId in the HTTP API — one in the body, one in the query string — and this action publishes the query one under the longer name so the two cannot be confused. Setting endpointIdQuery does not change where the stack is migrated to. " +
				"name renames the stack as it is re-created, and swarmId names the target Swarm cluster, which a Swarm stack needs because the cluster identifier does not travel with it. " +
				"Business Edition additionally accepts namespace, the target Kubernetes namespace, and isHelm for a Helm-deployed stack; neither field exists against a Community Edition server. " +
				"Not marked idempotent, deliberately: a second call is not a no-op. By then the stack is already in the target environment, and Portainer answers 409 \"A stack with the same name is already running on the target environment(endpoint)\" — so this must not be retried unattended. " +
				"Answers with the migrated stack, git credentials stripped.",
		}
	case "EdgeStackWebhookInvoke":
		return toolutil.ActionNarrative{
			Title: "Trigger an Edge stack's git auto-update webhook",
			Description: "Invokes one EDGE stack's GitOps webhook by its UUID: Portainer pulls from the Edge stack's repository and redeploys it to the edge environments that stack targets. " +
				"Same act as stacks.webhook_invoke on a different resource — POST /edge_stacks/webhooks/{webhookID} rather than /stacks/webhooks/{webhookID} — and it is in the stacks domain rather than a future edge_stacks one only because the vendored specification tags it \"stacks\". " +
				"Flagged destructive for the same reason: the caller names no revision, and what the remote holds now replaces what the edge environments are running. " +
				"Public, like its sibling: the webhook UUID is the only secret protecting the route. Answers with no body, only a status. " +
				"Business Edition only — /edge_stacks/webhooks/{webhookID} is absent from the Community Edition document entirely.",
		}
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
