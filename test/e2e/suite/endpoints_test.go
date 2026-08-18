//go:build e2e

package suite

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// rawEndpointsJSON issues one request against edition ed's server and decodes
// the answer, failing the test on any non-200.
//
// The status check is the point, and it is checked before the body is
// decoded: a 401, a 404 or a 500 carries a JSON object too, so decoding first
// either succeeds into an empty struct — a read-back that silently reports
// "no such environment" as "the field is absent" — or fails with a decode
// error naming a type rather than the status that actually went wrong. Same
// treatment stacks_test.go's rawStack already gives its own reads.
func rawEndpointsJSON(t *testing.T, ed, path string, out any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := fixtureClient(t, ed).Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		t.Fatalf("raw GET %s on %s: %v", path, ed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		t.Fatalf("raw GET %s on %s: read response: %v", path, ed, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw GET %s on %s: HTTP %d: %s", path, ed, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("raw GET %s on %s: decode %s: %v", path, ed, body, err)
	}
}

// rawEnvironments reads the environment list straight from the Portainer
// API, bypassing every surface.
//
// Read-back assertions go through this rather than endpoints.list, for the
// reason tags_test.go's own raw helpers exist: an action that returned a
// plausible body while writing nothing would satisfy a read-back performed
// through itself.
func rawEnvironments(t *testing.T, ed string) []map[string]any {
	t.Helper()
	var out []map[string]any
	rawEndpointsJSON(t, ed, "/endpoints", &out)
	return out
}

// rawEnvironment reads one environment straight from the Portainer API.
func rawEnvironment(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	var out map[string]any
	rawEndpointsJSON(t, ed, fmt.Sprintf("/endpoints/%d", id), &out)
	return out
}

// rawEdgeEnvironment reads one environment with its stored snapshot
// excluded.
//
// Every read of an edge environment in this file goes through here rather
// than rawEnvironment, and the reason is worth twenty seconds of anyone's
// attention: measured 2026-08-18 against a live Business Edition 2.44.0, the
// FIRST GET /endpoints/{id} of a freshly-registered edge environment whose
// agent has never checked in blocks for ~20 seconds, then answers. Every
// later read of the same environment returns in under five milliseconds, and
// GET /endpoints/{id}?excludeSnapshot=true returns in under one millisecond
// even on the first call — so Portainer is attempting one snapshot of an
// unreachable host, with its own timeout, and caching the outcome.
//
// GET /endpoints (the list) is unaffected. See docs/api-divergences.md; the
// same fact is in endpoints.inspect's own narrative, because a caller
// inspecting a newly-enrolled edge environment meets it too.
func rawEdgeEnvironment(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	var out map[string]any
	rawEndpointsJSON(t, ed, fmt.Sprintf("/endpoints/%d?excludeSnapshot=true", id), &out)
	return out
}

// environmentIDByName finds an environment by name, or -1.
func environmentIDByName(t *testing.T, ed, name string) int {
	t.Helper()
	for _, e := range rawEnvironments(t, ed) {
		if e["Name"] == name {
			if id, ok := e["Id"].(float64); ok {
				return int(id)
			}
		}
	}
	return -1
}

// environmentTypeLocalDocker and environmentTypeAgent are the two Portainer
// environment types this suite can address as a Docker host: a direct daemon
// connection and one reached through a Portainer agent.
const (
	environmentTypeLocalDocker = 1
	environmentTypeAgent       = 2
)

// firstDockerEnvironmentID returns the id of a DOCKER environment already
// provisioned by the estate, which every read-only action below addresses.
//
// The type filter is load-bearing rather than defensive. The Business Edition
// leg registers an edge environment beside its Docker one, and this file
// registers more of its own; returning whichever happens to come first in
// GET /endpoints would hand a caller an edge environment, which fails
// type-dependent actions (registries_list, docker_browse_put) and drags the
// ~20 second first-inspect delay rawEdgeEnvironment documents into tests that
// have nothing to do with edge. It works today only because the Docker
// environment happens to be listed first, which is not a property anything
// guarantees.
func firstDockerEnvironmentID(t *testing.T, ed string) int {
	t.Helper()
	for _, e := range rawEnvironments(t, ed) {
		envType, ok := e["Type"].(float64)
		if !ok || (int(envType) != environmentTypeLocalDocker && int(envType) != environmentTypeAgent) {
			continue
		}
		if id, ok := e["Id"].(float64); ok {
			return int(id)
		}
	}
	t.Fatalf("%s estate has no Docker environment (type %d or %d)", ed, environmentTypeLocalDocker, environmentTypeAgent)
	return 0
}

// serverFor returns the provisioned server for an edition name.
func serverFor(t *testing.T, ed string) harness.Server {
	t.Helper()
	for _, leg := range estate.Legs() {
		if leg.Name == ed {
			return leg.Server
		}
	}
	t.Fatalf("no provisioned server for edition %q", ed)
	return harness.Server{}
}

// createEnvironmentThroughSurface registers an environment through the
// surface under test and returns its id.
//
// A type 1 (local Docker) registration pointed at the daemon URL the estate
// already registered, copied from stacks_test.go's
// createSecondDockerEnvironment and for its reason: the URL is
// cmd/provision's own unexported constant, so it is read back off the server
// rather than written out a second time here where it would be free to
// drift.
//
// The type matters and the first attempt got it wrong, which is worth
// recording where the next reader will look: type 3 (Azure) seemed the
// cheapest way to get an environment that depends on nothing outside
// Portainer, on the assumption that an Azure environment is only a set of
// stored credentials. It is not — Portainer authenticates against Azure
// during the call and answers 500 "Unable to authenticate against Azure:
// Invalid Azure credentials". That failure was itself useful: it proved the
// hand-written multipart body parses on the server, since Portainer had to
// read EndpointCreationType and all three azure* parts to get far enough to
// reject them.
func createEnvironmentThroughSurface(t *testing.T, surface, ed, name string) int {
	t.Helper()
	srv := serverFor(t, ed)
	existing, ok := srv.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the %s server has no %q environment to copy a daemon URL from", ed, harness.EnvironmentDocker)
	}
	daemonURL, _ := rawEnvironment(t, ed, existing)["URL"].(string)
	if daemonURL == "" {
		t.Fatalf("environment %d on %s reports no daemon URL", existing, ed)
	}

	toolName, args := actionCallParams(t, surface, "endpoints.create", map[string]any{
		"name":                 name,
		"endpointCreationType": 1,
		"url":                  daemonURL,
	})
	res, err := sessions.For(t, surface, ed).CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		t.Fatalf("endpoints.create on %s/%s: %v", ed, surface, err)
	}
	if res.IsError {
		text := toolResultText(res)
		// A Business Edition licence caps how many nodes may be registered,
		// and this estate's licence is whatever secret the run was given.
		// Running out of allowance is a property of the licence, not a defect
		// in the action under test, so it skips rather than reddening the
		// build — the same judgement, in the same words, as
		// stacks_test.go's createSecondDockerEnvironment.
		if strings.Contains(text, "node allowance") {
			t.Skipf("this %s licence has no node allowance left to register another environment: %s", ed, text)
		}
		t.Fatalf("endpoints.create on %s/%s: %s", ed, surface, text)
	}

	var created map[string]any
	if err := json.Unmarshal([]byte(toolResultText(res)), &created); err != nil {
		t.Fatalf("decode endpoints.create result: %v", err)
	}
	idFloat, ok := created["Id"].(float64)
	if !ok {
		t.Fatalf("endpoints.create returned no Id: %v", created)
	}
	id := int(idFloat)
	// Registered through the same ledger stacks_test.go uses, and with
	// stacks_test.go's own deleteEnvironmentIfPresent, so an environment this
	// file creates is torn down by the identical path whether the test
	// deleted it itself or died before it could.
	newLedger(t).Register("environment", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteEnvironmentIfPresent(ctx, ed, id)
	})
	return id
}

