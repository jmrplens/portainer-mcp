//go:build e2e

package suite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// This file covers all nine custom_templates actions against the live
// estate, on all three surfaces and on every edition the estate carries.
// `make audit-e2e-gaps` reported every one of the nine as unreferenced
// before it existed — the same audit that, one stage earlier, found six of
// docker's eight actions with no e2e test touching them while the catalog
// reported full coverage.
//
// Two of the nine (create_repository, git_fetch) cannot be exercised at all
// without a git repository Portainer can clone, which is what
// docker-compose.yml's `git` service serves and test/e2e/harness/
// gitfixture.go addresses. Safe mode is not a substitute for either: it
// intercepts before the handler runs (internal/tools/register.go), so a
// safe-mode-only test proves the interception and nothing about the action.

// customTemplateFixtureType is the stack type every template this file
// creates carries: 2, compose. It is also what the list calls below ask for,
// and they ask for exactly this one type rather than all three for a
// measured reason — see
// TestCustomTemplates_List_RefusesMoreThanOneTypeAtATime.
const customTemplateFixtureType = 2

// customTemplateListTypes is every stack type the list route accepts. Used
// only where a caller has to sweep everything (orphan cleanup) and therefore
// issues one request per type.
var customTemplateListTypes = []int{1, 2, 3}

// customTemplateStackFile builds a stack file carrying marker, so a content
// assertion identifies one specific template rather than passing against
// whatever another test in the matrix happened to leave on the same server.
// It is a real compose file: nothing here deploys it, but a fixture shaped
// like the thing it stands in for costs nothing and keeps the probe honest.
func customTemplateStackFile(marker string) string {
	return fmt.Sprintf("services:\n  hello:\n    image: busybox:1.36\n    command: [\"echo\", %q]\n", marker)
}

// createCustomTemplateFixture creates an inline (non-git) custom template on
// edition ed's server through the raw Portainer API — never through an MCP
// surface — and returns its id, with cleanup registered.
//
// Raw on purpose: this exists for the tests whose subject is something other
// than creation (safe mode's update/delete/git_fetch previews need a
// template that already exists), and building a fixture through the very
// action under test would make a create regression look like a failure of
// whatever the test was really about.
func createCustomTemplateFixture(t *testing.T, ed, title string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create custom template %q", title), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		platform := apigen.PortainerCustomTemplatePlatform(1)
		body := apigen.CustomTemplateCreateStringJSONRequestBody{
			Title:       title,
			Description: "e2e fixture",
			FileContent: customTemplateStackFile(title),
			Platform:    &platform,
			Type:        2,
		}
		resp, err := client.API.CustomTemplateCreateStringWithResponse(ctx, body)
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			return fmt.Errorf("create custom template %q: response carried no id", title)
		}
		id = *resp.JSON200.Id
		return nil
	})

	registerCustomTemplateCleanup(t, ed, id)
	return id
}

// registerCustomTemplateCleanup registers id's deletion with the calling
// test's ledger. It is separate from createCustomTemplateFixture because the
// templates this file mostly cares about are created through the actions
// under test, not through the fixture helper, and every one of those still
// has to be cleaned up.
func registerCustomTemplateCleanup(t *testing.T, ed string, id int) {
	t.Helper()
	newLedger(t).Register("custom_template", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteCustomTemplateIfPresent(ctx, ed, id)
	})
}

// deleteCustomTemplateIfPresent deletes template id on edition ed's server,
// treating "already gone" as success: custom_templates.delete is itself one
// of the actions under test here, so by the time a test ends the template it
// created may already have been deleted by the test body. See
// deleteTagIfPresent, which exists for the identical reason.
func deleteCustomTemplateIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.CustomTemplateDeleteWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("delete custom template %d: %w", id, err)
	}
	// 404 means the test under it already deleted the template, which is a
	// success for this cleanup rather than something to report.
	if resp.StatusCode() == http.StatusNotFound {
		return nil
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("delete custom template %d: %w", id, err)
	}
	return nil
}

// rawCustomTemplate reads template id straight from edition ed's Portainer
// API, bypassing every MCP surface. It is what makes an assertion about our
// own redaction possible: the server's own answer is the only thing that can
// show a field is present upstream and absent downstream.
func rawCustomTemplate(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.Do(ctx, "GET", fmt.Sprintf("/custom_templates/%d", id), nil)
	if err != nil {
		t.Fatalf("raw GET /custom_templates/%d: %v", id, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw GET /custom_templates/%d: status %d, body %s", id, resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding raw GET /custom_templates/%d body %q: %v", id, body, err)
	}
	return out
}

// rawCustomTemplateFile reads template id's stored stack file straight from
// edition ed's Portainer API. Used where a test has to know what the server
// really holds, independently of the action whose behaviour is in question.
func rawCustomTemplateFile(t *testing.T, ed string, id int) string {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.API.CustomTemplateFileWithResponse(ctx, id)
	if err != nil {
		t.Fatalf("raw GET /custom_templates/%d/file: %v", id, err)
	}
	if err := toolutil.Check(resp); err != nil {
		t.Fatalf("raw GET /custom_templates/%d/file: %v", id, err)
	}
	if resp.JSON200 == nil || resp.JSON200.FileContent == nil {
		t.Fatalf("raw GET /custom_templates/%d/file returned no FileContent", id)
	}
	return *resp.JSON200.FileContent
}

