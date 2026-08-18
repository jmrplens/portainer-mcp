package stacks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func ptr[T any](v T) *T { return &v }

func clientFor(t *testing.T, body []byte) *portainer.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	return c
}

func findSpec(t *testing.T, name string) toolutil.ActionSpec {
	t.Helper()
	for _, s := range Specs() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("action %q not declared", name)
	return toolutil.ActionSpec{}
}

func find(t *testing.T, name string) toolutil.Handler {
	t.Helper()
	return findSpec(t, name).Handler
}

func specByOperationID(t *testing.T, operationID string) toolutil.ActionSpec {
	t.Helper()
	for _, s := range Specs() {
		if s.OperationID == operationID {
			return s
		}
	}
	t.Fatalf("no action declared for operationId %q", operationID)
	return toolutil.ActionSpec{}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return body
}

// gitCredential is the sub-object every wrapper in this domain has to drop,
// built once so the fixtures below cannot disagree about what a leak looks
// like.
func gitCredential() *apigen.GithubComPortainerPortainerApiGitTypesGitAuthentication {
	return &apigen.GithubComPortainerPortainerApiGitTypesGitAuthentication{
		Username:        ptr("deploy-bot"),
		Password:        ptr("hunter2"),
		GitCredentialID: ptr(7),
	}
}

func gitConfig() *apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig {
	return &apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig{
		URL:            ptr("https://git.example.com/team/app.git"),
		ReferenceName:  ptr("refs/heads/main"),
		ConfigFilePath: ptr("docker-compose.yml"),
		Authentication: gitCredential(),
	}
}

// gitBackedStack is a stack whose git configuration carries a live
// credential, built from the real generated response type rather than from a
// JSON literal.
//
// The concrete type is the whole point. The generated guard in
// redaction_test.go drives every wrapper through reflect.New on the
// wrapper's *own declared parameter type*, so a wrapper declared
// func redactX(r any) any { return r } satisfies it — and satisfies
// `make audit-spec-drift` too — while redacting nothing at all
// (docs/api-divergences.md §9.4). A fixture that only type-checks against
// `any` proves nothing about PortainereeStack.GitConfig.Authentication
// actually being dropped; this one will not compile unless the wrapper takes
// the type the route really answers with.
func gitBackedStack() apigen.PortainereeStack {
	return apigen.PortainereeStack{
		Id:         ptr(3),
		Name:       ptr("nginx-stack"),
		EndpointId: ptr(1),
		GitConfig:  gitConfig(),
	}
}

// gitBackedStackResponse is gitBackedStack for apigen.StacksStackResponse,
// the success type of StackUpdateGit and of no other operation in this
// domain.
//
// It exists because a redactor that works for the thirteen
// PortainereeStack-shaped wrappers and not for the fourteenth type would
// otherwise pass every test in this file: redactStackUpdateGit is the one
// wrapper that does not route through redactStack, and POST /stacks/{id}/git
// — the route that sets a stack's git credential — is the last place a leak
// should be allowed to hide.
func gitBackedStackResponse() apigen.StacksStackResponse {
	return apigen.StacksStackResponse{
		Id:        ptr(3),
		Name:      ptr("nginx-stack"),
		GitConfig: gitConfig(),
	}
}

// assertNoCredential asserts that none of the three parts of a git
// credential survived into what a model would receive, and that redaction
// removed the credential and nothing else.
//
// Username and GitCredentialID are asserted absent alongside Password
// deliberately: redact.RepoConfig drops the whole Authentication sub-object,
// so these actions return strictly less than Portainer does, and asserting
// only Password would let a field-by-field redactor that kept Username pass.
// No narrative in this domain promises a caller a git username.
func assertNoCredential(t *testing.T, action string, out any, mustSurvive ...string) {
	t.Helper()
	encoded := string(mustMarshal(t, out))
	if strings.Contains(encoded, "hunter2") {
		t.Errorf("%s returned the git password to the caller: %s", action, encoded)
	}
	if strings.Contains(encoded, "deploy-bot") {
		t.Errorf("%s returned the git username to the caller; Authentication must be dropped whole: %s", action, encoded)
	}
	if strings.Contains(encoded, `"GitCredentialID"`) {
		t.Errorf("%s returned the git credential identifier to the caller: %s", action, encoded)
	}
	for _, want := range mustSurvive {
		if !strings.Contains(encoded, want) {
			t.Errorf("%s: redaction removed more than the credential, %q is gone: %s", action, want, encoded)
		}
	}
}