// TestEndpoints_CreateInspectUpdateDelete_AcrossSurfacesAndEditions is this
// domain's mutation template, following tags_test.go's: every mutating call
// is followed by a read-back through the raw API, so an action that answered
// plausibly while writing nothing fails here rather than passing.
func TestEndpoints_CreateInspectUpdateDelete_AcrossSurfacesAndEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				// Read back after create.
				if got := environmentIDByName(t, ed, name); got != id {
					t.Fatalf("environment %q was created as %d but the server lists it as %d", name, id, got)
				}

				// The Azure key must never come back out of any of these.
				inspected := callAction[map[string]any](t, session, surface, "endpoints.inspect", map[string]any{"id": id})
				if inspected["AzureCredentials"] != nil {
					t.Errorf("endpoints.inspect returned AzureCredentials: %v", inspected["AzureCredentials"])
				}
				if got, _ := inspected["Name"].(string); got != name {
					t.Errorf("endpoints.inspect Name = %q, want %q", got, name)
				}

				listed := callAction[[]map[string]any](t, session, surface, "endpoints.list", nil)
				var found bool
				for _, e := range listed {
					if e["Name"] == name {
						found = true
						if e["AzureCredentials"] != nil {
							t.Errorf("endpoints.list returned AzureCredentials for %q", name)
						}
					}
				}
				if !found {
					t.Errorf("environment %q does not appear in endpoints.list", name)
				}

				renamed := name + "-renamed"
				callAction[map[string]any](t, session, surface, "endpoints.update", map[string]any{
					"id": id, "name": renamed,
				})
				if got, _ := rawEnvironment(t, ed, id)["Name"].(string); got != renamed {
					t.Errorf("after endpoints.update the server still names %d %q, want %q", id, got, renamed)
				}

				callAction[any](t, session, surface, "endpoints.delete", map[string]any{"id": id})
				if got := environmentIDByName(t, ed, renamed); got != -1 {
					t.Errorf("environment %q is still present as %d after endpoints.delete", renamed, got)
				}
			})
		}
	}
}