// templateID pulls the Id out of a custom-template action result. JSON
// numbers decode into float64 through map[string]any, and a template id that
// silently arrived as something else (a string, or absent) would otherwise
// turn into a confusing failure several calls later.
func templateID(t *testing.T, out map[string]any, action string) int {
	t.Helper()
	raw, ok := out["Id"].(float64)
	if !ok {
		t.Fatalf("%s returned no usable Id: %v", action, out)
	}
	return int(raw)
}

// templateFileContent calls custom_templates.file and returns the stored
// stack file. The response carries exactly one field (measured against
// 2.44.0, and stated in the action's own narrative), so its absence is a
// real shape regression rather than an empty-template artefact.
func templateFileContent(t *testing.T, surface, action string, out map[string]any) string {
	t.Helper()
	content, ok := out["FileContent"].(string)
	if !ok {
		t.Fatalf("%s on the %s surface returned no FileContent: %v", action, surface, out)
	}
	return content
}

// gitFixtureContainerID finds the running git fixture container by its
// compose labels rather than by container name: compose names it
// "<project>-git-1" today, but the number is compose's to choose, and the
// labels are the documented identity.
func gitFixtureContainerID(t *testing.T) string {
	t.Helper()
	out, err := gitFixtureDocker(t, nil,
		"ps", "-q",
		"--filter", "label=com.docker.compose.project="+harness.ComposeProject,
		"--filter", "label=com.docker.compose.service="+harness.GitFixtureService,
	)
	if err != nil {
		t.Fatalf("locating the git fixture container: %v", err)
	}
	id := strings.TrimSpace(out)
	if id == "" {
		t.Fatalf("no running container for compose service %q in project %q: run `make e2e-up`",
			harness.GitFixtureService, harness.ComposeProject)
	}
	// More than one line means more than one candidate container, and
	// execing into an arbitrary one of them would push a commit somewhere
	// this test cannot then read back.
	if strings.Contains(id, "\n") {
		t.Fatalf("more than one container matches compose service %q in project %q: %q",
			harness.GitFixtureService, harness.ComposeProject, id)
	}
	return id
}

// gitFixtureCommit commits content as the mutable fixture repository's stack
// file and pushes it, by execing the /commit.sh the git service wrote into
// its own filesystem (see docker-compose.yml).
//
// This is the one thing in this file that reaches the Docker daemon rather
// than Portainer, and it is what makes the git_fetch assertion mean
// anything: without a commit landing between the create and the fetch, an
// implementation that answered from the copy stored at create time — the
// stale-cache behaviour docs/api-divergences.md section 2.4 records for
// another operation — would satisfy the test exactly as well as a real
// re-fetch does.
func gitFixtureCommit(t *testing.T, content string) {
	t.Helper()
	if _, err := gitFixtureDocker(t, []byte(content), "exec", "-i", gitFixtureContainerID(t), harness.GitFixtureCommitScript); err != nil {
		t.Fatalf("pushing a commit into the mutable fixture repository: %v", err)
	}
}

// gitFixtureMutableContent returns the stack file the mutable fixture
// repository serves right now, read out of the bare repository itself rather
// than out of the work tree that produced it.
//
// It exists because that repository is written to by the very test that
// reads it: after one run, its content is whatever that run pushed last, so
// a test that compared a fresh clone against the seeded constant would pass
// on a newly upped estate and fail on every `go test` after it, for no
// reason connected to the code under test.
func gitFixtureMutableContent(t *testing.T) string {
	t.Helper()
	repo := gitFixtureMutableRepoPath(t)
	out, err := gitFixtureDocker(t, nil, "exec", gitFixtureContainerID(t),
		"git", "--git-dir="+repo, "show", "main:"+harness.GitFixtureConfigFilePath)
	if err != nil {
		t.Fatalf("reading %s out of the mutable fixture repository: %v", harness.GitFixtureConfigFilePath, err)
	}
	return out
}

// gitFixtureMutableRepoPath is where the mutable bare repository lives inside
// the git container, derived from the URL the suite clones it by rather than
// written out a second time.
func gitFixtureMutableRepoPath(t *testing.T) string {
	t.Helper()
	parsed, err := url.Parse(harness.GitFixtureMutableRepositoryURL)
	if err != nil {
		t.Fatalf("parsing %q: %v", harness.GitFixtureMutableRepositoryURL, err)
	}
	return "/srv/" + path.Base(parsed.Path)
}