// TestUnit_HandlerWithGitCredentialInResponse_ReturnsNoCredential runs each
// of this domain's twelve credential-bearing generated handlers end to end
// against a response carrying a populated GitConfig.Authentication, and
// asserts nothing of it survives into the marshalled result.
//
// This is the discriminating test the generated redaction_test.go is not.
// That file's fixture is built by reflection from whatever type the wrapper
// happens to declare, so it cannot tell a wrapper that redacts a
// PortainereeStack apart from one that declares `any` and redacts nothing.
// Here the fixture is a real PortainereeStack (or a real
// StacksStackResponse, or a real slice of stacks) and the assertion is made
// on the bytes a model would actually receive.
func TestUnit_HandlerWithGitCredentialInResponse_ReturnsNoCredential(t *testing.T) {
	t.Parallel()

	single := mustMarshal(t, gitBackedStack())
	// Two entries, only the second credential-bearing: a list redactor that
	// forgot its per-element loop, or that redacted only the first element,
	// fails here and nowhere else in this file.
	list := mustMarshal(t, []apigen.PortainereeStack{
		{Id: ptr(1), Name: ptr("plain-stack")},
		gitBackedStack(),
	})
	updateGit := mustMarshal(t, gitBackedStackResponse())

	tests := []struct {
		name        string
		action      string
		body        []byte
		input       string
		mustSurvive []string
	}{
		{
			name:   "inspect",
			action: "stacks.inspect",
			body:   single,
			input:  `{"id":3}`,
		},
		{
			name:        "list",
			action:      "stacks.list",
			body:        list,
			input:       `{}`,
			mustSurvive: []string{"plain-stack"},
		},
		{
			name:   "associate",
			action: "stacks.associate",
			body:   single,
			input:  `{"id":3,"endpointId":1,"swarmId":0,"orphanedRunning":true}`,
		},
		{
			name:   "create_docker_standalone_repository",
			action: "stacks.create_docker_standalone_repository",
			body:   single,
			input:  `{"endpointId":1,"name":"nginx-stack","repositoryUrl":"https://git.example.com/team/app.git"}`,
		},
		{
			name:   "create_docker_standalone_string",
			action: "stacks.create_docker_standalone_string",
			body:   single,
			input:  `{"endpointId":1,"name":"nginx-stack","stackFileContent":"services: {}"}`,
		},
		{
			name:   "create_docker_swarm_repository",
			action: "stacks.create_docker_swarm_repository",
			body:   single,
			input:  `{"endpointId":1,"name":"nginx-stack","swarmId":"jpofkc0i9uo9wtx1zesuk649w","repositoryUrl":"https://git.example.com/team/app.git"}`,
		},
		{
			name:   "create_docker_swarm_string",
			action: "stacks.create_docker_swarm_string",
			body:   single,
			input:  `{"endpointId":1,"name":"nginx-stack","swarmId":"jpofkc0i9uo9wtx1zesuk649w","stackFileContent":"services: {}"}`,
		},
		{
			name:   "git_redeploy",
			action: "stacks.git_redeploy",
			body:   single,
			input:  `{"id":3}`,
		},
		{
			name:   "start",
			action: "stacks.start",
			body:   single,
			input:  `{"id":3,"endpointId":1}`,
		},
		{
			name:   "stop",
			action: "stacks.stop",
			body:   single,
			input:  `{"id":3,"endpointId":1}`,
		},
		{
			name:   "update",
			action: "stacks.update",
			body:   single,
			input:  `{"id":3,"endpointId":1,"stackFileContent":"services: {}"}`,
		},
		{
			// The one wrapper whose parameter is not a PortainereeStack.
			name:   "update_git",
			action: "stacks.update_git",
			body:   updateGit,
			input:  `{"id":3}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := clientFor(t, tt.body)

			out, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input))
			if err != nil {
				t.Fatalf("%s: handler error = %v", tt.action, err)
			}
			// The stack name and the repository URL are what the action
			// exists to return; a redactor that dropped GitConfig whole, or
			// returned nil, would pass every credential assertion above.
			survive := append([]string{"nginx-stack", "https://git.example.com/team/app.git"}, tt.mustSurvive...)
			assertNoCredential(t, tt.action, out, survive...)
		})
	}
}

// TestUnit_ListWithACredentialInOneEntry_LeavesTheOtherEntriesIntact pins
// what the table above cannot say on its own: the list handler returns both
// elements, not just the redacted one. A redactStackList that dropped every
// element it had to scrub — returning a shorter slice — would pass every
// credential assertion above.
func TestUnit_ListWithACredentialInOneEntry_LeavesTheOtherEntriesIntact(t *testing.T) {
	t.Parallel()
	c := clientFor(t, mustMarshal(t, []apigen.PortainereeStack{
		{Id: ptr(1), Name: ptr("plain-stack")},
		gitBackedStack(),
	}))

	out, err := find(t, "stacks.list")(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	stacks, ok := out.(*[]apigen.PortainereeStack)
	if !ok {
		t.Fatalf("handler returned %T, want *[]apigen.PortainereeStack", out)
	}
	if len(*stacks) != 2 {
		t.Fatalf("handler returned %d stack(s), want both", len(*stacks))
	}
	if (*stacks)[0].GitConfig != nil {
		t.Error("the credential-free entry acquired a GitConfig it did not have")
	}
	if (*stacks)[1].GitConfig == nil {
		t.Fatal("the git-backed entry lost its GitConfig; redaction removed more than the credential")
	}
	if (*stacks)[1].GitConfig.Authentication != nil {
		t.Errorf("the git-backed entry's Authentication = %+v, want the sub-object dropped and the rest kept", (*stacks)[1].GitConfig.Authentication)
	}
	if (*stacks)[1].GitConfig.URL == nil {
		t.Error("the git-backed entry lost its repository URL, which is not a credential")
	}
}

// TestUnit_UpdateGit_RedactsThroughItsOwnResponseType asserts on the typed
// value rather than on marshalled bytes for the one operation whose success
// type is not PortainereeStack.
//
// redactStackUpdateGit is the only wrapper in this domain that does not route
// through redactStack, so a redactor that worked for thirteen wrappers and
// not for the fourteenth would leak here and nowhere else — on POST
// /stacks/{id}/git, which is the route that *sets* a stack's git credential.
func TestUnit_UpdateGit_RedactsThroughItsOwnResponseType(t *testing.T) {
	t.Parallel()
	c := clientFor(t, mustMarshal(t, gitBackedStackResponse()))

	out, err := find(t, "stacks.update_git")(context.Background(), c, json.RawMessage(`{"id":3}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	resp, ok := out.(*apigen.StacksStackResponse)
	if !ok {
		t.Fatalf("handler returned %T, want *apigen.StacksStackResponse", out)
	}
	if resp.GitConfig == nil {
		t.Fatal("redaction removed the whole GitConfig, not only the credential")
	}
	if resp.GitConfig.Authentication != nil {
		t.Errorf("Authentication = %+v, want nil", resp.GitConfig.Authentication)
	}
	if resp.GitConfig.URL == nil || *resp.GitConfig.URL != "https://git.example.com/team/app.git" {
		t.Errorf("GitConfig.URL = %v, want the repository URL kept", resp.GitConfig.URL)
	}
}

// TestUnit_RedactionDoesNotMutateTheDecodedResponse proves the wrappers copy
// rather than scrub in place. Nothing else in this package would notice:
// every other assertion reads only what the wrapper returned, so a wrapper
// that blanked the caller's own struct and returned it would pass them all.
func TestUnit_RedactionDoesNotMutateTheDecodedResponse(t *testing.T) {
	t.Parallel()
	original := gitBackedStack()

	if redacted := redactStack(&original); redacted.GitConfig.Authentication != nil {
		t.Error("redactStack returned a value that still carries Authentication")
	}
	if original.GitConfig.Authentication == nil {
		t.Error("redactStack scrubbed its argument in place instead of copying it")
	}
}

// TestUnit_Specs_AreAllValid guards the invariants ActionSpec.Validate
// enforces, including Destructive implying Mutating — which every one of
// this domain's hand rulings below relies on.
func TestUnit_Specs_AreAllValid(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}
}

// generatedOperations is every operationId this domain ships today: the
// twenty-two cmd/gen_action_inputs generated. The three it refused —
// StackCreateDockerStandaloneFile, StackCreateDockerSwarmFile and
// StackMigrate — are absent on purpose; the tasks that hand-write them add
// them here in the same commit as their ActionSpecs.
var generatedOperations = []string{
	"EdgeStackWebhookInvoke",
	"StackAssociate",
	"StackConvert",
	"StackCreateDockerStandaloneRepository",
	"StackCreateDockerStandaloneString",
	"StackCreateDockerSwarmRepository",
	"StackCreateDockerSwarmString",
	"StackCreateKubernetesFile",
	"StackCreateKubernetesGit",
	"StackCreateKubernetesUrl",
	"StackDelete",
	"StackDeleteKubernetesByName",
	"StackFileInspect",
	"StackGitRedeploy",
	"StackImagesStatus",
	"StackInspect",
	"StackList",
	"StackStart",
	"StackStop",
	"StackUpdate",
	"StackUpdateGit",
	"StacksWebhookInvoke",
}

// TestUnit_Specs_CoverExactlyTheOperationsThisDomainShips is the floor the
// three tests after it stand on: each of them asserts something about "every
// action", and every one of them would pass vacuously against a Specs() that
// had quietly lost half its entries.
func TestUnit_Specs_CoverExactlyTheOperationsThisDomainShips(t *testing.T) {
	t.Parallel()
	got := make([]string, 0, len(Specs()))
	for _, s := range Specs() {
		got = append(got, s.OperationID)
	}
	sort.Strings(got)
	want := append([]string(nil), generatedOperations...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Specs() covers\n  %v\nwant\n  %v", got, want)
	}
}

// TestUnit_Narrative_OverridesTitleAndDescriptionAwayFromTheSpecText pins
// the narrative hook itself, per operation. Nothing else does: `go test
// ./...`, `make audit-spec-drift` and `make audit-1to1` all stay green when
// the whole switch statement in narrative() is deleted, because none of them
// reads what a Description says — only that one exists. Without this test
// this domain silently reverts to the vendored specification's own prose, in
// which seven distinct create actions share one byte-identical description
// that additionally names the wrong orchestrator for three of them, two
// deletes share another, and four actions have no description at all.
//
// Asserted as "an override is in place" rather than as literal strings:
// improving one of these sentences is expected, ordinary work (see
// toolutil.WithNarrative's doc comment), and pinning the text would tax
// every such improvement for no gain against the failure this catches — the
// override disappearing.
func TestUnit_Narrative_OverridesTitleAndDescriptionAwayFromTheSpecText(t *testing.T) {
	t.Parallel()
	for _, id := range generatedOperations {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			spec := specByOperationID(t, id)
			if !spec.TitleOverridden {
				t.Errorf("%s: Title is the vendored specification's own summary; its narrative override is missing", id)
			}
			if !spec.DescriptionOverridden {
				t.Errorf("%s: Description is the vendored specification's own text; its narrative override is missing", id)
			}
			if spec.Description == spec.Title {
				t.Errorf("%s: Description == Title (%q), which is the generator's boilerplate fallback", id, spec.Title)
			}
		})
	}
}