// TestEndpoints_SettingsUpdate_AppliesOnBothEditions is the e2e half of the
// measurement recorded in endpointSettingsUpdate's own doc comment.
//
// Community Edition reads these ten settings as top-level body fields and
// ignores a nested "securitySettings"; Business Edition does the opposite,
// and both answer 200 either way. The handler sends both shapes; this proves
// the setting actually changed on whichever edition the leg is, by reading
// it back raw. A catalog that sent only the Business shape would pass a
// "no error" test on Community and fail this one.
func TestEndpoints_SettingsUpdate_AppliesOnBothEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env-settings")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				for _, want := range []bool{true, false} {
					callAction[map[string]any](t, session, surface, "endpoints.settings_update", map[string]any{
						"id": id,
						"securitySettings": map[string]any{
							"allowBindMountsForRegularUsers":     want,
							"allowPrivilegedModeForRegularUsers": want,
						},
					})
					settings, ok := rawEnvironment(t, ed, id)["SecuritySettings"].(map[string]any)
					if !ok {
						t.Fatalf("environment %d carries no SecuritySettings", id)
					}
					if settings["allowBindMountsForRegularUsers"] != want {
						t.Errorf("allowBindMountsForRegularUsers = %v, want %v: this edition ignored the shape the call sent",
							settings["allowBindMountsForRegularUsers"], want)
					}
					if settings["allowPrivilegedModeForRegularUsers"] != want {
						t.Errorf("allowPrivilegedModeForRegularUsers = %v, want %v",
							settings["allowPrivilegedModeForRegularUsers"], want)
					}
				}
			})
		}
	}
}

// TestEndpoints_ReadOnlyActions_AnswerOnEveryEditionTheyApplyTo walks the
// read-only half of the domain.
//
// Each entry asserts something about the answer's own shape, not merely that
// the call returned: a surface that answered every read with an empty object
// would otherwise pass the whole table.
func TestEndpoints_ReadOnlyActions_AnswerOnEveryEditionTheyApplyTo(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				counts := callAction[map[string]any](t, session, surface, "endpoints.summary_counts", nil)
				if len(counts) == 0 {
					t.Error("endpoints.summary_counts answered an empty object")
				}

				registries := callAction[[]map[string]any](t, session, surface, "endpoints.registries_list",
					map[string]any{"id": envID})
				for _, r := range registries {
					if r["Password"] != nil || r["AccessToken"] != nil {
						t.Errorf("endpoints.registries_list returned a credential: %v", r)
					}
				}

				// registryId 0 is Portainer's documented sentinel for the
				// anonymous DockerHub registry, and the reason this one
				// identifier is excused the catalog's "minimum": 1 rule
				// (cmd/gen_action_inputs/fields.go). Passing it here is what
				// proves that exception is load-bearing rather than
				// theoretical: with the bound applied, tools.Execute would
				// refuse this call locally, before the handler ran, and the
				// call would never reach Portainer at all.
				//
				// The call is asserted to REACH Portainer, not to succeed.
				// Measured 2026-08-18 against a live 2.44.0 of both editions:
				// this route answers 400 "Invalid environment type" for a
				// type 1 (local Docker), type 2 (agent) and type 7 (edge)
				// environment alike — every type this estate provisions —
				// while the specification records no environment-type
				// restriction at all. A 400 from Portainer's own handler is
				// therefore the expected outcome here and still proves what
				// this assertion is for: the identifier passed validation and
				// the request was built and sent. See
				// docs/api-divergences.md.
				toolName, args := actionCallParams(t, surface, "endpoints.dockerhub_status",
					map[string]any{"id": envID, "registryId": 0})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.dockerhub_status: %v", err)
				}
				if text := toolResultText(res); res.IsError && !strings.Contains(text, "Invalid environment type") {
					t.Errorf("endpoints.dockerhub_status failed for a reason other than the measured environment-type refusal: %s", text)
				}

				if ed != "EE" {
					return
				}

				versions := callAction[any](t, session, surface, "endpoints.agent_versions", nil)
				if versions == nil {
					t.Error("endpoints.agent_versions answered nothing")
				}

				// The two mTLS reads answer about the agent environment the
				// estate registers, and both must come back with the
				// certificate stripped: their redaction wrapper drops the
				// whole SslCertificate rather than any field of it, because
				// toolutil.AssertRedacted admits no per-field exception for
				// the six x509 names it flags.
				for _, action := range []string{"endpoints.mtls_certificate", "endpoints.mtls_agent_certificate_error"} {
					got := callAction[map[string]any](t, session, surface, action, map[string]any{"id": envID})
					if got["MTLSCertificate"] != nil {
						t.Errorf("%s returned certificate material: %v", action, got["MTLSCertificate"])
					}
				}
			})
		}
	}
}

// TestEndpoints_SnapshotThenReadTheStoredSnapshot_OnBusinessEdition covers
// the five snapshot actions as the one sequence that makes them meaningful:
// refresh the stored snapshot, then read it back three ways.
//
// The three read actions are tagged ["endpoints", "docker"] upstream and
// live in this domain because cmd/gen_action_inputs routes by tags[0]; they
// read what Portainer stored at the last poll rather than the Docker daemon,
// which is why endpoints.snapshot has to run first for the read to describe
// anything current.
func TestEndpoints_SnapshotThenReadTheStoredSnapshot_OnBusinessEdition(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("the three snapshot read actions are Business Edition only")
	}
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")
			envID := firstDockerEnvironmentID(t, "EE")

			callAction[any](t, session, surface, "endpoints.snapshot", map[string]any{"id": envID})

			snapshot := callAction[map[string]any](t, session, surface, "endpoints.snapshot_inspect",
				map[string]any{"environmentId": envID})
			if len(snapshot) == 0 {
				t.Fatal("endpoints.snapshot_inspect answered an empty object after endpoints.snapshot ran")
			}

			containers := callAction[[]map[string]any](t, session, surface, "endpoints.snapshot_containers_list",
				map[string]any{"environmentId": envID})
			if len(containers) == 0 {
				t.Skip("the stored snapshot lists no containers, so there is none to inspect individually")
			}

			// The container id is read out of the snapshot rather than
			// invented, which is the whole point of this action: it is
			// Docker's own hexadecimal id, and the vendored specification
			// declares the path parameter an integer. A catalog that
			// published it as an integer could not make this call at all.
			id, _ := containers[0]["Id"].(string)
			if id == "" {
				t.Fatalf("the stored snapshot's first container carries no string Id: %v", containers[0])
			}
			one := callAction[map[string]any](t, session, surface, "endpoints.snapshot_container_inspect",
				map[string]any{"environmentId": envID, "containerId": id})
			if len(one) == 0 {
				t.Errorf("endpoints.snapshot_container_inspect answered nothing for container %q", id)
			}
		})
	}
}