// gitFixtureDocker runs one docker command against the daemon the estate was
// brought up on, feeding it stdin when there is any.
//
// It honours the destination test/e2e/scripts/lib.sh's record_docker_host
// wrote (".docker-host", absent for a local estate), so that on an estate
// brought up with `make e2e-up-remote` this reaches the same daemon the
// containers actually run on instead of silently finding nothing here.
func gitFixtureDocker(t *testing.T, stdin []byte, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...) //nolint:gosec // fixed argv built from constants and a compose-derived id
	cmd.Env = os.Environ()
	if dest := recordedDockerHost(t); dest != "" {
		cmd.Env = append(cmd.Env, "DOCKER_HOST=ssh://"+dest)
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// recordedDockerHost returns the ssh destination the compose estate was
// brought up against, or "" for the ordinary local estate. It reads the same
// marker file scripts/lib.sh's record_docker_host writes, so `make e2e-down`
// and this test agree on where the estate lives without either one taking a
// flag.
func recordedDockerHost(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", ".docker-host"))
	if err != nil {
		t.Fatalf("resolving the docker host marker path: %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // fixed, repo-relative path; not user input
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
}

// TestCustomTemplates_InlineTemplateLifecycle_CreatesReadsUpdatesAndDeletes
// walks one template through five of the nine actions in the order a caller
// would: create_string, list, inspect, file, update, delete. Every step
// reads back what the previous one claimed, because a create that answers
// 200 and stores nothing, and an update that answers 200 and changes
// nothing, are both real Portainer behaviours this repository has already
// measured elsewhere (docs/api-divergences.md section 2.1).
//
// It runs on every compose leg and every surface. custom_templates is
// declared edition.CE — available on both editions — and both legs are
// separate servers with their own template database, so each (leg, surface)
// pair creates and destroys its own template.
func TestCustomTemplates_InlineTemplateLifecycle_CreatesReadsUpdatesAndDeletes(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				title := uniqueName("template")
				content := customTemplateStackFile(title)
				created := callAction[map[string]any](t, session, surface, "custom_templates.create_string", map[string]any{
					"title":       title,
					"description": "created by " + t.Name(),
					"fileContent": content,
					"platform":    1,
					"type":        2,
				})
				id := templateID(t, created, "custom_templates.create_string")
				registerCustomTemplateCleanup(t, leg.Name, id)
				if got, _ := created["Title"].(string); got != title {
					t.Errorf("custom_templates.create_string returned Title %q, want %q", got, title)
				}

				listed := callAction[[]map[string]any](t, session, surface, "custom_templates.list", map[string]any{
					"type": []int{customTemplateFixtureType},
				})
				if !customTemplateListed(listed, id, title) {
					t.Errorf("custom_templates.list does not carry the template %d (%q) just created: %v", id, title, listed)
				}

				inspected := callAction[map[string]any](t, session, surface, "custom_templates.inspect", map[string]any{"id": id})
				if got, _ := inspected["Title"].(string); got != title {
					t.Errorf("custom_templates.inspect returned Title %q, want %q", got, title)
				}

				file := callAction[map[string]any](t, session, surface, "custom_templates.file", map[string]any{"id": id})
				if got := templateFileContent(t, surface, "custom_templates.file", file); got != content {
					t.Errorf("custom_templates.file returned\n%q\nwant the content create_string was given:\n%q", got, content)
				}

				// The update rewrites both the title and the stack file, and
				// both are read back below: the response alone would be
				// satisfied by a server that echoed the request without
				// storing it.
				newTitle := title + "-updated"
				newContent := customTemplateStackFile(newTitle)
				updated := callAction[map[string]any](t, session, surface, "custom_templates.update", map[string]any{
					"id":          id,
					"title":       newTitle,
					"description": "updated by " + t.Name(),
					"fileContent": newContent,
					"platform":    1,
					"type":        2,
				})
				if got, _ := updated["Title"].(string); got != newTitle {
					t.Errorf("custom_templates.update returned Title %q, want %q", got, newTitle)
				}
				afterUpdate := callAction[map[string]any](t, session, surface, "custom_templates.file", map[string]any{"id": id})
				if got := templateFileContent(t, surface, "custom_templates.file", afterUpdate); got != newContent {
					t.Errorf("after custom_templates.update the stored file is\n%q\nwant\n%q", got, newContent)
				}

				deleted := callAction[map[string]any](t, session, surface, "custom_templates.delete", map[string]any{"id": id})
				if got, _ := deleted["status"].(string); got != "ok" {
					t.Errorf("custom_templates.delete returned %v, want status ok", deleted)
				}
				// The read-back that makes the delete mean something: the
				// route answers 204 with no body, so its own response cannot
				// distinguish a real delete from an accepted no-op.
				assertActionFails(t, session, surface, "custom_templates.inspect", map[string]any{"id": id})
			})
		}
	}
}

// customTemplateListed reports whether a list response carries a template
// with this id and title.
func customTemplateListed(listed []map[string]any, id int, title string) bool {
	for _, entry := range listed {
		entryID, ok := entry["Id"].(float64)
		if !ok || int(entryID) != id {
			continue
		}
		if got, _ := entry["Title"].(string); got == title {
			return true
		}
	}
	return false
}

