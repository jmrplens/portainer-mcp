package custom_templates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
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

// gitBackedTemplate is a custom template whose git configuration carries a
// live credential, built from the real generated response type rather than
// from a JSON literal. The concrete type is the point: a wrapper typed `any`
// that returned its argument untouched would satisfy the generated
// reflective guard in redaction_test.go and `make audit-spec-drift` alike,
// so a fixture that only type-checks against `any` proves nothing about
// PortainereeCustomTemplate.GitConfig.Authentication actually being dropped
// (docs/api-divergences.md §9.4).
//
// A live server would not return this response. Portainer 2.44.0 was
// measured blanking Password itself on create/repository, GET
// custom_templates/{id} and GET custom_templates, on both editions — it
// answers Authentication: {"Username":"…","Password":""}. That is exactly
// why this has to be a unit test with a response constructed here: an
// end-to-end test against a real server cannot distinguish a redactor that
// works from one that does nothing, because the field it should strip
// arrives empty either way.
func gitBackedTemplate(id int, title string) apigen.PortainereeCustomTemplate {
	return apigen.PortainereeCustomTemplate{
		Id:          ptr(id),
		Title:       ptr(title),
		Description: ptr("a git-backed template"),
		EntryPoint:  ptr("docker-compose.yml"),
		GitConfig: &apigen.GithubComPortainerPortainerEeApiGitTypesRepoConfig{
			URL:            ptr("https://git.example.com/team/app.git"),
			ReferenceName:  ptr("refs/heads/main"),
			ConfigFilePath: ptr("docker-compose.yml"),
			Authentication: &apigen.GithubComPortainerPortainerApiGitTypesGitAuthentication{
				Username:        ptr("deploy-bot"),
				Password:        ptr("hunter2"),
				GitCredentialID: ptr(7),
			},
		},
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return body
}

// TestUnit_HandlerWithGitCredentialInResponse_ReturnsNoCredential runs each
// credential-bearing action's real handler against a response carrying a
// populated GitConfig.Authentication and asserts nothing of it survives.
//
// This is the discriminating test the generated redaction_test.go is not:
// that one drives every wrapper through reflect.New on the wrapper's own
// declared parameter type, so a wrapper declared `func(any) any` returning
// its argument unchanged passes it. Here the fixture is a real
// PortainereeCustomTemplate and the assertion is made on the marshalled
// bytes a model would actually receive.
//
// Username is asserted absent alongside Password on purpose. Portainer
// itself returns Username populated and Password blank; redactCustomTemplate
// drops the whole Authentication sub-object, so these actions deliberately
// return strictly less than the server does. Asserting only Password would
// let a field-by-field redactor that kept Username pass, and no narrative in
// this domain promises a caller that a username will be there.
func TestUnit_HandlerWithGitCredentialInResponse_ReturnsNoCredential(t *testing.T) {
	t.Parallel()

	single := mustMarshal(t, gitBackedTemplate(3, "nginx-stack"))
	// Two entries, only the first credential-bearing: a list redactor that
	// forgot its per-element loop, or that redacted only the first element,
	// fails here and nowhere else in this file.
	list := mustMarshal(t, []apigen.PortainereeCustomTemplate{
		{Id: ptr(1), Title: ptr("plain-template")},
		gitBackedTemplate(3, "nginx-stack"),
	})

	tests := []struct {
		name   string
		action string
		body   []byte
		input  string
	}{
		{
			name:   "inspect",
			action: "custom_templates.inspect",
			body:   single,
			input:  `{"id":3}`,
		},
		{
			name:   "update",
			action: "custom_templates.update",
			body:   single,
			input:  `{"id":3,"title":"nginx-stack","description":"a git-backed template","fileContent":"services: {}","type":2}`,
		},
		{
			name:   "create_repository",
			action: "custom_templates.create_repository",
			body:   single,
			input:  `{"title":"nginx-stack","description":"a git-backed template","type":2,"platform":1,"sourceId":0,"repositoryUrl":"https://git.example.com/team/app.git"}`,
		},
		{
			name:   "create_string",
			action: "custom_templates.create_string",
			body:   single,
			input:  `{"title":"nginx-stack","description":"a git-backed template","fileContent":"services: {}","type":2,"platform":1}`,
		},
		{
			name:   "list",
			action: "custom_templates.list",
			body:   list,
			input:  `{"type":[1,2,3]}`,
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
			encoded := string(mustMarshal(t, out))

			if strings.Contains(encoded, "hunter2") {
				t.Errorf("%s returned the git password to the caller: %s", tt.action, encoded)
			}
			if strings.Contains(encoded, "deploy-bot") {
				t.Errorf("%s returned the git username to the caller; Authentication must be dropped whole: %s", tt.action, encoded)
			}
			if strings.Contains(encoded, `"GitCredentialID"`) {
				t.Errorf("%s returned the git credential identifier to the caller: %s", tt.action, encoded)
			}
			// Redaction must remove the credential and nothing else: the
			// repository URL and the template's own fields are what the
			// action exists to return.
			if !strings.Contains(encoded, "nginx-stack") {
				t.Errorf("%s: redaction removed more than the credential, the template title is gone: %s", tt.action, encoded)
			}
			if !strings.Contains(encoded, "https://git.example.com/team/app.git") {
				t.Errorf("%s: redaction removed more than the credential, the repository URL is gone: %s", tt.action, encoded)
			}
		})
	}
}