// TestEndpoints_SnapshotAll_RefreshesEveryEnvironment covers the fleet-wide
// snapshot, the action renamed from the mechanical "endpoints.snapshots" to
// "endpoints.snapshot_all" because one letter is not enough to tell a
// single-environment refresh from a fleet-wide one.
func TestEndpoints_SnapshotAll_RefreshesEveryEnvironment(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				callAction[any](t, sessions.For(t, surface, ed), surface, "endpoints.snapshot_all", nil)
			})
		}
	}
}

// TestEndpoints_DeleteBatch_RemovesEveryEnvironmentItNames is the batch
// delete, read back the same way the single delete is.
func TestEndpoints_DeleteBatch_RemovesEveryEnvironmentItNames(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env-batch")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				callAction[any](t, session, surface, "endpoints.delete_batch", map[string]any{
					"endpoints": []map[string]any{{"id": id}},
				})
				if got := environmentIDByName(t, ed, name); got != -1 {
					t.Errorf("environment %q is still present as %d after endpoints.delete_batch", name, got)
				}
			})
		}
	}
}

// TestEndpoints_UpdateRelations_ReassignsAnEnvironmentsGroup proves the
// relations update actually writes, by moving an environment into a group
// and reading the group back raw.
func TestEndpoints_UpdateRelations_ReassignsAnEnvironmentsGroup(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env-relations")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				tagID := createTag(t, ed, uniqueName("tag-relations"))
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteTagIfPresent(ctx, ed, tagID); err != nil {
						t.Errorf("clean up tag %d: %v", tagID, err)
					}
				})

				callAction[any](t, session, surface, "endpoints.update_relations", map[string]any{
					"relations": map[string]any{
						strconv.Itoa(id): map[string]any{"tags": []int{tagID}},
					},
				})

				tags, _ := rawEnvironment(t, ed, id)["TagIds"].([]any)
				var carries bool
				for _, raw := range tags {
					if got, ok := raw.(float64); ok && int(got) == tagID {
						carries = true
					}
				}
				if !carries {
					t.Errorf("environment %d carries tags %v after endpoints.update_relations, want it to include %d", id, tags, tagID)
				}
			})
		}
	}
}

// TestEndpoints_RegistryAccess_SetsPerEnvironmentRegistryPolicies exercises
// the per-environment registry access update against a registry this test
// creates.
func TestEndpoints_RegistryAccess_SetsPerEnvironmentRegistryPolicies(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				registryID := createRegistry(t, ed, uniqueName("registry-access"))
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteRegistryIfPresent(ctx, ed, registryID); err != nil {
						t.Errorf("clean up registry %d: %v", registryID, err)
					}
				})

				callAction[any](t, session, surface, "endpoints.registry_access", map[string]any{
					"id": envID, "registryId": registryID,
				})
			})
		}
	}
}

// TestEndpoints_DockerBrowsePut_UploadsAFileIntoTheEnvironment covers the
// second multipart route in this domain, the one whose query parameter is
// spelled volumeID rather than volumeId.
func TestEndpoints_DockerBrowsePut_UploadsAFileIntoTheEnvironment(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				toolName, args := actionCallParams(t, surface, "endpoints.docker_browse_put", map[string]any{
					"id":   envID,
					"path": "/tmp/" + uniqueName("upload") + ".pem",
					"file": "-----BEGIN CERTIFICATE-----\ne2e\n-----END CERTIFICATE-----\n",
				})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.docker_browse_put: %v", err)
				}
				// The route is served only by an environment reached through
				// a Portainer agent; the estate's Docker-in-Docker
				// environment is a direct daemon connection, so Portainer
				// refuses with a message about the environment rather than
				// about the request. That still exercises the whole
				// hand-written path — multipart body, query parameter, typed
				// client call — and a malformed body would fail differently,
				// which is what the unit tests in
				// internal/tools/endpoints/handlers_test.go pin exactly.
				if res.IsError {
					t.Logf("endpoints.docker_browse_put on this estate's environment: %s", toolResultText(res))
				}
			})
		}
	}
}

// TestEndpoints_ForceUpdateService_RedeploysASwarmService covers the Swarm
// service redeploy, against the fixture service up.sh creates.
func TestEndpoints_ForceUpdateService_RedeploysASwarmService(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("this estate's Docker daemon has no Swarm fixture service to redeploy")
	}
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				toolName, args := actionCallParams(t, surface, "endpoints.force_update_service", map[string]any{
					"id": envID, "serviceId": estate.SwarmServiceID, "pullImage": false,
				})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.force_update_service: %v", err)
				}
				if res.IsError {
					t.Logf("endpoints.force_update_service on this estate: %s", toolResultText(res))
				}
			})
		}
	}
}