// TestCustomTemplates_List_RefusesMoreThanOneTypeAtATime pins a live defect
// this suite found rather than a property worth having.
//
// custom_templates.list takes an array of stack types, and the obvious call
// — "list every custom template", type [1, 2, 3] — fails against a real
// 2.44.0 with 400 "Invalid Custom template type: Failed parsing template
// type: strconv.Atoi: parsing \"1,2,3\": invalid syntax". The vendored
// specification declares the parameter `explode: false`, so the generated
// client encodes the array the OpenAPI way, as one comma-joined value, while
// Portainer's handler expects the parameter repeated. Neither side is
// misbehaving on its own terms; the document is wrong, and every caller
// passing more than one type pays for it. See docs/api-divergences.md
// section 6.7.
//
// It is asserted as the measured behaviour, with the server's own words, for
// the same reason TestDocker_ContainerImageStatus_AgainstARealContainer
// asserts that a fabricated container id answers "skipped": recording what
// the server really does is what makes a change to it — a Portainer that
// starts accepting the comma form, or a regenerated client that stops
// sending it — visible instead of silent. A test that only ever called list
// with one type would leave this defect invisible to the whole suite.
func TestCustomTemplates_List_RefusesMoreThanOneTypeAtATime(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)
				toolName, args := actionCallParams(t, surface, "custom_templates.list", map[string]any{
					"type": customTemplateListTypes,
				})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("CallTool(%s): %v", toolName, err)
				}
				if !res.IsError {
					t.Fatalf("custom_templates.list with types %v succeeded; if Portainer now accepts the comma-joined form, docs/api-divergences.md section 6.7 and this test are out of date: %s",
						customTemplateListTypes, toolResultText(res))
				}
				const want = "Invalid Custom template type"
				if text := toolResultText(res); !strings.Contains(text, want) {
					t.Errorf("custom_templates.list with types %v failed with %q, want the measured %q: a different failure is a different defect",
						customTemplateListTypes, text, want)
				}
			})
		}
	}
}

// TestCustomTemplates_CreateFile_StoresTheUploadedStackFile covers the one
// action in this domain with a hand-written handler
// (internal/tools/custom_templates/handlers.go): its request body is
// multipart/form-data, which oapi-codegen could not type, so the multipart
// writing is this repository's own code and nothing but a live call proves
// Portainer accepts what it produces.
//
// EntryPoint is asserted as the literal "docker-compose.yml" because that is
// what the server was measured returning for every upload, whatever the
// multipart filename says (docs/api-divergences.md section 2.5): the handler
// sends a constant "template.yml" and Portainer ignores it. Pinning the
// measured value here is what would catch that changing under a Portainer
// upgrade — at which point the constant would stop being merely mechanical
// and would need a caller-supplied value instead.
func TestCustomTemplates_CreateFile_StoresTheUploadedStackFile(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				title := uniqueName("template-file")
				content := customTemplateStackFile(title)
				note := "uploaded by " + t.Name()
				created := callAction[map[string]any](t, session, surface, "custom_templates.create_file", map[string]any{
					"title":       title,
					"description": "created by " + t.Name(),
					"note":        note,
					"file":        content,
					"platform":    1,
					"type":        2,
				})
				id := templateID(t, created, "custom_templates.create_file")
				registerCustomTemplateCleanup(t, leg.Name, id)

				if got, _ := created["Title"].(string); got != title {
					t.Errorf("custom_templates.create_file returned Title %q, want %q", got, title)
				}
				if got, _ := created["Note"].(string); got != note {
					t.Errorf("custom_templates.create_file returned Note %q, want %q", got, note)
				}
				if got, _ := created["EntryPoint"].(string); got != harness.GitFixtureConfigFilePath {
					t.Errorf("custom_templates.create_file returned EntryPoint %q, want the measured %q", got, harness.GitFixtureConfigFilePath)
				}

				// The assertion that proves the multipart body this
				// repository writes by hand reached the server intact: the
				// stored file must be the uploaded bytes, not an empty part
				// or a truncated one.
				file := callAction[map[string]any](t, session, surface, "custom_templates.file", map[string]any{"id": id})
				if got := templateFileContent(t, surface, "custom_templates.file", file); got != content {
					t.Errorf("custom_templates.file returned\n%q\nwant the uploaded content\n%q", got, content)
				}
			})
		}
	}
}

// TestCustomTemplates_CreateRepository_ClonesTheEstatesGitRepository is the
// action that could not be exercised at all before this estate grew a git
// server (docker-compose.yml's `git` service).
//
// The discriminating assertion is the stack file: it must be
// harness.GitFixtureStackFile byte for byte, which is content only the
// fixture repository holds. A create that answered 200 without cloning
// anything, or that cloned some other repository, fails it — where an
// assertion on the response's own Id or Title would not.
func TestCustomTemplates_CreateRepository_ClonesTheEstatesGitRepository(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				title := uniqueName("template-git")
				created := callAction[map[string]any](t, session, surface, "custom_templates.create_repository", map[string]any{
					"title":                       title,
					"description":                 "cloned by " + t.Name(),
					"repositoryUrl":               harness.GitFixtureRepositoryURL,
					"composeFilePathInRepository": harness.GitFixtureConfigFilePath,
					"platform":                    1,
					"type":                        2,
				})
				id := templateID(t, created, "custom_templates.create_repository")
				registerCustomTemplateCleanup(t, leg.Name, id)

				gitConfig, ok := created["GitConfig"].(map[string]any)
				if !ok {
					t.Fatalf("custom_templates.create_repository returned no GitConfig: %v", created)
				}
				if got, _ := gitConfig["ConfigFilePath"].(string); got != harness.GitFixtureConfigFilePath {
					t.Errorf("stored GitConfig.ConfigFilePath = %q, want %q", got, harness.GitFixtureConfigFilePath)
				}
				// Portainer stores the URL with the ".git" suffix stripped
				// (measured), so this is a prefix check rather than equality
				// — and still fails if the template were cloned from
				// somewhere else entirely.
				if got, _ := gitConfig["URL"].(string); !strings.HasPrefix(harness.GitFixtureRepositoryURL, got) || got == "" {
					t.Errorf("stored GitConfig.URL = %q, want a prefix of %q", got, harness.GitFixtureRepositoryURL)
				}

				file := callAction[map[string]any](t, session, surface, "custom_templates.file", map[string]any{"id": id})
				if got := templateFileContent(t, surface, "custom_templates.file", file); got != harness.GitFixtureStackFile {
					t.Errorf("the cloned template's stack file is\n%q\nwant the fixture repository's own content\n%q", got, harness.GitFixtureStackFile)
				}
			})
		}
	}
}