// TestUnit_Narrative_GivesEveryActionADistinctTitleAndDescription is the
// reason this domain overrides all twenty-two rather than the seven the
// scaffold run named. descriptionQualityWarnings compares each operation
// against itself and so cannot see the worse defect here: seven create
// operations carry the byte-identical description "Deploy a new stack into a
// Docker environment specified via the environment identifier.", and
// StackDelete and StackDeleteKubernetesByName both carry "Remove a stack.".
// Without an override, portainer_find_action ranks those seven identically
// and a model has nothing to choose on.
func TestUnit_Narrative_GivesEveryActionADistinctTitleAndDescription(t *testing.T) {
	t.Parallel()
	titles := map[string]string{}
	descriptions := map[string]string{}
	for _, s := range Specs() {
		if prior, clash := titles[s.Title]; clash {
			t.Errorf("%s and %s share the Title %q; a model cannot tell them apart", prior, s.Name, s.Title)
		}
		titles[s.Title] = s.Name
		if prior, clash := descriptions[s.Description]; clash {
			t.Errorf("%s and %s share the Description %q; a model cannot tell them apart", prior, s.Name, s.Description)
		}
		descriptions[s.Description] = s.Name
	}
}

// TestUnit_DangerFlags_MatchThisDomainsRulings pins every Mutating and
// Destructive flag this domain publishes, generated and hand-ruled alike.
//
// The flags live in generated code, and `make scaffold-domain FORCE=1`
// rewrites actions.go from the specification: a hand ruling is one verb-rule
// re-run away from being silently reverted, and the previous stage proved
// that matters. Every row is therefore listed, not only the overridden ones
// — a test that pinned only the hand rulings would go quiet the moment a
// regeneration changed one of the others.
//
// Three of the five Destructive rows are hand rulings the generator did not
// prompt: suspectDangerMismatch's keyword list matches nothing in this
// domain, so the scaffold run printed no verb-mismatch warning for stacks at
// all. See actions.go for the reasoning behind each.
func TestUnit_DangerFlags_MatchThisDomainsRulings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		action      string
		mutating    bool
		destructive bool
	}{
		// Reads.
		{name: "list only reads", action: "stacks.list", mutating: false, destructive: false},
		{name: "inspect only reads", action: "stacks.inspect", mutating: false, destructive: false},
		{name: "file_inspect only reads, despite the noun-like name", action: "stacks.file_inspect", mutating: false, destructive: false},
		{name: "images_status only reads", action: "stacks.images_status", mutating: false, destructive: false},

		// Generated Destructive: both deletes.
		{name: "delete removes the stack", action: "stacks.delete", mutating: true, destructive: true},
		{name: "delete_kubernetes_by_name removes stacks", action: "stacks.delete_kubernetes_by_name", mutating: true, destructive: true},

		// Hand ruling: content replaced from a remote at a revision the
		// request cannot name, with no stored copy to restore.
		{name: "git_redeploy overwrites the deployment from git", action: "stacks.git_redeploy", mutating: true, destructive: true},
		{name: "webhook_invoke is git_redeploy by another door", action: "stacks.webhook_invoke", mutating: true, destructive: true},
		{name: "edge_stack_webhook_invoke likewise, on edge environments", action: "stacks.edge_stack_webhook_invoke", mutating: true, destructive: true},

		// Hand ruling the other way: convert returns generated files for
		// preview and changes nothing, so the name is the only thing about
		// it that sounds irreversible.
		{name: "convert only previews", action: "stacks.convert", mutating: true, destructive: false},

		// Writes that are not destructive: every field they overwrite the
		// caller supplies in the same request, or they are reversible.
		{name: "update replaces caller-supplied content", action: "stacks.update", mutating: true, destructive: false},
		{name: "update_git changes settings only", action: "stacks.update_git", mutating: true, destructive: false},
		{name: "start is reversible by stop", action: "stacks.start", mutating: true, destructive: false},
		{name: "stop is reversible by start", action: "stacks.stop", mutating: true, destructive: false},
		{name: "associate re-parents a record", action: "stacks.associate", mutating: true, destructive: false},

		// Creates: they add a stack at a new identifier and remove nothing.
		{name: "create_docker_standalone_repository", action: "stacks.create_docker_standalone_repository", mutating: true, destructive: false},
		{name: "create_docker_standalone_string", action: "stacks.create_docker_standalone_string", mutating: true, destructive: false},
		{name: "create_docker_swarm_repository", action: "stacks.create_docker_swarm_repository", mutating: true, destructive: false},
		{name: "create_docker_swarm_string", action: "stacks.create_docker_swarm_string", mutating: true, destructive: false},
		{name: "create_kubernetes_string", action: "stacks.create_kubernetes_string", mutating: true, destructive: false},
		{name: "create_kubernetes_git", action: "stacks.create_kubernetes_git", mutating: true, destructive: false},
		{name: "create_kubernetes_url", action: "stacks.create_kubernetes_url", mutating: true, destructive: false},
	}
	if len(tests) != len(generatedOperations) {
		t.Fatalf("this table covers %d action(s) and the domain ships %d; every action must have a row", len(tests), len(generatedOperations))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := findSpec(t, tt.action)
			if spec.Mutating != tt.mutating {
				t.Errorf("%s: Mutating = %v, want %v", tt.action, spec.Mutating, tt.mutating)
			}
			if spec.Destructive != tt.destructive {
				t.Errorf("%s: Destructive = %v, want %v", tt.action, spec.Destructive, tt.destructive)
			}
		})
	}
}