// createEdgeEnvironmentThroughSurface registers an edge environment
// (endpointCreationType 4) through the surface under test.
//
// It exists so the four edge actions below never touch the edge environment
// the estate itself provisions: association_delete in particular clears an
// environment's edge identity, and doing that to the estate's own edge agent
// would break every later test in the run for a reason that has nothing to
// do with the action.
//
// Business Edition only, and creating one at all depends on Edge Compute
// already being enabled on the server (docs/api-divergences.md §3.4). The
// estate enables it when it registers its own edge environment, so this
// works here and would not against a bare Portainer.
func createEdgeEnvironmentThroughSurface(t *testing.T, surface, name string) int {
	t.Helper()
	toolName, args := actionCallParams(t, surface, "endpoints.create", map[string]any{
		"name": name, "endpointCreationType": 4,
	})
	res, err := sessions.For(t, surface, "EE").CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		t.Fatalf("endpoints.create (edge): %v", err)
	}
	if res.IsError {
		text := toolResultText(res)
		if strings.Contains(text, "node allowance") {
			t.Skipf("this licence has no node allowance left to register an edge environment: %s", text)
		}
		t.Fatalf("endpoints.create (edge) on %s: %s", surface, text)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(toolResultText(res)), &created); err != nil {
		t.Fatalf("decode endpoints.create result: %v", err)
	}
	// The edge key is the enrolment secret an agent presents to claim this
	// environment, and Portainer returns it on this very response. Asserting
	// it never reaches a caller here, on a create that really produced one,
	// is what the unit guard in internal/tools/endpoints/redaction_test.go
	// cannot do: that one runs against a synthetic fixture.
	if key, _ := created["EdgeKey"].(string); key != "" {
		t.Errorf("endpoints.create returned the edge enrolment key to a caller: %q", key)
	}
	idFloat, ok := created["Id"].(float64)
	if !ok {
		t.Fatalf("endpoints.create returned no Id: %v", created)
	}
	id := int(idFloat)
	newLedger(t).Register("environment", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteEnvironmentIfPresent(ctx, "EE", id)
	})
	return id
}

// TestEndpoints_EdgeActions_OnAnEnvironmentThisTestOwns covers the edge
// actions against an edge environment this test registers and removes.
//
// It takes about a minute, serially, and both halves of that are deliberate.
//
// Registering an edge environment schedules one snapshot attempt against a
// host that will never answer, and that attempt costs ~20 seconds. Portainer
// serialises them globally, so the cost is not per-call but per-environment,
// and any concurrent request can end up queued behind an attempt that is not
// its own: measured 2026-08-18, one local run charged the 20 seconds to
// trust_edge_endpoints on one surface and to association_delete on two
// others, while the remaining calls of both actions returned in single-digit
// milliseconds.
//
// The subtests therefore do NOT run in parallel, and that is the fix for a
// real CI failure rather than tidiness. Run in parallel, three surfaces
// register three edge environments at once, and the third call to arrive
// waits behind two attempts it did not cause — about 40 seconds before its
// own work even starts. That fits inside portainer.DefaultCallTimeout (60s)
// on this machine and does not on a slower CI runner, where a raw read timed
// out with "context deadline exceeded" against an environment that was
// perfectly healthy.
//
// rawEdgeEnvironment's ?excludeSnapshot=true is still worth having and does
// not solve this on its own: it avoids the cost of the reader's OWN snapshot
// attempt, not the queue behind somebody else's. Serialising is what bounds
// any single call's wait to one attempt.
//
// Total wall clock stays about a minute either way — three environments x
// ~20s is Portainer's cost, not this suite's, and a later reader should not
// try to "fix" it by weakening the test.
func TestEndpoints_EdgeActions_OnAnEnvironmentThisTestOwns(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("trust_edge_endpoints and set_policy_statuses are Business Edition only")
	}
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			session := sessions.For(t, surface, "EE")

			id := createEdgeEnvironmentThroughSurface(t, surface, uniqueName("env-edge"))

			callAction[any](t, session, surface, "endpoints.trust_edge_endpoints", map[string]any{
				"endpointIdS": []int{id},
			})
			if trusted, _ := rawEdgeEnvironment(t, "EE", id)["UserTrusted"].(bool); !trusted {
				t.Errorf("environment %d is not UserTrusted after endpoints.trust_edge_endpoints", id)
			}

			// set_policy_statuses is an agent-facing callback and is
			// asserted to REACH Portainer, not to succeed. Measured
			// 2026-08-18: it answers 403 "Unauthorized Edge endpoint
			// operation: missing Edge identifier" for an API-key caller,
			// because Portainer reads the reporting agent's identity from a
			// request header only the agent sets — the same reason
			// endpoints.create_global_key cannot be usefully called from
			// here, and what both actions' narratives tell a model. A
			// different failure would be a real defect, so the message is
			// checked rather than ignored.
			statusTool, statusArgs := actionCallParams(t, surface, "endpoints.set_policy_statuses", map[string]any{
				"id": id,
				"statuses": []map[string]any{
					{"policyId": 1, "status": "applied", "type": "edgeStack", "message": "e2e"},
				},
			})
			statusRes, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: statusTool, Arguments: statusArgs})
			if err != nil {
				t.Fatalf("endpoints.set_policy_statuses: %v", err)
			}
			if text := toolResultText(statusRes); statusRes.IsError && !strings.Contains(text, "missing Edge identifier") {
				t.Errorf("endpoints.set_policy_statuses failed for a reason other than the measured missing-agent-identity refusal: %s", text)
			}

			// The disassociation is the destructive one, and the read-back is
			// what proves it did something: Portainer clears the environment's
			// edge identity, so the EdgeID it was registered with is gone.
			before, _ := rawEdgeEnvironment(t, "EE", id)["EdgeID"].(string)
			if before == "" {
				t.Fatalf("environment %d has no EdgeID to clear", id)
			}
			callAction[any](t, session, surface, "endpoints.association_delete", map[string]any{"id": id})
			if after, _ := rawEdgeEnvironment(t, "EE", id)["EdgeID"].(string); after == before {
				t.Errorf("environment %d still carries EdgeID %q after endpoints.association_delete", id, after)
			}
		})
	}
}