// TestCustomTemplates_CreateRepository_AcceptsAKubernetesTypeTemplate is the
// live half of the enum widening docs/api-divergences.md §6.5 prescribed and
// this stage carried out: the vendored specification declares
// enum [1, 2] for Type on this route alone, the server accepts 3, and the
// catalog now publishes [1, 2, 3].
//
// It fails in two distinct ways, which is the point of having it: reverting
// customTemplateCreateRepositoryInput's EnumParams to [1, 2] makes
// ValidateInput refuse the call locally, before any request is built, and a
// Portainer that started rejecting type 3 on this route would fail it from
// the other side. Either way the catalog and the server would have stopped
// agreeing, which is the thing an allow-listed schema divergence has to keep
// being checked for.
func TestCustomTemplates_CreateRepository_AcceptsAKubernetesTypeTemplate(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				title := uniqueName("template-git-k8s")
				created := callAction[map[string]any](t, session, surface, "custom_templates.create_repository", map[string]any{
					"title":                       title,
					"description":                 "kubernetes template cloned by " + t.Name(),
					"repositoryUrl":               harness.GitFixtureRepositoryURL,
					"composeFilePathInRepository": harness.GitFixtureConfigFilePath,
					"platform":                    1,
					"type":                        3,
				})
				registerCustomTemplateCleanup(t, leg.Name, templateID(t, created, "custom_templates.create_repository"))
				if got, _ := created["Type"].(float64); int(got) != 3 {
					t.Errorf("custom_templates.create_repository stored Type %v, want 3 (kubernetes)", created["Type"])
				}
			})
		}
	}
}

// TestCustomTemplates_CreateRepository_DropsTheStoredGitUsername is the only
// redaction assertion in this file that can fail, and the reason for the
// shape it has.
//
// Portainer already blanks the git password itself: create/repository,
// GET /custom_templates/{id} and GET /custom_templates were all measured
// answering Authentication: {"Username":"…","Password":""} on both
// editions. An e2e assertion that "the password is absent" would therefore
// pass with internal/redact deleted, and would be worse than no test at all.
// What only this repository's redactor explains is the USERNAME: the raw API
// returns it, and every custom_templates action must not.
//
// Both halves are asserted against the same template, in the same test: the
// raw read is what proves the tool-surface absence is redaction rather than
// a server that simply had nothing to return.
func TestCustomTemplates_CreateRepository_DropsTheStoredGitUsername(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				title := uniqueName("template-git-auth")
				const username = "e2e-git-user"
				created := callAction[map[string]any](t, session, surface, "custom_templates.create_repository", map[string]any{
					"title":                       title,
					"description":                 "cloned with credentials by " + t.Name(),
					"repositoryUrl":               harness.GitFixtureRepositoryURL,
					"composeFilePathInRepository": harness.GitFixtureConfigFilePath,
					"platform":                    1,
					"type":                        2,
					"repositoryAuthentication":    true,
					"repositoryUsername":          username,
					"repositoryPassword":          "e2e-git-" + t.Name(),
				})
				id := templateID(t, created, "custom_templates.create_repository")
				registerCustomTemplateCleanup(t, leg.Name, id)

				// The server's own answer, first: without this the assertion
				// below could pass against a Portainer that never stored the
				// credential at all.
				rawGit, ok := rawCustomTemplate(t, leg.Name, id)["GitConfig"].(map[string]any)
				if !ok {
					t.Fatalf("the raw API returned no GitConfig for template %d", id)
				}
				rawAuth, ok := rawGit["Authentication"].(map[string]any)
				if !ok {
					t.Fatalf("the raw API returned no GitConfig.Authentication for template %d: this test cannot show redaction removed something the server never sent: %v", id, rawGit)
				}
				if got, _ := rawAuth["Username"].(string); got != username {
					t.Fatalf("the raw API returned GitConfig.Authentication.Username %q, want %q", got, username)
				}

				// ... and now the same field through each action that
				// returns a template.
				for action, out := range map[string]map[string]any{
					"custom_templates.create_repository": created,
					"custom_templates.inspect":           callAction[map[string]any](t, session, surface, "custom_templates.inspect", map[string]any{"id": id}),
				} {
					gitConfig, ok := out["GitConfig"].(map[string]any)
					if !ok {
						t.Errorf("%s returned no GitConfig: %v", action, out)
						continue
					}
					if auth, present := gitConfig["Authentication"]; present {
						t.Errorf("%s returned GitConfig.Authentication %v: the raw API carries a username here and internal/redact must drop the whole object", action, auth)
					}
				}

				listed := callAction[[]map[string]any](t, session, surface, "custom_templates.list", map[string]any{
					"type": []int{customTemplateFixtureType},
				})
				for _, entry := range listed {
					entryID, ok := entry["Id"].(float64)
					if !ok || int(entryID) != id {
						continue
					}
					gitConfig, ok := entry["GitConfig"].(map[string]any)
					if !ok {
						t.Errorf("custom_templates.list entry for template %d carries no GitConfig: %v", id, entry)
						continue
					}
					if auth, present := gitConfig["Authentication"]; present {
						t.Errorf("custom_templates.list returned GitConfig.Authentication %v for template %d", auth, id)
					}
				}
			})
		}
	}
}