// TestUnit_ListWithACredentialInOneEntry_LeavesTheOtherEntriesIntact pins
// what the table above cannot say on its own: the list handler returns both
// elements, not just the redacted one. A redactList that dropped every
// element it had to scrub — returning a shorter slice — would pass every
// credential assertion above.
func TestUnit_ListWithACredentialInOneEntry_LeavesTheOtherEntriesIntact(t *testing.T) {
	t.Parallel()
	body := mustMarshal(t, []apigen.PortainereeCustomTemplate{
		{Id: ptr(1), Title: ptr("plain-template")},
		gitBackedTemplate(3, "nginx-stack"),
	})
	c := clientFor(t, body)

	out, err := find(t, "custom_templates.list")(context.Background(), c, json.RawMessage(`{"type":[1,2,3]}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	templates, ok := out.(*[]apigen.PortainereeCustomTemplate)
	if !ok {
		t.Fatalf("handler returned %T, want *[]apigen.PortainereeCustomTemplate", out)
	}
	if len(*templates) != 2 {
		t.Fatalf("handler returned %d template(s), want both", len(*templates))
	}
	if (*templates)[0].GitConfig != nil {
		t.Error("the credential-free entry acquired a GitConfig it did not have")
	}
	if (*templates)[1].GitConfig == nil || (*templates)[1].GitConfig.Authentication != nil {
		t.Errorf("the git-backed entry's Authentication = %+v, want the sub-object dropped and the rest kept", (*templates)[1].GitConfig)
	}
}

// TestUnit_Specs_AreAllValid guards the invariants ActionSpec.Validate
// enforces, including Destructive implying Mutating.
func TestUnit_Specs_AreAllValid(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}
}

// TestUnit_Narrative_OverridesTitleAndDescriptionAwayFromTheSpecText pins
// the narrative hook itself, per operation. Nothing else does: `go test
// ./...`, `make audit-spec-drift` and `make audit-1to1` all stay green when
// the whole switch statement in narrative() is deleted, because none of them
// reads what a Description says — only that one exists. Without this test,
// this domain silently reverts to the vendored specification's own prose,
// in which three distinct create actions all read "Create a custom template"
// and seven of ten descriptions merely restate their summary.
//
// Asserted as "differs from the specification's own text" rather than as
// literal strings: improving one of these sentences is expected, ordinary
// work (see toolutil.WithNarrative's doc comment), and pinning the literal
// text would tax every such improvement for no gain against the failure this
// catches — the override disappearing.
func TestUnit_Narrative_OverridesTitleAndDescriptionAwayFromTheSpecText(t *testing.T) {
	t.Parallel()
	// All nine shippable operations: the eight generated ones and
	// CustomTemplateCreateFile, whose ActionSpec is declared by hand in
	// handWrittenSpecs but goes through toolutil.WithNarrative exactly like
	// the rest.
	overridden := []string{
		"CustomTemplateList",
		"CustomTemplateInspect",
		"CustomTemplateFile",
		"CustomTemplateGitFetch",
		"CustomTemplateUpdate",
		"CustomTemplateCreateString",
		"CustomTemplateCreateRepository",
		"CustomTemplateCreateFile",
		"CustomTemplateDelete",
	}
	for _, id := range overridden {
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
// reason this domain overrides every action rather than the two or three a
// reviewer would notice. CustomTemplateCreateFile, CustomTemplateCreateRepository
// and CustomTemplateCreateString share the byte-identical summary "Create a
// custom template" in the vendored specification, so without an override
// three shipping actions arrive with the same Title and the same
// Description and portainer_find_action cannot rank one above another.
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

// TestUnit_DangerFlags_MatchThisDomainsRuling pins the two rulings recorded
// in actions.go, both of which are hand decisions the generator did not
// prompt: suspectDangerMismatch's keyword list matches nothing in this
// domain, so the scaffold run printed no warning at all and the verb-derived
// defaults would give both PUTs Mutating+Idempotent and neither Destructive.
// Regenerating this domain would silently restore those defaults; this test
// is what notices.
func TestUnit_DangerFlags_MatchThisDomainsRuling(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		action      string
		mutating    bool
		destructive bool
	}{
		// Overwrites the stored stack file from the remote, discarding what
		// was there, with nothing in the request describing what is lost.
		{name: "git_fetch overwrites unseen content", action: "custom_templates.git_fetch", mutating: true, destructive: true},
		// A whole-object replace, but of fields the caller supplies in the
		// same request; registries.update, the identical shape, is not
		// destructive either.
		{name: "update replaces caller-supplied fields", action: "custom_templates.update", mutating: true, destructive: false},
		{name: "delete removes the template", action: "custom_templates.delete", mutating: true, destructive: true},
		{name: "file only reads", action: "custom_templates.file", mutating: false, destructive: false},
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