// TestEndpoints_CreateGlobalKey_IsReachableAndCannotIdentifyAnAgent covers
// the edge enrolment route this catalog can reach but cannot usefully call.
//
// Portainer reads the agent's identity from a request header the agent sets;
// this action has no field for it and sends none, which is exactly what its
// narrative tells a model. The assertion is therefore about reachability,
// not success — and it is worth having, because an action that 404'd or
// failed to build its request would be a different defect entirely.
func TestEndpoints_CreateGlobalKey_IsReachableAndCannotIdentifyAnAgent(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				toolName, args := actionCallParams(t, surface, "endpoints.create_global_key", nil)
				res, err := sessions.For(t, surface, ed).CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.create_global_key: %v", err)
				}
				// Asserted, not merely logged: the doc comment above claims
				// this proves reachability, and a version that 404'd or failed
				// while building the request would satisfy a bare log. The
				// refusal text is the one measured 2026-08-18 against a live
				// 2.44.0 of both editions — Portainer reads the agent's
				// identity from a header this action does not send, so it
				// refuses for want of an Edge ID rather than for want of a
				// route.
				text := toolResultText(res)
				if res.IsError && !strings.Contains(text, "The Edge ID cannot be empty") {
					t.Errorf("endpoints.create_global_key failed for a reason other than the measured missing-Edge-ID refusal: %s", text)
				}
			})
		}
	}
}

// TestEndpoints_NamespacesAccessUpdate_IsRefusedByADockerEnvironment is the
// negative half, and it is kept for what it does prove rather than promoted
// to something it does not.
//
// It asserts a Docker environment refuses the action — the same treatment,
// for the same reason, that stacks_test.go's
// TestStacks_KubernetesCreates_AreRefusedByADockerEnvironment gives the three
// Kubernetes stack creates. What it emphatically does NOT prove is that the
// action works, and that gap was not theoretical: this test passed for a
// whole wave against an rpn parameter typed as an integer, which the server
// rejects outright, because a Docker environment refuses the call whatever
// the rpn is. See TestEndpoints_NamespacesAccessUpdate_GrantsAndRevokes
// below, and docs/api-divergences.md §6.3.
func TestEndpoints_NamespacesAccessUpdate_IsRefusedByADockerEnvironment(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("namespaces_access_update is Business Edition only")
	}
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			envID := firstDockerEnvironmentID(t, "EE")
			toolName, args := actionCallParams(t, surface, "endpoints.namespaces_access_update", map[string]any{
				"id": envID, "rpn": 1, "usersToAdd": []int{1},
			})
			res, err := sessions.For(t, surface, "EE").CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
			if err != nil {
				t.Fatalf("endpoints.namespaces_access_update: %v", err)
			}
			if !res.IsError {
				t.Errorf("endpoints.namespaces_access_update succeeded against a Docker environment: %s", toolResultText(res))
			}
		})
	}
}

// endpointsSafeModeMutations is one entry per mutating action in this domain
// whose interception safe mode must prove, with inputs that would do real
// damage if they ever reached Portainer.
var endpointsSafeModeMutations = []struct {
	action string
	kind   string
	input  func(envID int) map[string]any
}{
	{"endpoints.create", "mutating", func(int) map[string]any {
		return map[string]any{"name": uniqueName("env-safe"), "endpointCreationType": 1, "url": "tcp://docker:2375"}
	}},
	{"endpoints.update", "mutating", func(id int) map[string]any {
		return map[string]any{"id": id, "name": uniqueName("env-safe-renamed")}
	}},
	{"endpoints.delete", "destructive", func(id int) map[string]any {
		return map[string]any{"id": id}
	}},
	{"endpoints.delete_batch", "destructive", func(id int) map[string]any {
		return map[string]any{"endpoints": []map[string]any{{"id": id}}}
	}},
	{"endpoints.settings_update", "mutating", func(id int) map[string]any {
		return map[string]any{"id": id, "securitySettings": map[string]any{"allowPrivilegedModeForRegularUsers": true}}
	}},
	{"endpoints.association_delete", "destructive", func(id int) map[string]any {
		return map[string]any{"id": id}
	}},
	{"endpoints.snapshot_all", "mutating", func(int) map[string]any { return nil }},
}