// TestCustomTemplates_GitFetch_ReplacesTheStoredFileWithTheRemotesNewCommit
// is the proof that git_fetch really re-reads the remote.
//
// A commit is pushed into the fixture repository between the create and the
// fetch, and the fetch must answer with the NEW content. Nothing weaker
// discriminates: an implementation answering from the copy Portainer stored
// at create time — the exact stale-cache behaviour section 2.4 of
// docs/api-divergences.md records for docker.service_image_status, which
// kept a Stage A test green against a service that no longer existed — would
// satisfy a "did it answer 200" test and fail this one.
//
// It also reads the stored file back afterwards, because git_fetch is
// flagged destructive for overwriting it: the response alone would not show
// that the stored copy moved.
//
// Nothing here runs in parallel, and that is load-bearing rather than
// conservative: all six (leg, surface) pairs share one mutable repository,
// and two concurrent pushes would each assert against content the other had
// already replaced. The repository is separate from the read-only one every
// other test clones precisely so these pushes cannot disturb them.
func TestCustomTemplates_GitFetch_ReplacesTheStoredFileWithTheRemotesNewCommit(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				session := sessions.For(t, surface, leg.Name)

				title := uniqueName("template-git-fetch")
				created := callAction[map[string]any](t, session, surface, "custom_templates.create_repository", map[string]any{
					"title":                       title,
					"description":                 "cloned by " + t.Name(),
					"repositoryUrl":               harness.GitFixtureMutableRepositoryURL,
					"composeFilePathInRepository": harness.GitFixtureConfigFilePath,
					"platform":                    1,
					"type":                        2,
				})
				id := templateID(t, created, "custom_templates.create_repository")
				registerCustomTemplateCleanup(t, leg.Name, id)

				// What Portainer holds before the push, checked against
				// what the repository serves right now rather than against
				// the seeded constant: this test pushes to that repository,
				// so its content is whatever the last run left there, and a
				// constant here would pass exactly once per `make e2e-up`.
				before := callAction[map[string]any](t, session, surface, "custom_templates.file", map[string]any{"id": id})
				beforeContent := templateFileContent(t, surface, "custom_templates.file", before)
				if served := gitFixtureMutableContent(t); beforeContent != served {
					t.Fatalf("the freshly cloned template's stack file is\n%q\nwant what the mutable fixture repository currently serves\n%q", beforeContent, served)
				}

				pushed := customTemplateStackFile(uniqueName("git-fetch-revision"))
				// The assertion below is only discriminating while these two
				// differ: a fetch that answered from the copy stored at
				// create time would otherwise be indistinguishable from a
				// real one.
				if pushed == beforeContent {
					t.Fatalf("the content about to be pushed is already what the template stores: this test would prove nothing")
				}
				gitFixtureCommit(t, pushed)

				fetched := callAction[map[string]any](t, session, surface, "custom_templates.git_fetch", map[string]any{"id": id})
				if got := templateFileContent(t, surface, "custom_templates.git_fetch", fetched); got != pushed {
					t.Errorf("custom_templates.git_fetch answered\n%q\nwant the commit just pushed\n%q", got, pushed)
				}

				after := callAction[map[string]any](t, session, surface, "custom_templates.file", map[string]any{"id": id})
				if got := templateFileContent(t, surface, "custom_templates.file", after); got != pushed {
					t.Errorf("after custom_templates.git_fetch the stored file is\n%q\nwant the commit just pushed\n%q", got, pushed)
				}
			})
		}
	}
}

// TestCustomTemplates_GitFetch_FailsOnATemplateWithNoGitConfiguration is the
// negative half: git_fetch is only meaningful for a git-backed template, and
// the action's narrative says so. Measured live, an inline template answers
// 400 "Git configuration does not exist in this custom template" — so the
// server does look, and a test that only ever called it on a git-backed
// template could not tell that apart from a route that fetches nothing and
// always answers 200.
func TestCustomTemplates_GitFetch_FailsOnATemplateWithNoGitConfiguration(t *testing.T) {
	id := createCustomTemplateFixture(t, "CE", uniqueName("template-inline"))
	assertActionFails(t, sessions.For(t, "dynamic", "CE"), "dynamic", "custom_templates.git_fetch", map[string]any{"id": id})
}