// TestUnit_ActionNames_MatchThisDomainsRulings pins the one action name this
// domain overrides and the one it deliberately left alone.
//
// StackCreateKubernetesFile's mechanical name, "stacks.create_kubernetes_file",
// names a file upload that does not exist: the route is POST
// /stacks/create/kubernetes/string and its body is JSON with a
// stackFileContent property. The rename is registered in
// cmd/gen_action_inputs's actionNameOverrides so a regeneration reproduces
// it, but nothing in this package's build would notice if that entry were
// removed and the literal here reverted — the name would simply change under
// every caller. This is what notices.
func TestUnit_ActionNames_MatchThisDomainsRulings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		operationID string
		action      string
	}{
		{
			name:        "the kubernetes inline-manifest route is named for the string it takes",
			operationID: "StackCreateKubernetesFile",
			action:      "stacks.create_kubernetes_string",
		},
		{
			// Inconsistent with create_docker_standalone_repository for the
			// same /repository route shape, but not misleading, so it keeps
			// the mechanical name.
			name:        "the kubernetes git route keeps its mechanical name",
			operationID: "StackCreateKubernetesGit",
			action:      "stacks.create_kubernetes_git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := specByOperationID(t, tt.operationID).Name; got != tt.action {
				t.Errorf("%s is named %q, want %q", tt.operationID, got, tt.action)
			}
		})
	}
}