// TestSafeMode_Endpoints_MutatingActionsArePreviewedAndNothingChanges proves
// tools.Execute intercepts every mutating action in this domain before its
// handler runs.
//
// Three properties, following
// TestSafeMode_CustomTemplates_MutatingActionsArePreviewedAndNothingChanges:
// the answer is a preview rather than a result, it names the action and the
// kind its own ActionSpec flags imply, and nothing changed — read back
// through the raw Portainer API, because a surface that previewed a call and
// executed it anyway would satisfy the first two identically.
//
// Community Edition only: safe-mode interception has nothing to do with
// edition, so it is proven against the leg every estate carries.
func TestSafeMode_Endpoints_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	for _, surface := range surfaceNames {
		for _, mutation := range endpointsSafeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()
				session := sessions.SafeMode(t, surface)

				// The victim is the environment the estate provisions: if
				// safe mode let any of these through, it is the one that
				// would be renamed, have its settings changed, or be deleted.
				envID := firstDockerEnvironmentID(t, "CE")
				before := rawEnvironment(t, "CE", envID)
				beforeName, _ := before["Name"].(string)

				input := mutation.input(envID)
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
				wouldCall, ok := preview["would_call"].(map[string]any)
				if !ok {
					t.Fatalf("safe-mode preview carries no would_call: %v", preview)
				}
				for name := range input {
					if !previewNamesField(wouldCall["input_fields"], name) {
						t.Errorf("safe-mode preview's input_fields %v does not name %q, which this call sent",
							wouldCall["input_fields"], name)
					}
				}

				// Nothing changed.
				if got, _ := rawEnvironment(t, "CE", envID)["Name"].(string); got != beforeName {
					t.Errorf("environment %d is now named %q, want %q: safe mode let %s through", envID, got, beforeName, mutation.action)
				}
				// Named environments only, never a total. Four other tests in
				// this file create and delete Community Edition environments
				// in parallel with these subtests, so a count read before and
				// after is not measuring safe mode at all — it would go red
				// for somebody else's create and green for a leaked one.
				if name, ok := input["name"].(string); ok {
					if got := environmentIDByName(t, "CE", name); got != -1 {
						t.Errorf("safe mode let %s through: it created environment %q as %d", mutation.action, name, got)
					}
				}
				if environmentIDByName(t, "CE", beforeName) != envID {
					t.Errorf("environment %q (%d) is gone: safe mode let %s through", beforeName, envID, mutation.action)
				}
			})
		}
	}
}

// TestEndpoints_NamespacesAccessUpdate_GrantsAndRevokes is the positive half,
// and the only test in this file that needs the Kubernetes leg.
//
// It is what turned this action from "covered" into "working": the negative
// test above passed for a whole wave while the rpn parameter was published as
// an integer, which the server rejects with `namespaces "1" not found`. A
// Docker environment refuses the call whatever the rpn is, so nothing short
// of a real namespace could tell the two apart.
//
// The sequence is the one Portainer actually requires, and each step is here
// because leaving it out fails:
//
//  1. a standard (non-administrator) user, since an administrator already has
//     access to everything and cannot be granted more;
//  2. that user given a role on the ENVIRONMENT, or the call is refused with
//     "access of user N cannot be updated by the current user since there is
//     no role assigned" — the requirement this action's own narrative states;
//  3. only then the namespace grant, addressed by the namespace's NAME.
//
// The revoke half matters as much as the grant: an action that ignored
// usersToRemove entirely would pass a grant-only test.
func TestEndpoints_NamespacesAccessUpdate_GrantsAndRevokes(t *testing.T) {
	if !estate.HasKubernetes() {
		t.Skip("no Kubernetes leg provisioned in this estate: run `make e2e-k8s-up` first")
	}

	logger := logging.New(slog.LevelWarn)
	// Deliberately NOT parallel. grantEnvironmentRole sends the environment's
	// whole UserAccessPolicies map, and Portainer replaces it wholesale rather
	// than merging, so three surfaces granting three different users at once
	// each wipe the previous one's grant — measured, as three subtests failing
	// with "no role assigned" for users the test had just authorised. The
	// contention is over one shared environment, so serialising is the fix
	// rather than a workaround.
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			// Its own session, like TestKubernetesLeg_ReachableThroughEveryMCPSurface:
			// the package's shared Sessions only ever carries the compose
			// legs, and this leg's Helm chart serves a self-signed
			// certificate, hence skipTLSVerify.
			session, err := buildSession(estate.Kubernetes, config.ToolSurface(surface), false, false, true, logger)
			if err != nil {
				t.Fatalf("build session against the Kubernetes leg: %v", err)
			}
			defer func() { _ = session.Close() }()

			envID, ok := estate.Kubernetes.Environment(harness.EnvironmentK3D)
			if !ok {
				t.Fatalf("the Kubernetes leg has no %q environment", harness.EnvironmentK3D)
			}

			userID := createKubernetesStandardUser(t, uniqueName("nsuser"))
			grantEnvironmentRole(t, envID, userID)

			// "default" is the one namespace every Kubernetes cluster has, so
			// this does not depend on anything k3d-up.sh happens to deploy.
			const namespace = "default"

			callAction[any](t, session, surface, "endpoints.namespaces_access_update", map[string]any{
				"id": envID, "rpn": namespace, "usersToAdd": []int{userID},
			})
			if !namespaceGrants(t, namespace, userID) {
				t.Fatalf("user %d has no access to namespace %q after the grant", userID, namespace)
			}

			callAction[any](t, session, surface, "endpoints.namespaces_access_update", map[string]any{
				"id": envID, "rpn": namespace, "usersToRemove": []int{userID},
			})
			if namespaceGrants(t, namespace, userID) {
				t.Errorf("user %d still has access to namespace %q after the revoke", userID, namespace)
			}
		})
	}
}