// safeModeMutations are the six mutating actions of this domain, each with
// an input good enough that safe mode is the only reason it does not
// execute. inputFor takes the id of a real, existing template, so nothing
// here could be refused for naming a template that does not exist — the
// interception has to be what stops it.
//
// The `kind` field is asserted per action rather than ignored, because it is
// derived from the ActionSpec flags this domain argued over at length:
// delete and git_fetch are Destructive and must preview as "destructive",
// the other four as "mutating".
var safeModeMutations = []struct {
	action   string
	kind     string
	inputFor func(id int, title, password string) map[string]any
}{
	{
		action: "custom_templates.create_string", kind: "mutating",
		inputFor: func(_ int, title, _ string) map[string]any {
			return map[string]any{
				"title": title, "description": "safe mode", "fileContent": customTemplateStackFile(title),
				"platform": 1, "type": 2,
			}
		},
	},
	{
		action: "custom_templates.create_file", kind: "mutating",
		inputFor: func(_ int, title, _ string) map[string]any {
			return map[string]any{
				"title": title, "description": "safe mode", "note": "safe mode",
				"file": customTemplateStackFile(title), "platform": 1, "type": 2,
			}
		},
	},
	{
		// The one that carries a credential, which is why the preview's
		// values-versus-names property is asserted at all.
		action: "custom_templates.create_repository", kind: "mutating",
		inputFor: func(_ int, title, password string) map[string]any {
			return map[string]any{
				"title": title, "description": "safe mode",
				"repositoryUrl":               harness.GitFixtureRepositoryURL,
				"composeFilePathInRepository": harness.GitFixtureConfigFilePath,
				"platform":                    1, "type": 2,
				"repositoryAuthentication": true,
				"repositoryUsername":       "e2e-git-user",
				"repositoryPassword":       password,
			}
		},
	},
	{
		action: "custom_templates.update", kind: "mutating",
		inputFor: func(id int, title, password string) map[string]any {
			return map[string]any{
				"id": id, "title": title, "description": "safe mode",
				"fileContent": customTemplateStackFile(title), "platform": 1, "type": 2,
				"repositoryUrl":            harness.GitFixtureRepositoryURL,
				"repositoryAuthentication": true,
				"repositoryUsername":       "e2e-git-user",
				"repositoryPassword":       password,
			}
		},
	},
	{
		action: "custom_templates.git_fetch", kind: "destructive",
		inputFor: func(id int, _, _ string) map[string]any {
			return map[string]any{"id": id}
		},
	},
	{
		action: "custom_templates.delete", kind: "destructive",
		inputFor: func(id int, _, _ string) map[string]any {
			return map[string]any{"id": id}
		},
	},
}

// TestSafeMode_CustomTemplates_MutatingActionsArePreviewedAndNothingChanges
// covers all six mutating actions of this domain under safe mode, on all
// three surfaces.
//
// Three properties, and the third is what makes the first two worth
// anything:
//
//  1. the call comes back as a preview naming the action and its kind;
//  2. the preview lists the input's FIELD NAMES and never its values —
//     asserted against a real credential in the create_repository and update
//     inputs, since that is the case the property exists for
//     (internal/tools/register.go's safeModePreview);
//  3. nothing was written: the template named by the destructive inputs is
//     still there with its original content, and no template by the name the
//     create inputs used ever appeared.
//
// Property 3 is read back through the raw Portainer API, not through any
// surface, for the same reason assertTagAbsent is: a surface that hid or
// previewed a call and executed it anyway would satisfy 1 and 2 identically.
//
// Community Edition only, like TestSafeMode_TagsCreate_DoesNotActuallyCreateATag:
// custom_templates exists on both editions and safe-mode interception has
// nothing to do with edition, so it is proven against the leg every estate
// carries.
func TestSafeMode_CustomTemplates_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	for _, surface := range surfaceNames {
		for _, mutation := range safeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()
				session := sessions.SafeMode(t, surface)

				existingTitle := uniqueName("template-safe-existing")
				existingID := createCustomTemplateFixture(t, "CE", existingTitle)
				existingContent := rawCustomTemplateFile(t, "CE", existingID)

				title := uniqueName("template-safe")
				password := "e2e-git-" + uniqueName("secret")
				input := mutation.inputFor(existingID, title, password)

				toolName, args := actionCallParams(t, surface, mutation.action, input)
				text := toolResultText(callToolExpectingSuccess(t, session, toolName, args))

				var preview map[string]any
				if err := json.Unmarshal([]byte(text), &preview); err != nil {
					t.Fatalf("decoding the safe-mode preview %q: %v", text, err)
				}
				if safeMode, _ := preview["safe_mode"].(bool); !safeMode {
					t.Fatalf("%s under safe mode = %v, want a safe_mode preview, not a real call", mutation.action, preview)
				}
				if got, _ := preview["action"].(string); got != mutation.action {
					t.Errorf("safe-mode preview action = %q, want %q", got, mutation.action)
				}
				if got, _ := preview["kind"].(string); got != mutation.kind {
					t.Errorf("safe-mode preview kind = %q, want %q (from this action's own ActionSpec flags)", got, mutation.kind)
				}

				// Field names, never values. Both halves are checked: the
				// absence of the secret alone would pass against a preview
				// that described nothing at all.
				wouldCall, ok := preview["would_call"].(map[string]any)
				if !ok {
					t.Fatalf("safe-mode preview carries no would_call: %v", preview)
				}
				fields := wouldCall["input_fields"]
				for name := range input {
					if !previewNamesField(fields, name) {
						t.Errorf("safe-mode preview's input_fields %v does not name %q, which this call sent", fields, name)
					}
				}
				if strings.Contains(text, password) {
					t.Errorf("the safe-mode preview echoed the plaintext repositoryPassword back to the caller: %s", text)
				}

				// Nothing was written.
				assertCustomTemplateAbsent(t, "CE", title, "safe mode intercepting "+mutation.action)
				after := rawCustomTemplate(t, "CE", existingID)
				if got, _ := after["Title"].(string); got != existingTitle {
					t.Errorf("template %d is now titled %q, want %q: safe mode let %s through", existingID, got, existingTitle, mutation.action)
				}
				if got := rawCustomTemplateFile(t, "CE", existingID); got != existingContent {
					t.Errorf("template %d's stack file changed under safe mode's %s:\ngot  %q\nwant %q", existingID, mutation.action, got, existingContent)
				}
			})
		}
	}
}