// TestUnit_Editions_MatchTheVendoredSpecs pins the four Business Edition
// actions in this domain, two of which are easy to get wrong in opposite
// directions.
//
// EdgeStackWebhookInvoke is Business Edition because
// /edge_stacks/webhooks/{webhookID} is absent from api/specs/ce-2.44.0.json
// altogether — that document declares seven /edge_stacks/ paths against the
// Business Edition document's fifteen. This wave's reconnaissance recorded
// the route as present and tagged "stacks" in *both* documents, which would
// have put it on CE.
//
// StacksWebhookInvoke is Business Edition for a different and weaker reason:
// POST /stacks/webhooks/{webhookID} is served by both editions, but the
// Community Edition document names the operation WebhookInvoke, and the
// catalog is built from the Business Edition document alone. That is a
// catalogue artefact rather than a real edition boundary, and resolving it —
// a second hand-declared ActionSpec, a coverage allow-list entry, or an
// accepted gap recorded in docs/api-divergences.md — belongs to the task
// that registers this domain. Pinned here so that resolution is a deliberate
// change to this line rather than a silent one.
func TestUnit_Editions_MatchTheVendoredSpecs(t *testing.T) {
	t.Parallel()
	businessEditionOnly := map[string]bool{
		"EdgeStackWebhookInvoke": true,
		"StacksWebhookInvoke":    true,
		"StackConvert":           true,
		"StackImagesStatus":      true,
	}
	for _, id := range generatedOperations {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			want := edition.CE
			if businessEditionOnly[id] {
				want = edition.EE
			}
			if got := specByOperationID(t, id).Edition; got != want {
				t.Errorf("%s: Edition = %v, want %v", id, got, want)
			}
		})
	}
}