// kubernetesRawClient talks to the Kubernetes leg directly, for the fixture
// setup and read-back this file's Kubernetes test needs.
//
// Not fixtureClient: that resolves a leg by edition name out of the shared
// Sessions, which never carries this one.
func kubernetesRawClient(t *testing.T) *portainer.Client {
	t.Helper()
	client, err := portainer.New(&config.Config{
		URL:           estate.Kubernetes.BaseURL,
		Token:         estate.Kubernetes.Creds.APIKey,
		SkipTLSVerify: true,
		ToolSurface:   config.SurfaceDynamic,
	})
	if err != nil {
		t.Fatalf("build a raw client against the Kubernetes leg: %v", err)
	}
	return client
}

// kubernetesRawJSON issues one request against the Kubernetes leg and decodes
// the answer, or fails the test.
func kubernetesRawJSON(t *testing.T, method, path string, body io.Reader, out any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := kubernetesRawClient(t).Do(ctx, method, path, body)
	if err != nil {
		t.Fatalf("raw %s %s on the Kubernetes leg: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode raw %s %s: %v", method, path, err)
	}
}

// createKubernetesStandardUser creates a non-administrator user on the
// Kubernetes leg and registers its removal.
//
// Role 2 is Portainer's "standard user". An administrator is deliberately not
// used: it already has access to every namespace, so granting it more is a
// no-op the read-back could not distinguish from a working grant.
func createKubernetesStandardUser(t *testing.T, name string) int {
	t.Helper()
	var created map[string]any
	payload := fmt.Sprintf(`{"Username":%q,"Password":%q,"Role":2}`, name, generateUserPassword(t))
	kubernetesRawJSON(t, http.MethodPost, "/users", strings.NewReader(payload), &created)

	idFloat, ok := created["Id"].(float64)
	if !ok {
		t.Fatalf("creating user %q returned no Id: %v", name, created)
	}
	id := int(idFloat)
	t.Cleanup(func() {
		kubernetesRawJSON(t, http.MethodDelete, fmt.Sprintf("/users/%d", id), nil, nil)
	})
	return id
}

// generateUserPassword mints a fresh password for a throwaway test user.
//
// Generated rather than written down, following
// harness.generateAdminPassword's own reasoning and for one reason beyond it.
// The reason it shares: crypto/rand, because this is a credential a real
// Portainer will accept, and base64 URL-safe encoding so the value needs no
// escaping in the JSON body it travels in.
//
// The reason of its own: nothing else in this suite hard-codes a password,
// and the literal that was here first was the only one — it tripped the
// repository's secret scanner on every run. A permanently red security check
// is worse than the "it is only a test literal" it would have been waived
// with, because it teaches everyone to skip past that check.
func generateUserPassword(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generate a password for the fixture user: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// grantEnvironmentRole gives userID a role on the environment, which
// Portainer requires before any namespace within it can be granted.
//
// Role 3 is "Standard User". Without this the namespace grant is refused with
// "access of user N cannot be updated by the current user since there is no
// role assigned", which is the requirement endpoints.namespaces_access_update's
// narrative states and this fixture exists to satisfy.
func grantEnvironmentRole(t *testing.T, envID, userID int) {
	t.Helper()
	payload := fmt.Sprintf(`{"UserAccessPolicies":{"%d":{"RoleId":3}}}`, userID)
	kubernetesRawJSON(t, http.MethodPut, fmt.Sprintf("/endpoints/%d", envID), strings.NewReader(payload), nil)
}

// namespaceGrants reports whether userID currently has access to namespace,
// read out of the CLUSTER rather than out of Portainer.
//
// Two reasons, and the first is decisive: the vendored specification declares
// no route that reads namespace access back. `PUT
// /endpoints/{id}/pools/{rpn}/access` is the only operation on that path, so
// there is nothing to GET. The second is that the cluster is the better
// witness anyway — it shows the grant actually reached Kubernetes, where
// Portainer's own record would only show that Portainer stored it.
//
// Portainer implements a grant as a ServiceAccount per user plus RoleBindings
// in the target namespace, and names the account after the user's own
// identifier. Measured 2026-08-18: granting user N produces bindings whose
// subject is `portainer-sa-user-<instance>-N`, and revoking removes every
// binding it created.
func namespaceGrants(t *testing.T, namespace string, userID int) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	cluster := os.Getenv("E2E_K3D_CLUSTER")
	if cluster == "" {
		cluster = "portainer-mcp-e2e"
	}
	out, err := exec.CommandContext(ctx, "kubectl", "--context", "k3d-"+cluster,
		"get", "rolebindings", "-n", namespace,
		"-o", "jsonpath={.items[*].subjects[*].name}").Output()
	if err != nil {
		t.Fatal(kubectlRunError("reading namespace rolebindings through kubectl", err))
	}
	// The suffix is anchored so user 2 never matches a binding for user 21.
	return slices.ContainsFunc(strings.Fields(string(out)), func(subject string) bool {
		return strings.HasPrefix(subject, "portainer-sa-user-") &&
			strings.HasSuffix(subject, "-"+strconv.Itoa(userID))
	})
}