// callToolExpectingSuccess calls a tool and fails the test unless it comes
// back as a non-error result, returning the result itself.
//
// callTool cannot serve the safe-mode tests: it decodes the result into a
// typed value, and these assertions need the raw text, to prove a secret
// does not appear anywhere in it — not merely that it is absent from the
// fields a struct happens to model.
func callToolExpectingSuccess(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) returned an error result: %s", name, toolResultText(res))
	}
	return res
}

// previewNamesField reports whether the preview's input_fields list carries
// name. The list arrives as []any of strings through a generic JSON decode.
func previewNamesField(fields any, name string) bool {
	list, ok := fields.([]any)
	if !ok {
		return false
	}
	for _, field := range list {
		if got, _ := field.(string); got == name {
			return true
		}
	}
	return false
}

// assertCustomTemplateAbsent fails t if a template named title exists on
// edition ed's server, read directly through the Portainer API rather than
// through any MCP surface — the one check a surface cannot satisfy by
// hiding, refusing or previewing a call and executing it anyway.
func assertCustomTemplateAbsent(t *testing.T, ed, title, reason string) {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	templates, err := listAllCustomTemplates(ctx, client)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, template := range templates {
		if template.Title != nil && *template.Title == title {
			t.Errorf("custom template %q exists on the %s server despite %s", title, ed, reason)
		}
	}
}

// listAllCustomTemplates returns every custom template on client's server,
// issuing ONE REQUEST PER STACK TYPE rather than one request naming all
// three.
//
// That is not a stylistic choice. The vendored specification declares the
// type parameter `explode: false`, so the generated client encodes a
// three-element slice as `type=1,2,3` — and Portainer's own handler parses
// each value with strconv.Atoi and answers
// 400 "Invalid Custom template type: Failed parsing template type:
// strconv.Atoi: parsing \"1,2,3\": invalid syntax". Measured live; see
// docs/api-divergences.md section 6.7 and
// TestCustomTemplates_List_RefusesMoreThanOneTypeAtATime, which pins the
// behaviour so a Portainer that starts accepting the comma form (or a
// client that stops sending it) is noticed rather than silently changing
// what this sweep can see.
func listAllCustomTemplates(ctx context.Context, client *portainer.Client) ([]apigen.PortainereeCustomTemplate, error) {
	var all []apigen.PortainereeCustomTemplate
	for _, stackType := range customTemplateListTypes {
		resp, err := client.API.CustomTemplateListWithResponse(ctx, &apigen.CustomTemplateListParams{
			Type: []apigen.CustomTemplateListParamsType{apigen.CustomTemplateListParamsType(stackType)},
		})
		if err != nil {
			return nil, fmt.Errorf("list custom templates of type %d: %w", stackType, err)
		}
		if err := toolutil.Check(resp); err != nil {
			return nil, fmt.Errorf("list custom templates of type %d: %w", stackType, err)
		}
		if resp.JSON200 != nil {
			all = append(all, *resp.JSON200...)
		}
	}
	return all, nil
}

// deleteOrphanCustomTemplates is this domain's cross-run sweep, registered in
// fixtures_test.go's orphanSweeps. Every template these tests create is named
// with uniqueName, so a run that died between creating one and its own
// cleanup leaves a template the next run can recognise and remove.
func deleteOrphanCustomTemplates(ctx context.Context, client *portainer.Client, now time.Time) error {
	templates, err := listAllCustomTemplates(ctx, client)
	if err != nil {
		return err
	}

	var errs []error
	for _, template := range templates {
		if template.Title == nil || template.Id == nil || !strings.HasPrefix(*template.Title, orphanPrefix) {
			continue
		}
		if !isOrphanEligible(*template.Title, now) {
			continue
		}
		delResp, err := client.API.CustomTemplateDeleteWithResponse(ctx, *template.Id)
		if err != nil {
			errs = append(errs, fmt.Errorf("delete orphan custom template %q: %w", *template.Title, err))
			continue
		}
		if err := toolutil.Check(delResp); err != nil {
			errs = append(errs, fmt.Errorf("delete orphan custom template %q: %w", *template.Title, err))
		}
	}
	return errors.Join(errs...)
}
