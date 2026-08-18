//go:build e2e

package suite

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// This file covers all twenty-five stacks actions against the live estate,
// on all three surfaces and on every edition each action applies to.
// `make audit-e2e-gaps` reported every one of the twenty-five as
// unreferenced before it existed: 25 of 61, with every other domain at
// 36/36, and six of the twenty-five destructive.
//
// What a stack action does is only observable where the stack is deployed,
// which is why almost nothing here asserts on a response alone. A create is
// checked against a container or a Swarm service really existing inside the
// estate's own dind; a stop against that container being gone; a delete
// against both the container and the stack record; a migrate against which
// environment the stack ends up in; a git redeploy against the stack file
// changing to a commit pushed between the two calls. Every one of those has
// a plausible failure that answers 200, and several are recorded in
// docs/api-divergences.md as behaviours this server really has.
//
// Safe mode is not a substitute for any of it: it intercepts before the
// handler runs (internal/tools/register.go's Execute), so a safe-mode-only
// test proves the interception and nothing about the action.

// stackFileFor builds a Compose file carrying marker, so a content
// assertion identifies one specific stack rather than passing against
// whatever another test in the matrix left on the same server. The service
// sleeps rather than echoing, because a container that has already exited is
// not evidence that stacks.start started anything.
func stackFileFor(marker string) string {
	return fmt.Sprintf("services:\n  hello:\n    image: %s\n    command: [\"sleep\", \"3600\"]\n    environment:\n      MARKER: %q\n", dockerFixtureImage, marker)
}

// stackDeployTimeout bounds waitStackSettled. Measured against this estate:
// a Compose stack of one busybox service leaves StackStatusDeploying within
// about two seconds, so this is generous by more than an order of magnitude
// and only ever pays out when something is genuinely wrong.
const stackDeployTimeout = 90 * time.Second

// Portainer's own portainer.StackStatus enum, as the vendored document
// declares it (x-enum-varnames on portainer.StackStatus): 1 deployed and
// running, 2 intentionally stopped, 3 deployment in progress, 4 deployment
// failed. Named here because three of them are asserted on directly and a
// bare 1/2/3 in an assertion says nothing to a reader.
const (
	stackStatusActive    = 1
	stackStatusInactive  = 2
	stackStatusDeploying = 3
)

// createStackFixture deploys a Compose stack on edition ed's server through
// the raw Portainer API — never through an MCP surface — and returns its id,
// with cleanup registered.
//
// Raw on purpose, exactly as createCustomTemplateFixture is: this exists for
// the tests whose subject is something other than creation (safe mode's
// update/stop/delete previews need a stack that already exists), and
// building a fixture through the very action under test would make a create
// regression look like a failure of whatever the test was really about.
func createStackFixture(t *testing.T, ed string, envID int, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create stack %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		body := apigen.StackCreateDockerStandaloneStringJSONRequestBody{
			Name:             name,
			StackFileContent: stackFileFor(name),
		}
		resp, err := client.API.StackCreateDockerStandaloneStringWithResponse(ctx,
			&apigen.StackCreateDockerStandaloneStringParams{EndpointId: envID}, body)
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			return fmt.Errorf("create stack %q: response carried no id", name)
		}
		id = *resp.JSON200.Id
		return nil
	})

	registerStackCleanup(t, ed, envID, id)
	waitStackSettled(t, ed, id)
	return id
}

// registerStackCleanup registers id's deletion with the calling test's
// ledger. It is separate from createStackFixture because most stacks in this
// file are created through the actions under test, not through the fixture
// helper, and every one of those still has to be cleaned up.
func registerStackCleanup(t *testing.T, ed string, envID, id int) {
	t.Helper()
	newLedger(t).Register("stack", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteStackIfPresent(ctx, ed, envID, id)
	})
}

// swarmTeardownRetries and swarmTeardownBackoff bound how long
// deleteStackIfPresent waits out the one failure Docker Swarm itself
// produces when a stack is removed shortly after it was deployed. See that
// function's own doc.
const (
	swarmTeardownRetries = 6
	swarmTeardownBackoff = 2 * time.Second
)

// swarmNetworkInUse is the substring identifying that failure. It is
// Docker's own wording, quoted back by Portainer inside a 500.
const swarmNetworkInUse = "is in use by task"

// deleteStackIfPresent deletes stack id on edition ed's server, treating
// "already gone" as success: stacks.delete is itself one of the actions
// under test here, so by the time a test ends the stack it created may
// already have been deleted by the test body. See deleteTagIfPresent and
// deleteCustomTemplateIfPresent, which exist for the identical reason.
//
// envID is required by the route rather than optional: DELETE /stacks/{id}
// declares endpointId a required query parameter, and a stack whose
// environment has since been removed (which one test in this file
// deliberately produces) answers 404 here, which this treats as success for
// the same reason an already-deleted stack does.
//
// The retry is for one specific, measured failure, and it is Docker Swarm's
// rather than Portainer's or this catalog's: removing a Swarm stack tears
// down its services and then its overlay network, and while a stopping task
// is still attached the daemon refuses with
// 500 "failed to remove network ..._default: ... network ... is in use by
// task ...". Measured against this estate, a delete issued a few seconds
// after the deploy hits it and the same delete succeeds a couple of seconds
// later. It is retried rather than ignored because the network really does
// have to go: leaving it behind would strand an overlay network on the
// estate's daemon for every swarm stack the suite creates.
func deleteStackIfPresent(ctx context.Context, ed string, envID, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < swarmTeardownRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("delete stack %d: %w", id, ctx.Err())
			case <-time.After(swarmTeardownBackoff):
			}
		}
		resp, err := client.API.StackDeleteWithResponse(ctx, id, &apigen.StackDeleteParams{EndpointId: envID})
		if err != nil {
			return fmt.Errorf("delete stack %d: %w", id, err)
		}
		if resp.StatusCode() == http.StatusNotFound {
			return nil
		}
		checkErr := toolutil.Check(resp)
		if checkErr == nil {
			return nil
		}
		lastErr = fmt.Errorf("delete stack %d: %w", id, checkErr)
		if !strings.Contains(checkErr.Error(), swarmNetworkInUse) {
			return lastErr
		}
	}
	return lastErr
}

// rawStack reads stack id straight from edition ed's Portainer API,
// bypassing every MCP surface. It is what makes an assertion about our own
// redaction possible: the server's own answer is the only thing that can
// show a field is present upstream and absent downstream.
func rawStack(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.Do(ctx, http.MethodGet, fmt.Sprintf("/stacks/%d", id), nil)
	if err != nil {
		t.Fatalf("raw GET /stacks/%d: %v", id, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw GET /stacks/%d: status %d, body %s", id, resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding raw GET /stacks/%d body %q: %v", id, body, err)
	}
	return out
}

// rawStackFile reads stack id's stored stack file straight from edition ed's
// Portainer API. Used where a test has to know what the server really holds,
// independently of the action whose behaviour is in question.
func rawStackFile(t *testing.T, ed string, id int) string {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.Do(ctx, http.MethodGet, fmt.Sprintf("/stacks/%d/file", id), nil)
	if err != nil {
		t.Fatalf("raw GET /stacks/%d/file: %v", id, err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw GET /stacks/%d/file: status %d, body %s", id, resp.StatusCode, body)
	}
	var out struct {
		StackFileContent string
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decoding raw GET /stacks/%d/file body %q: %v", id, body, err)
	}
	return out.StackFileContent
}

// waitStackSettled blocks until stack id is no longer reported as
// StackStatusDeploying.
//
// It exists because Portainer refuses concurrent work on a stack that is
// still deploying rather than queueing it: measured against this estate, a
// stacks.webhook_invoke issued immediately after a create answers
// 409 "Unable to update stack / Stack deployment is already in progress",
// and the same stack answers 204 a couple of seconds later. Polling the
// stack's own status is what makes the tests that chain two writes together
// deterministic instead of racing the deployment they just started.
func waitStackSettled(t *testing.T, ed string, id int) {
	t.Helper()
	deadline := time.Now().Add(stackDeployTimeout)
	for {
		status, _ := rawStack(t, ed, id)["Status"].(float64)
		if int(status) != stackStatusDeploying {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stack %d on the %s server is still reported as deploying after %s", id, ed, stackDeployTimeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// waitStackFileEquals blocks until stack id's stored file is want, and fails
// the test naming what it saw instead if it never becomes that.
//
// Polling rather than a single read, because the two webhook routes are
// ASYNCHRONOUS: measured, POST /stacks/webhooks/{uuid} answers 204
// immediately and Portainer pulls and redeploys afterwards, so a read taken
// straight after the call sees the previous revision. A fixed sleep would
// either be too short on a loaded estate or waste time on an idle one; this
// waits exactly as long as it has to.
//
// It stays discriminating: a webhook that fetched nothing never reaches want
// and this fails on the timeout, reporting the stale content — the same
// failure, and the same message, as a webhook that fetched the wrong thing.
func waitStackFileEquals(t *testing.T, ed string, id int, want, action string) {
	t.Helper()
	deadline := time.Now().Add(stackDeployTimeout)
	var got string
	for {
		got = rawStackFile(t, ed, id)
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("%s did not replace stack %d's stored file within %s; it is still\n%q\nwant the commit just pushed\n%q",
				action, id, stackDeployTimeout, got, want)
			return
		}
		time.Sleep(time.Second)
	}
}

// waitEdgeStackFileEquals is waitStackFileEquals for an edge stack, which is
// read through a different route and belongs to no domain this catalog
// declares yet. POST /edge_stacks/webhooks/{uuid} is asynchronous in the same
// way its /stacks sibling is.
func waitEdgeStackFileEquals(t *testing.T, id int, want, action string) {
	t.Helper()
	deadline := time.Now().Add(stackDeployTimeout)
	var got string
	for {
		got = rawEdgeStackFile(t, id)
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("%s did not replace edge stack %d's stored file within %s; it is still\n%q\nwant the commit just pushed\n%q",
				action, id, stackDeployTimeout, got, want)
			return
		}
		time.Sleep(time.Second)
	}
}

// stackID pulls the Id out of a stack action result. JSON numbers decode
// into float64 through map[string]any, and a stack id that silently arrived
// as something else (a string, or absent) would otherwise turn into a
// confusing failure several calls later.
func stackID(t *testing.T, out map[string]any, action string) int {
	t.Helper()
	raw, ok := out["Id"].(float64)
	if !ok {
		t.Fatalf("%s returned no usable Id: %v", action, out)
	}
	return int(raw)
}

// stackFileContentOf reads the StackFileContent out of a stacks.file_inspect
// result. The response carries exactly that one field (measured against
// 2.44.0), so its absence is a real shape regression rather than an
// empty-stack artefact.
//
// It names the action in its failure rather than taking it as a parameter:
// stacks.file_inspect is the only action in this domain that answers with
// this shape, and a parameter that every call site passes the same literal
// to is one more place for a copy-paste to name the wrong action.
func stackFileContentOf(t *testing.T, surface string, out map[string]any) string {
	t.Helper()
	content, ok := out["StackFileContent"].(string)
	if !ok {
		t.Fatalf("stacks.file_inspect on the %s surface returned no StackFileContent: %v", surface, out)
	}
	return content
}

// dockerEnvID returns leg's own "docker" environment id, failing rather than
// skipping when it is missing: every compose leg registers one at
// provisioning time (cmd/provision/main.go), so its absence is a broken
// estate rather than an optional capability.
func dockerEnvID(t *testing.T, leg harness.Leg) int {
	t.Helper()
	envID, ok := leg.Server.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("%s: estate has no %q environment", leg.Name, harness.EnvironmentDocker)
	}
	return envID
}

// swarmClusterID returns the real Swarm cluster identifier of the daemon
// behind envID, read from `docker info` through Portainer's own Docker
// proxy.
//
// Read live rather than recorded in the estate or invented: this is exactly
// the identifier docs/api-divergences.md §6.3's "cheat this is written down
// to forbid" warns against hand-labelling. A swarmId a test made up would
// make the string-typed field look correct while proving nothing, and
// Portainer accepts a wrong one — a stack created with it simply never
// matches the SwarmID list filter, which is a silent failure rather than a
// loud one.
func swarmClusterID(t *testing.T, srv harness.Server, envID int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp := dockerProxy(ctx, t, srv, envID, http.MethodGet, "/info", nil)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /info through the docker proxy: status %d, body %s", resp.StatusCode, body)
	}
	var info struct {
		Swarm struct {
			LocalNodeState string
			Cluster        struct {
				ID string
			}
		}
	}
	if err := json.Unmarshal([]byte(body), &info); err != nil {
		t.Fatalf("decoding docker info %q: %v", body, err)
	}
	if info.Swarm.Cluster.ID == "" {
		t.Fatalf("the estate's docker daemon reports Swarm state %q and no cluster id: run `make e2e-up` on a host where `docker swarm init` succeeds",
			info.Swarm.LocalNodeState)
	}
	return info.Swarm.Cluster.ID
}

// stackContainerNames returns the names of every container the Docker
// Compose deployment of stackName currently has inside the estate's dind,
// running or not.
//
// It filters on the com.docker.compose.project label rather than on a
// container name prefix: the project label is what Compose itself keys on,
// and a name-prefix match would also catch a container another test's stack
// happened to name similarly. Measured against this estate, a stack named N
// with one service "hello" produces exactly one container labelled
// com.docker.compose.project=N.
func stackContainerNames(t *testing.T, srv harness.Server, envID int, stackName string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	path := `/containers/json?all=true&filters={"label":["com.docker.compose.project=` + stackName + `"]}`
	resp := dockerProxy(ctx, t, srv, envID, http.MethodGet, path, nil)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listing containers of stack %q: status %d, body %s", stackName, resp.StatusCode, body)
	}
	var containers []struct {
		Names []string
	}
	if err := json.Unmarshal([]byte(body), &containers); err != nil {
		t.Fatalf("decoding the container list %q: %v", body, err)
	}
	var names []string
	for _, c := range containers {
		names = append(names, c.Names...)
	}
	return names
}

// swarmServiceNames returns the names of every Swarm service belonging to
// the stack named stackName, keyed on the com.docker.stack.namespace label
// Docker's own stack deployer writes (measured on this estate: a stack N
// with service "hello" produces one service named "N_hello" carrying
// com.docker.stack.namespace=N).
func swarmServiceNames(t *testing.T, srv harness.Server, envID int, stackName string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	path := `/services?filters={"label":["com.docker.stack.namespace=` + stackName + `"]}`
	resp := dockerProxy(ctx, t, srv, envID, http.MethodGet, path, nil)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("listing swarm services of stack %q: status %d, body %s", stackName, resp.StatusCode, body)
	}
	var services []struct {
		Spec struct {
			Name string
		}
	}
	if err := json.Unmarshal([]byte(body), &services); err != nil {
		t.Fatalf("decoding the service list %q: %v", body, err)
	}
	var names []string
	for _, s := range services {
		names = append(names, s.Spec.Name)
	}
	return names
}

// createSecondDockerEnvironment registers one more Docker environment on
// srv, pointing at the same daemon srv's own "docker" environment already
// points at, and returns its id with removal registered on the calling
// test's ledger.
//
// stacks.migrate and stacks.associate both need a SECOND environment on the
// same server to mean anything, and only the Community Edition leg is
// provisioned with one (its agent environment). Rather than restricting
// those two actions to the leg that happens to have a spare environment,
// this makes one.
//
// The daemon URL is read back from the existing environment rather than
// written out again here: "tcp://docker:2375" is cmd/provision/main.go's own
// dindDaemonURL constant, unexported and in a package main this suite cannot
// import, and a second literal copy of it would be free to drift from the
// estate it is supposed to describe. Reading it from the server means this
// registers whatever the estate actually registered.
func createSecondDockerEnvironment(t *testing.T, ed string, srv harness.Server, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	existing, ok := srv.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the %s server has no %q environment to copy a daemon URL from", ed, harness.EnvironmentDocker)
	}
	inspect, err := client.API.EndpointInspectWithResponse(ctx, existing, &apigen.EndpointInspectParams{})
	if err != nil {
		t.Fatalf("inspect environment %d on %s: %v", existing, ed, err)
	}
	if err := toolutil.Check(inspect); err != nil {
		t.Fatalf("inspect environment %d on %s: %v", existing, ed, err)
	}
	if inspect.JSON200 == nil || inspect.JSON200.URL == "" {
		t.Fatalf("environment %d on %s reports no daemon URL to register a second environment against", existing, ed)
	}

	id, err := harness.CreateEndpoint(ctx, &http.Client{Timeout: portainer.DefaultCallTimeout},
		srv.BaseURL, srv.Creds.APIKey, harness.EndpointSpec{
			Name:         name,
			CreationType: 1,
			URL:          inspect.JSON200.URL,
		})
	if err != nil {
		// A Business Edition licence caps how many nodes may be registered,
		// and this estate's licence is whatever secret the run was given —
		// CI's is not the one a developer holds. Running out of allowance is
		// a property of the licence, not a defect in the action under test,
		// so it skips rather than reddening the build. Any other failure
		// still fails: an estate that cannot register an environment for
		// some other reason is a real problem.
		if strings.Contains(err.Error(), "node allowance") {
			t.Skipf("this %s licence has no node allowance left to register a second environment, which this test needs: %v", ed, err)
		}
		t.Fatalf("register a second docker environment on %s: %v", ed, err)
	}

	newLedger(t).Register("environment", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteEnvironmentIfPresent(ctx, ed, id)
	})
	return id
}

// deleteEnvironmentIfPresent removes environment id from edition ed's
// server, treating "already gone" as success — one test in this file deletes
// the environment itself as the way it produces a genuinely orphaned stack.
func deleteEnvironmentIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.EndpointDeleteWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("delete environment %d: %w", id, err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("delete environment %d: %w", id, err)
	}
	return nil
}

// stackListed reports whether a list response carries a stack with this id.
func stackListed(listed []map[string]any, id int) bool {
	for _, entry := range listed {
		if entryID, ok := entry["Id"].(float64); ok && int(entryID) == id {
			return true
		}
	}
	return false
}

// TestStacks_ComposeStackLifecycle_CreatesReadsUpdatesStopsStartsAndDeletes
// walks one Compose stack through eight of the twenty-five actions in the
// order a caller would: create_docker_standalone_string, list, inspect,
// file_inspect, update, stop, start, delete.
//
// Every step that changes something is read back against the estate's own
// Docker daemon, not merely against the response. A create that answers 200
// and deploys nothing, an update that answers 200 and stores nothing, and a
// stop that answers 200 and leaves the containers running are all failures
// this domain's own responses cannot distinguish from success — a stack
// object comes back either way — and docs/api-divergences.md §2.1 records
// exactly that shape of lie on another route of this same server.
//
// The delete at the end is one of the three assertions this task required to
// be proven discriminating (see the task report): it asserts BOTH that the
// stack record is gone (stacks.inspect fails) AND that the container it
// deployed is gone from the dind. Either alone would pass against a delete
// that removed the record and orphaned the containers, or one that stopped
// the containers and kept the record.
func TestStacks_ComposeStackLifecycle_CreatesReadsUpdatesStopsStartsAndDeletes(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("stack")
				content := stackFileFor(name)
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_string", map[string]any{
					"endpointId":       envID,
					"name":             name,
					"stackFileContent": content,
				})
				id := stackID(t, created, "stacks.create_docker_standalone_string")
				registerStackCleanup(t, leg.Name, envID, id)
				if got, _ := created["Name"].(string); got != name {
					t.Errorf("stacks.create_docker_standalone_string returned Name %q, want %q", got, name)
				}
				waitStackSettled(t, leg.Name, id)

				// The create's discriminating half: a container really
				// exists inside the estate's own daemon.
				if names := stackContainerNames(t, leg.Server, envID, name); len(names) == 0 {
					t.Fatalf("stacks.create_docker_standalone_string answered 200 but deployed no container for stack %q", name)
				}

				listed := callAction[[]map[string]any](t, session, surface, "stacks.list", map[string]any{})
				if !stackListed(listed, id) {
					t.Errorf("stacks.list does not carry the stack %d (%q) just created", id, name)
				}

				inspected := callAction[map[string]any](t, session, surface, "stacks.inspect", map[string]any{"id": id})
				if got, _ := inspected["Name"].(string); got != name {
					t.Errorf("stacks.inspect returned Name %q, want %q", got, name)
				}
				if got, _ := inspected["EndpointId"].(float64); int(got) != envID {
					t.Errorf("stacks.inspect returned EndpointId %v, want %d", inspected["EndpointId"], envID)
				}

				file := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, file); got != content {
					t.Errorf("stacks.file_inspect returned\n%q\nwant the content create was given:\n%q", got, content)
				}

				// The update rewrites both the stack file and the variable
				// list, and both are read back below: the response alone
				// would be satisfied by a server that echoed the request
				// without storing it.
				newContent := stackFileFor(name + "-updated")
				updated := callAction[map[string]any](t, session, surface, "stacks.update", map[string]any{
					"id":               id,
					"endpointId":       envID,
					"stackFileContent": newContent,
					"env":              []map[string]any{{"name": "E2E_MARKER", "value": name}},
				})
				if got, _ := updated["Id"].(float64); int(got) != id {
					t.Errorf("stacks.update returned Id %v, want %d", updated["Id"], id)
				}
				waitStackSettled(t, leg.Name, id)
				afterUpdate := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, afterUpdate); got != newContent {
					t.Errorf("after stacks.update the stored file is\n%q\nwant\n%q", got, newContent)
				}

				stopped := callAction[map[string]any](t, session, surface, "stacks.stop", map[string]any{
					"id": id, "endpointId": envID,
				})
				if got, _ := stopped["Status"].(float64); int(got) != stackStatusInactive {
					t.Errorf("stacks.stop returned Status %v, want %d (intentionally stopped)", stopped["Status"], stackStatusInactive)
				}
				// The half a status field cannot give: Portainer REMOVES a
				// stopped Compose stack's containers rather than pausing
				// them (measured), so their continued existence is a real
				// regression signal.
				if names := stackContainerNames(t, leg.Server, envID, name); len(names) != 0 {
					t.Errorf("after stacks.stop the stack %q still has containers in the estate's daemon: %v", name, names)
				}

				started := callAction[map[string]any](t, session, surface, "stacks.start", map[string]any{
					"id": id, "endpointId": envID,
				})
				if got, _ := started["Status"].(float64); int(got) != stackStatusActive {
					t.Errorf("stacks.start returned Status %v, want %d (deployed and running)", started["Status"], stackStatusActive)
				}
				waitStackSettled(t, leg.Name, id)
				if names := stackContainerNames(t, leg.Server, envID, name); len(names) == 0 {
					t.Errorf("after stacks.start the stack %q has no container in the estate's daemon", name)
				}

				callAction[map[string]any](t, session, surface, "stacks.delete", map[string]any{
					"id": id, "endpointId": envID,
				})
				// Both halves of the destructive proof, together.
				assertActionFails(t, session, surface, "stacks.inspect", map[string]any{"id": id})
				if names := stackContainerNames(t, leg.Server, envID, name); len(names) != 0 {
					t.Errorf("after stacks.delete the stack %q still has containers in the estate's daemon: %v", name, names)
				}
			})
		}
	}
}

// TestStacks_SwarmStackFromInlineContent_DeploysARealSwarmService covers
// stacks.create_docker_swarm_string, and it is the reason this domain needed
// a Swarm-capable estate at all: the route refuses to deploy without a real
// Swarm cluster identifier, and what it produces is a Swarm SERVICE, not a
// container, so nothing in the Compose lifecycle test above can stand in
// for it.
//
// swarmId comes from `docker info` on the estate's own daemon, never from a
// literal: see swarmClusterID's own doc for why an invented one would look
// correct and prove nothing.
func TestStacks_SwarmStackFromInlineContent_DeploysARealSwarmService(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("no confirmed swarm leg on this estate's docker daemon: run `make e2e-up` on a host where `docker swarm init` succeeds")
	}
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		swarmID := swarmClusterID(t, leg.Server, envID)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("swarmstack")
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_swarm_string", map[string]any{
					"endpointId":       envID,
					"name":             name,
					"swarmId":          swarmID,
					"stackFileContent": stackFileFor(name),
				})
				id := stackID(t, created, "stacks.create_docker_swarm_string")
				registerStackCleanup(t, leg.Name, envID, id)
				if got, _ := created["SwarmId"].(string); got != swarmID {
					t.Errorf("stacks.create_docker_swarm_string stored SwarmId %q, want the daemon's own %q", got, swarmID)
				}
				waitStackSettled(t, leg.Name, id)

				// The assertion a 200 cannot give: a real Swarm service.
				services := swarmServiceNames(t, leg.Server, envID, name)
				if len(services) == 0 {
					t.Fatalf("stacks.create_docker_swarm_string answered 200 but the daemon has no swarm service for stack %q", name)
				}

				// stacks.delete is deliberately NOT called through the
				// surface here, and the reason is Docker Swarm's rather
				// than this domain's: removing a Swarm stack seconds after
				// deploying it races the daemon's own teardown and answers
				// 500 "network ... is in use by task ..." (see
				// deleteStackIfPresent). The action's live delete proof
				// belongs to the Compose lifecycle test above, which asserts
				// both halves of it without a race; here the ledger's
				// retrying cleanup removes the stack, and what this test is
				// for is the Swarm deployment itself.
			})
		}
	}
}

// TestStacks_CreateFromUploadedFile_StoresTheUploadedStackFile covers the two
// actions in this domain with hand-written multipart handlers
// (internal/tools/stacks/handlers.go): their request bodies are
// multipart/form-data, which oapi-codegen could not type, so the multipart
// writing is this repository's own code and nothing but a live call proves
// Portainer accepts what it produces.
//
// The part names are the vendored schema's own capitalisation ("Name",
// "SwarmID", "Env", but lowercase "file"), matched literally by Go's
// multipart reader rather than case-folded like a header — so a
// transcription slip there reaches the server as a missing field, which is
// exactly what this test would catch. Reading the stored file back is what
// proves the file part in particular arrived intact rather than empty.
func TestStacks_CreateFromUploadedFile_StoresTheUploadedStackFile(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("no confirmed swarm leg on this estate's docker daemon: stacks.create_docker_swarm_file needs one")
	}
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		swarmID := swarmClusterID(t, leg.Server, envID)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface+"/standalone", func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("stackfile")
				content := stackFileFor(name)
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_file", map[string]any{
					"endpointId": envID,
					"name":       name,
					"file":       content,
					"env":        `[{"name":"E2E_UPLOAD","value":"standalone"}]`,
				})
				id := stackID(t, created, "stacks.create_docker_standalone_file")
				registerStackCleanup(t, leg.Name, envID, id)
				if got, _ := created["Name"].(string); got != name {
					t.Errorf("stacks.create_docker_standalone_file returned Name %q, want %q", got, name)
				}
				waitStackSettled(t, leg.Name, id)

				file := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, file); got != content {
					t.Errorf("stacks.file_inspect returned\n%q\nwant the uploaded content\n%q", got, content)
				}
				if names := stackContainerNames(t, leg.Server, envID, name); len(names) == 0 {
					t.Errorf("the uploaded stack %q deployed no container", name)
				}
			})

			t.Run(leg.Name+"/"+surface+"/swarm", func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("swarmfile")
				content := stackFileFor(name)
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_swarm_file", map[string]any{
					"endpointId": envID,
					"name":       name,
					"swarmId":    swarmID,
					"file":       content,
					"env":        `[{"name":"E2E_UPLOAD","value":"swarm"}]`,
				})
				id := stackID(t, created, "stacks.create_docker_swarm_file")
				registerStackCleanup(t, leg.Name, envID, id)
				// The SwarmID part specifically: it is written as a string
				// part by hand, and a stack that came back with an empty
				// SwarmId would be one the multipart body failed to carry it
				// into — which the list filter test below would then also
				// fail on, for a reason that has nothing to do with lists.
				if got, _ := created["SwarmId"].(string); got != swarmID {
					t.Errorf("stacks.create_docker_swarm_file stored SwarmId %q, want the daemon's own %q", got, swarmID)
				}
				waitStackSettled(t, leg.Name, id)

				file := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, file); got != content {
					t.Errorf("stacks.file_inspect returned\n%q\nwant the uploaded content\n%q", got, content)
				}
				if services := swarmServiceNames(t, leg.Server, envID, name); len(services) == 0 {
					t.Errorf("the uploaded swarm stack %q deployed no swarm service", name)
				}
			})
		}
	}
}

// TestStacks_CreateFromGitRepository_ClonesTheEstatesGitRepository covers the
// two git-backed create actions against the estate's own git server
// (docker-compose.yml's `git` service), which serves smart HTTP because that
// is the only transport BOTH editions clone (docs/api-divergences.md §3.8).
//
// The discriminating assertion is the stack file: it must be
// harness.GitFixtureStackFile byte for byte, content only the fixture
// repository holds. A create that answered 200 without cloning anything, or
// that cloned some other repository, fails it — where an assertion on the
// response's own Id or Name would not.
//
// It clones the read-only repository, never the mutable one: these subtests
// run in parallel across six (leg, surface) pairs, and the mutable
// repository is written to by the redeploy and webhook tests below.
func TestStacks_CreateFromGitRepository_ClonesTheEstatesGitRepository(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("no confirmed swarm leg on this estate's docker daemon: stacks.create_docker_swarm_repository needs one")
	}
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		swarmID := swarmClusterID(t, leg.Server, envID)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface+"/standalone", func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("stackgit")
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_repository", map[string]any{
					"endpointId":    envID,
					"name":          name,
					"repositoryUrl": harness.GitFixtureRepositoryURL,
					"composeFile":   harness.GitFixtureConfigFilePath,
				})
				id := stackID(t, created, "stacks.create_docker_standalone_repository")
				registerStackCleanup(t, leg.Name, envID, id)
				assertClonedFromTheFixture(t, created, "stacks.create_docker_standalone_repository")
				waitStackSettled(t, leg.Name, id)

				file := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, file); got != harness.GitFixtureStackFile {
					t.Errorf("the cloned stack's file is\n%q\nwant the fixture repository's own content\n%q", got, harness.GitFixtureStackFile)
				}
			})

			t.Run(leg.Name+"/"+surface+"/swarm", func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("swarmgit")
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_swarm_repository", map[string]any{
					"endpointId":    envID,
					"name":          name,
					"swarmId":       swarmID,
					"repositoryUrl": harness.GitFixtureRepositoryURL,
					"composeFile":   harness.GitFixtureConfigFilePath,
				})
				id := stackID(t, created, "stacks.create_docker_swarm_repository")
				registerStackCleanup(t, leg.Name, envID, id)
				assertClonedFromTheFixture(t, created, "stacks.create_docker_swarm_repository")
				waitStackSettled(t, leg.Name, id)

				file := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, file); got != harness.GitFixtureStackFile {
					t.Errorf("the cloned swarm stack's file is\n%q\nwant the fixture repository's own content\n%q", got, harness.GitFixtureStackFile)
				}
			})
		}
	}
}

// assertClonedFromTheFixture checks a git-backed create's own GitConfig
// against the repository it was told to clone.
//
// The URL is checked as a prefix rather than for equality because Portainer
// stores it with the ".git" suffix stripped (measured: GitConfig.URL comes
// back as ".../cgi-bin/git/repo"), the same behaviour custom_templates
// already records. It still fails if the stack were cloned from somewhere
// else entirely.
func assertClonedFromTheFixture(t *testing.T, created map[string]any, action string) {
	t.Helper()
	gitConfig, ok := created["GitConfig"].(map[string]any)
	if !ok {
		t.Fatalf("%s returned no GitConfig: %v", action, created)
	}
	if got, _ := gitConfig["ConfigFilePath"].(string); got != harness.GitFixtureConfigFilePath {
		t.Errorf("%s stored GitConfig.ConfigFilePath = %q, want %q", action, got, harness.GitFixtureConfigFilePath)
	}
	if got, _ := gitConfig["URL"].(string); got == "" || !strings.HasPrefix(harness.GitFixtureRepositoryURL, got) {
		t.Errorf("%s stored GitConfig.URL = %q, want a prefix of %q", action, got, harness.GitFixtureRepositoryURL)
	}
}

// TestStacks_CreateFromGitRepository_DropsTheStoredGitUsername is the only
// redaction assertion in this file that can fail, and the reason for the
// shape it has.
//
// Portainer already blanks the git password itself: measured on both
// editions, POST /stacks/create/standalone/repository answers
// Authentication: {"Username":"e2e-git-user","Password":""}. An e2e
// assertion that "the password is absent" would therefore pass with
// internal/redact deleted, and would be worse than no test at all. What only
// this repository's redactor explains is the USERNAME: the raw API returns
// it, and every stacks action must not.
//
// Both halves are asserted against the same stack, in the same test: the raw
// read is what proves the tool-surface absence is redaction rather than a
// server that simply had nothing to return.
func TestStacks_CreateFromGitRepository_DropsTheStoredGitUsername(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("stackauth")
				const username = "e2e-git-user"
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_repository", map[string]any{
					"endpointId":               envID,
					"name":                     name,
					"repositoryUrl":            harness.GitFixtureRepositoryURL,
					"composeFile":              harness.GitFixtureConfigFilePath,
					"repositoryAuthentication": true,
					"repositoryUsername":       username,
					"repositoryPassword":       "e2e-git-" + t.Name(),
				})
				id := stackID(t, created, "stacks.create_docker_standalone_repository")
				registerStackCleanup(t, leg.Name, envID, id)
				waitStackSettled(t, leg.Name, id)

				// The server's own answer, first: without this the assertion
				// below could pass against a Portainer that never stored the
				// credential at all.
				rawGit, ok := rawStack(t, leg.Name, id)["GitConfig"].(map[string]any)
				if !ok {
					t.Fatalf("the raw API returned no GitConfig for stack %d", id)
				}
				rawAuth, ok := rawGit["Authentication"].(map[string]any)
				if !ok {
					t.Fatalf("the raw API returned no GitConfig.Authentication for stack %d: this test cannot show redaction removed something the server never sent: %v", id, rawGit)
				}
				if got, _ := rawAuth["Username"].(string); got != username {
					t.Fatalf("the raw API returned GitConfig.Authentication.Username %q, want %q", got, username)
				}

				// ... and now the same field through each action that
				// returns a stack.
				for action, out := range map[string]map[string]any{
					"stacks.create_docker_standalone_repository": created,
					"stacks.inspect": callAction[map[string]any](t, session, surface, "stacks.inspect", map[string]any{"id": id}),
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

				listed := callAction[[]map[string]any](t, session, surface, "stacks.list", map[string]any{})
				for _, entry := range listed {
					entryID, ok := entry["Id"].(float64)
					if !ok || int(entryID) != id {
						continue
					}
					gitConfig, ok := entry["GitConfig"].(map[string]any)
					if !ok {
						t.Errorf("stacks.list entry for stack %d carries no GitConfig: %v", id, entry)
						continue
					}
					if auth, present := gitConfig["Authentication"]; present {
						t.Errorf("stacks.list returned GitConfig.Authentication %v for stack %d", auth, id)
					}
				}
			})
		}
	}
}

// TestStacks_UpdateGit_ChangesTheStoredGitConfigurationWithoutRedeploying
// covers stacks.update_git, whose whole point is that it changes where a
// stack pulls from and deploys NOTHING — the action's own narrative says so,
// and stacks.git_redeploy is the one that deploys.
//
// Two properties, and the second is what makes it more than a 200 check:
// the new reference name is readable back through stacks.inspect, and the
// stored stack file is byte-identical to what it was before the call. A
// handler that quietly redeployed would still satisfy the first.
func TestStacks_UpdateGit_ChangesTheStoredGitConfigurationWithoutRedeploying(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("stackgitcfg")
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_repository", map[string]any{
					"endpointId":    envID,
					"name":          name,
					"repositoryUrl": harness.GitFixtureRepositoryURL,
					"composeFile":   harness.GitFixtureConfigFilePath,
				})
				id := stackID(t, created, "stacks.create_docker_standalone_repository")
				registerStackCleanup(t, leg.Name, envID, id)
				waitStackSettled(t, leg.Name, id)
				before := rawStackFile(t, leg.Name, id)

				const reference = "refs/heads/main"
				updated := callAction[map[string]any](t, session, surface, "stacks.update_git", map[string]any{
					"id":                      id,
					"endpointId":              envID,
					"repositoryReferenceName": reference,
					"configFilePath":          harness.GitFixtureConfigFilePath,
				})
				if got, _ := updated["Id"].(float64); int(got) != id {
					t.Errorf("stacks.update_git returned Id %v, want %d", updated["Id"], id)
				}

				inspected := callAction[map[string]any](t, session, surface, "stacks.inspect", map[string]any{"id": id})
				gitConfig, ok := inspected["GitConfig"].(map[string]any)
				if !ok {
					t.Fatalf("stacks.inspect returned no GitConfig after stacks.update_git: %v", inspected)
				}
				if got, _ := gitConfig["ReferenceName"].(string); got != reference {
					t.Errorf("after stacks.update_git the stored GitConfig.ReferenceName is %q, want %q", got, reference)
				}
				if after := rawStackFile(t, leg.Name, id); after != before {
					t.Errorf("stacks.update_git changed the stored stack file:\ngot  %q\nwant it unchanged at %q", after, before)
				}
			})
		}
	}
}

// TestStacks_GitRedeploy_DeploysTheCommitPushedSinceTheStackWasCreated is
// the proof that stacks.git_redeploy really re-reads the remote, and it is
// one of the three assertions this task required to be proven
// discriminating.
//
// A commit is pushed into the mutable fixture repository between the create
// and the redeploy, and the stored stack file must afterwards be the NEW
// content. Nothing weaker discriminates, and that is measured rather than
// assumed: called a second time with no new commit, this route answers 200
// and leaves the stored file exactly as it was — so a test that redeployed
// an unchanged repository would pass against an implementation that fetched
// nothing at all.
//
// Nothing here runs in parallel, and that is load-bearing rather than
// conservative: all six (leg, surface) pairs share one mutable repository,
// and two concurrent pushes would each assert against content the other had
// already replaced. It is the same repository — and the same rule —
// TestCustomTemplates_GitFetch_ReplacesTheStoredFileWithTheRemotesNewCommit
// uses, and Go runs sequential top-level tests one at a time, so the two
// never overlap either.
func TestStacks_GitRedeploy_DeploysTheCommitPushedSinceTheStackWasCreated(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				session := sessions.For(t, surface, leg.Name)

				name := uniqueName("stackredeploy")
				created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_repository", map[string]any{
					"endpointId":    envID,
					"name":          name,
					"repositoryUrl": harness.GitFixtureMutableRepositoryURL,
					"composeFile":   harness.GitFixtureConfigFilePath,
				})
				id := stackID(t, created, "stacks.create_docker_standalone_repository")
				registerStackCleanup(t, leg.Name, envID, id)
				waitStackSettled(t, leg.Name, id)

				// What Portainer holds before the push, checked against what
				// the repository serves right now rather than against the
				// seeded constant: this test pushes to that repository, so
				// its content is whatever the last run left there, and a
				// constant here would pass exactly once per `make e2e-up`.
				before := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				beforeContent := stackFileContentOf(t, surface, before)
				if served := gitFixtureMutableContent(t); beforeContent != served {
					t.Fatalf("the freshly cloned stack's file is\n%q\nwant what the mutable fixture repository currently serves\n%q", beforeContent, served)
				}

				pushed := stackFileFor(uniqueName("redeploy-revision"))
				// The assertion below is only discriminating while these two
				// differ: a redeploy that did nothing at all would otherwise
				// be indistinguishable from a real one.
				if pushed == beforeContent {
					t.Fatalf("the content about to be pushed is already what the stack stores: this test would prove nothing")
				}
				gitFixtureCommit(t, pushed)

				callAction[map[string]any](t, session, surface, "stacks.git_redeploy", map[string]any{
					"id": id, "endpointId": envID,
				})
				waitStackSettled(t, leg.Name, id)

				after := callAction[map[string]any](t, session, surface, "stacks.file_inspect", map[string]any{"id": id})
				if got := stackFileContentOf(t, surface, after); got != pushed {
					t.Errorf("after stacks.git_redeploy the stored file is\n%q\nwant the commit just pushed\n%q", got, pushed)
				}
			})
		}
	}
}

// TestStacks_Migrate_MovesTheStackToTheBodyEndpointNotTheQueryOne is the
// live half of the split internal/specnaming exists for, and the reason
// cmd/gen_action_inputs refused to generate this operation's handler at all.
//
// POST /stacks/{id}/migrate declares an optional QUERY parameter "endpointId"
// and a required BODY property that also renders to "endpointId". The body
// one is the migration target; the query one is the pre-1.18 fixup that
// merely records where a stack already is. A generated handler would have
// unmarshalled the caller's raw input into apigen.StackMigrateParams, whose
// only field is tagged `json:"endpointId,omitempty"`, and sent the migration
// target as the fixup.
//
// This test sends two DIFFERENT values — endpointIdQuery is the environment
// the stack is in now, endpointId is a second, freshly registered
// environment — and asserts where the stack ENDS UP. That is what makes it
// discriminating, and it is measured, not assumed: issued by hand against
// this estate with the two values swapped, the server answers 200 either way
// and the stack simply lands in the other environment. A test asserting on
// the status code, or one that sent the same value for both, would pass
// while every migration went to the wrong place.
//
// Each (leg, surface) pair registers its own target environment rather than
// reusing the Community Edition leg's agent environment: the Business
// Edition leg has no second Docker environment of its own, and an action
// available on both editions must be exercised on both.
func TestStacks_Migrate_MovesTheStackToTheBodyEndpointNotTheQueryOne(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		sourceEnv := dockerEnvID(t, leg)
		// One target environment per leg, shared by the three surfaces,
		// rather than one per surface. Each surface migrates its own stack
		// into it, so sharing costs nothing — and a Business Edition licence
		// caps registered nodes, so three simultaneous extra environments
		// exhausted CI's allowance while a developer's larger licence
		// absorbed it. The estate's limits belong in the test's design, not
		// in whether it happens to pass on the machine it was written on.
		targetEnv := createSecondDockerEnvironment(t, leg.Name, leg.Server, uniqueName("migrate-target"))
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				if targetEnv == sourceEnv {
					t.Fatalf("the migration target environment is the source environment (%d): this test could not tell the two endpointId fields apart", targetEnv)
				}

				name := uniqueName("stackmigrate")
				id := createStackFixture(t, leg.Name, sourceEnv, name)

				// The stack is renamed as it moves, and that is a
				// requirement of this estate rather than a flourish. Both
				// environments are registered against the SAME Docker
				// daemon (see createSecondDockerEnvironment), so the Compose
				// project the stack already runs is visible from the target
				// environment too, and Portainer refuses the migration with
				// 409 "A stack with the name '...' is already running on
				// endpoint '...'" — measured, on both editions and every
				// surface. Renaming is what the action's own `name` field is
				// for, and it gives this test a second thing to read back.
				migratedName := uniqueName("stackmigrated")
				migrated := callAction[map[string]any](t, session, surface, "stacks.migrate", map[string]any{
					"id": id,
					// The migration TARGET.
					"endpointId": targetEnv,
					// The pre-1.18 fixup, deliberately a different value:
					// the stack really is deployed in sourceEnv, so this is
					// the one legitimate thing to say here, and it must not
					// decide where the stack goes.
					"endpointIdQuery": sourceEnv,
					"name":            migratedName,
				})

				// The response's own claim ...
				if got, _ := migrated["EndpointId"].(float64); int(got) != targetEnv {
					t.Errorf("stacks.migrate returned EndpointId %v, want the body's endpointId %d (not the query's %d)",
						migrated["EndpointId"], targetEnv, sourceEnv)
				}
				if got, _ := migrated["Name"].(string); got != migratedName {
					t.Errorf("stacks.migrate returned Name %q, want the requested %q", got, migratedName)
				}
				// ... and where the stack actually ended up, read back from
				// the server. A migrate that answered with the target and
				// stored the source would pass the check above alone.
				stored := rawStack(t, leg.Name, id)
				after, _ := stored["EndpointId"].(float64)
				if int(after) != targetEnv {
					t.Fatalf("after stacks.migrate the server reports stack %d in environment %d, want %d: the body's endpointId did not decide the destination",
						id, int(after), targetEnv)
				}
				if got, _ := stored["Name"].(string); got != migratedName {
					t.Errorf("after stacks.migrate the server reports stack %d named %q, want %q", id, got, migratedName)
				}
				// The cleanup registered by createStackFixture deletes
				// through sourceEnv, which is no longer where the stack is.
				registerStackCleanup(t, leg.Name, targetEnv, id)
			})
		}
	}
}

// TestStacks_Associate_ReparentsAStackOrphanedByARemovedEnvironment covers
// stacks.associate against a genuine orphan rather than an ordinary stack.
//
// The orphan is made the way orphans really happen, and the way the action's
// own narrative describes: a stack is created in an environment, and that
// environment is then removed from Portainer, leaving the stack record
// pointing at an id that no longer resolves. Measured against this estate,
// DELETE /endpoints/{id} answers 204 and leaves the stack behind exactly so.
//
// The discriminating assertion is the re-parented EndpointId read back from
// the server, plus the swarmId the route stores: swarmId is declared an
// INTEGER on this one route (everywhere else in this API a Swarm cluster
// identifier is a string), and Portainer stores the integer 0 as the string
// "0" — so asserting on it proves the integer really travelled rather than
// being dropped by a type mismatch nothing else in this domain would catch.
func TestStacks_Associate_ReparentsAStackOrphanedByARemovedEnvironment(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		homeEnv := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			// Deliberately NOT parallel across surfaces, unlike the rest of
			// this file. Each surface registers its own extra environment
			// and then deletes it — that deletion is how the test produces a
			// genuinely orphaned stack, so unlike the migrate test above
			// these cannot share one. Run in parallel, three extra
			// environments exist at once and a Business Edition licence's
			// node allowance can refuse the third; run in sequence, only one
			// is ever registered. The cost is wall-clock on a test that is
			// already dominated by its estate calls.
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				session := sessions.For(t, surface, leg.Name)

				doomedEnv := createSecondDockerEnvironment(t, leg.Name, leg.Server, uniqueName("orphan-source"))
				name := uniqueName("stackorphan")
				id := createStackFixture(t, leg.Name, doomedEnv, name)
				// From here on the stack's own cleanup has to go through the
				// environment it will be associated to, since the one it was
				// created in is about to disappear.
				registerStackCleanup(t, leg.Name, homeEnv, id)

				ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
				if err := deleteEnvironmentIfPresent(ctx, leg.Name, doomedEnv); err != nil {
					cancel()
					t.Fatalf("removing environment %d to orphan stack %d: %v", doomedEnv, id, err)
				}
				cancel()
				if got, _ := rawStack(t, leg.Name, id)["EndpointId"].(float64); int(got) != doomedEnv {
					t.Fatalf("stack %d reports environment %d after its own environment was removed, want the dangling %d: this test has no orphan to associate",
						id, int(got), doomedEnv)
				}

				associated := callAction[map[string]any](t, session, surface, "stacks.associate", map[string]any{
					"id":              id,
					"endpointId":      homeEnv,
					"swarmId":         0,
					"orphanedRunning": true,
				})
				if got, _ := associated["EndpointId"].(float64); int(got) != homeEnv {
					t.Errorf("stacks.associate returned EndpointId %v, want %d", associated["EndpointId"], homeEnv)
				}
				if got, _ := associated["SwarmId"].(string); got != "0" {
					t.Errorf("stacks.associate stored SwarmId %q, want %q: the integer swarmId this route alone declares did not reach the server", got, "0")
				}
				if got, _ := rawStack(t, leg.Name, id)["EndpointId"].(float64); int(got) != homeEnv {
					t.Errorf("after stacks.associate the server reports stack %d in environment %d, want %d", id, int(got), homeEnv)
				}
			})
		}
	}
}

// TestStacks_List_FiltersComposeByEnvironmentAndSwarmBySwarmIdSeparately
// pins what the `filters` parameter actually does, which is not what either
// the vendored document or this domain's own narrative said before this test
// measured it (docs/api-divergences.md §2.7 and §6.8).
//
// This is the stacks equivalent of the question §6.7 answered for
// custom_templates.list. It is NOT the same defect: stacks.list's filter is
// a single JSON-encoded string, not a repeated parameter, so nothing here is
// mis-encoded by the generated client. What is wrong is the documented
// USAGE, in two independent ways, and both are asserted here because a model
// following the published description gets a 400 for the first and a silently
// short answer for the second:
//
//  1. EndpointID must be a JSON NUMBER. The narrative's own example wrote it
//     quoted, and the server answers
//     400 "Json: cannot unmarshal ... into Go struct field
//     stacks.stackListOperationFilters.EndpointID of type int".
//  2. EndpointID matches COMPOSE stacks only and SwarmID matches SWARM
//     stacks only, and sending both returns their UNION rather than their
//     intersection. A Swarm stack deployed in environment N is therefore
//     invisible to {"EndpointID":N}, which no wording in the document
//     suggests.
//
// Both fixtures are created through the raw API, and the assertions are
// membership assertions about those two stacks specifically rather than
// about the whole answer: every other test in this file is creating and
// deleting stacks on the same server at the same time.
func TestStacks_List_FiltersComposeByEnvironmentAndSwarmBySwarmIdSeparately(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("no confirmed swarm leg on this estate's docker daemon: this test needs a real swarm stack to tell the two filters apart")
	}
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		swarmID := swarmClusterID(t, leg.Server, envID)

		composeName := uniqueName("stacklist-compose")
		composeID := createStackFixture(t, leg.Name, envID, composeName)
		swarmName := uniqueName("stacklist-swarm")
		swarmStackID := createSwarmStackFixture(t, leg.Name, envID, swarmID, swarmName)

		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				list := func(filters string) []map[string]any {
					in := map[string]any{}
					if filters != "" {
						in["filters"] = filters
					}
					return callAction[[]map[string]any](t, session, surface, "stacks.list", in)
				}

				byEndpoint := list(fmt.Sprintf(`{"EndpointID":%d}`, envID))
				if !stackListed(byEndpoint, composeID) {
					t.Errorf(`stacks.list {"EndpointID":%d} does not carry the compose stack %d (%q)`, envID, composeID, composeName)
				}
				if stackListed(byEndpoint, swarmStackID) {
					t.Errorf(`stacks.list {"EndpointID":%d} carries the swarm stack %d (%q): measured, this filter matches compose stacks only`,
						envID, swarmStackID, swarmName)
				}

				bySwarm := list(fmt.Sprintf(`{"SwarmID":%q}`, swarmID))
				if !stackListed(bySwarm, swarmStackID) {
					t.Errorf(`stacks.list {"SwarmID":%q} does not carry the swarm stack %d (%q)`, swarmID, swarmStackID, swarmName)
				}
				if stackListed(bySwarm, composeID) {
					t.Errorf(`stacks.list {"SwarmID":%q} carries the compose stack %d (%q): measured, this filter matches swarm stacks only`,
						swarmID, composeID, composeName)
				}

				// Both keys together: the UNION, not the intersection. This
				// is the half that would still pass if the two filters were
				// simply ANDed, so it is asserted on its own.
				both := list(fmt.Sprintf(`{"EndpointID":%d,"SwarmID":%q}`, envID, swarmID))
				if !stackListed(both, composeID) || !stackListed(both, swarmStackID) {
					t.Errorf(`stacks.list {"EndpointID":%d,"SwarmID":%q} does not carry both the compose stack %d and the swarm stack %d: measured, the two filters union rather than intersect`,
						envID, swarmID, composeID, swarmStackID)
				}

				// Unfiltered carries both, which is what makes the two
				// exclusions above a property of the filter rather than of
				// the stacks.
				all := list("")
				if !stackListed(all, composeID) || !stackListed(all, swarmStackID) {
					t.Errorf("unfiltered stacks.list does not carry both the compose stack %d and the swarm stack %d", composeID, swarmStackID)
				}

				// Property 1: a quoted EndpointID is refused. Asserted
				// through the surface rather than raw, because the value a
				// model sends arrives here as the caller wrote it.
				assertActionFails(t, session, surface, "stacks.list", map[string]any{
					"filters": fmt.Sprintf(`{"EndpointID":"%d"}`, envID),
				})
			})
		}
	}
}

// createSwarmStackFixture deploys a Swarm stack through the raw Portainer
// API, for the tests whose subject is not creation. It is the Swarm sibling
// of createStackFixture and exists for the same reason.
func createSwarmStackFixture(t *testing.T, ed string, envID int, swarmID, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create swarm stack %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		body := apigen.StackCreateDockerSwarmStringJSONRequestBody{
			Name:             name,
			SwarmID:          swarmID,
			StackFileContent: stackFileFor(name),
		}
		resp, err := client.API.StackCreateDockerSwarmStringWithResponse(ctx,
			&apigen.StackCreateDockerSwarmStringParams{EndpointId: envID}, body)
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			return fmt.Errorf("create swarm stack %q: response carried no id", name)
		}
		id = *resp.JSON200.Id
		return nil
	})

	registerStackCleanup(t, ed, envID, id)
	waitStackSettled(t, ed, id)
	return id
}

// TestStacks_DeleteKubernetesByName_SucceedsWithTheUndocumentedNamespaceParameter
// is the live half of a divergence the unit tests pin on the wire.
//
// Measured against both editions of this estate:
// DELETE /stacks/name/{name}?endpointId=1 answers
// 400 {"message":"Invalid query parameter: namespace","details":"Missing
// query parameter"}, and the identical request with &namespace=default
// answers 204. NEITHER vendored document declares a namespace parameter on
// this route — api/specs/ce-2.44.0.json and api/specs/ee-2.44.0.json both
// list exactly name, external and endpointId. See docs/api-divergences.md
// §3.9.
//
// When that was first measured the action was generated from the document,
// so it carried no field that could hold the parameter and every call it
// could make failed. It is now hand-written and publishes `namespace`, and
// this test asserts the consequence that matters: the call SUCCEEDS.
//
// It deliberately no longer asserts the 400. That failure is now unreachable
// from here — `namespace` is a required field, so omitting it is refused by
// toolutil.ActionSpec.ValidateInput before a request is ever built, and a
// test asserting the refusal would be asserting our own validator while
// reading as though it still measured Portainer. The server-side behaviour
// stays pinned where it can still be observed: the unit test on the query
// string this handler emits, and §3.9's recorded exchange.
func TestStacks_DeleteKubernetesByName_SucceedsWithTheUndocumentedNamespaceParameter(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)

				toolName, args := actionCallParams(t, surface, "stacks.delete_kubernetes_by_name", map[string]any{
					"name":       uniqueName("stackk8sname"),
					"endpointId": envID,
					"namespace":  "default",
				})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("CallTool(%s): %v", toolName, err)
				}
				if res.IsError {
					text := toolResultText(res)
					if strings.Contains(text, "namespace") {
						t.Fatalf("stacks.delete_kubernetes_by_name still fails on the namespace parameter with it supplied, so the hand-written handler is not putting it on the wire: %s", text)
					}
					t.Fatalf("stacks.delete_kubernetes_by_name failed with namespace supplied: %s", text)
				}
			})
		}
	}
}

// TestStacks_KubernetesCreates_AreRefusedByADockerEnvironment covers the
// three Kubernetes create actions on the only environments the compose
// estate has, which are Docker ones.
//
// This is deliberately a negative test, and the reason is a property of the
// estate rather than of the actions: `make e2e-up` provisions Docker
// environments only, and the Kubernetes leg `make e2e-k8s-up` adds is
// excluded from composeLegs (no pinned CA in this process to verify its
// self-signed certificate against — see composeLegs' own doc), so there is
// no Kubernetes environment any session in this file can deploy into. What
// can be proven here is that the server really checks: measured,
// POST /stacks/create/kubernetes/string and .../repository against a Docker
// environment answer 400 "Environment type does not match", and .../url
// answers 500 while fetching the manifest, having got past that check
// because it fetches before it validates.
//
// A positive deployment for these three needs a Kubernetes environment
// registered against a compose leg, which is a change to the estate rather
// than to this file; it is recorded as an open gap in the task report rather
// than papered over here.
func TestStacks_KubernetesCreates_AreRefusedByADockerEnvironment(t *testing.T) {
	cases := []struct {
		action string
		input  func(envID int, name string) map[string]any
	}{
		{
			action: "stacks.create_kubernetes_string",
			input: func(envID int, name string) map[string]any {
				return map[string]any{
					"endpointId": envID, "stackName": name, "namespace": "default",
					"stackFileContent": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n",
				}
			},
		},
		{
			action: "stacks.create_kubernetes_git",
			input: func(envID int, name string) map[string]any {
				return map[string]any{
					"endpointId": envID, "stackName": name, "namespace": "default",
					"repositoryUrl": harness.GitFixtureRepositoryURL,
					"manifestFile":  harness.GitFixtureConfigFilePath,
				}
			},
		},
		{
			action: "stacks.create_kubernetes_url",
			input: func(envID int, name string) map[string]any {
				return map[string]any{
					"endpointId": envID, "stackName": name, "namespace": "default",
					"manifestUrl": harness.GitFixtureRepositoryURL,
				}
			},
		},
	}

	for _, leg := range composeLegs(estate) {
		envID := dockerEnvID(t, leg)
		for _, surface := range surfaceNames {
			for _, tc := range cases {
				t.Run(leg.Name+"/"+surface+"/"+tc.action, func(t *testing.T) {
					t.Parallel()
					session := sessions.For(t, surface, leg.Name)
					assertActionFails(t, session, surface, tc.action, tc.input(envID, uniqueName("stackk8s")))
				})
			}
		}
	}
}

// TestStacks_ImagesStatus_ReportsARealStatusForADeployedStack covers the one
// read-only Business-Edition-only action in this domain.
//
// refresh:true is passed for the same reason
// TestDocker_ServiceImageStatus_AgainstARealSwarmService passes it: the
// sibling operation on this same server was measured answering from a cache
// that outlived the resource entirely (docs/api-divergences.md §2.4), and a
// cached answer would satisfy a "did it come back" test against a stack that
// no longer existed.
//
// "error" is rejected rather than accepted as a pass: it means Portainer
// itself could not determine the status against a real, existing stack,
// which is not a result this test should treat as success. Measured against
// this estate, a freshly deployed busybox stack answers
// {"Status":"updated","Message":""}.
func TestStacks_ImagesStatus_ReportsARealStatusForADeployedStack(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: stacks.images_status is EE-only")
	}
	envID, ok := estate.EE.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate's Business Edition server has no %q environment", harness.EnvironmentDocker)
	}

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")

			id := createStackFixture(t, "EE", envID, uniqueName("stackimages"))
			out := callAction[map[string]any](t, session, surface, "stacks.images_status", map[string]any{
				"id": id, "refresh": true,
			})
			status, ok := out["Status"].(string)
			if !ok || status == "" {
				t.Fatalf("stacks.images_status returned no usable Status: %v", out)
			}
			if status == "error" {
				t.Fatalf("stacks.images_status reported Status=%q (Message=%v) against a real deployed stack; want a real status, not an error",
					status, out["Message"])
			}
		})
	}
}

// TestStacks_Convert_ReturnsGeneratedKubernetesManifests covers
// stacks.convert, the Business-Edition-only action this domain deliberately
// did NOT flag destructive despite the word "convert" and despite it being a
// POST (see its ActionSpec's own comment in internal/tools/stacks/actions.go).
//
// The test asserts both halves of that ruling, and the second is the one
// that makes it a ruling rather than a guess:
//
//  1. the answer really carries generated files — a non-empty "files" map
//     whose content names the converted service, not merely a 200;
//  2. the source stack is untouched afterwards: same type, same stored stack
//     file. An action that actually converted the stack in place would pass
//     the first check alone, and this domain would then have the strongest
//     warning it has missing from a genuinely destructive action.
func TestStacks_Convert_ReturnsGeneratedKubernetesManifests(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: stacks.convert is EE-only")
	}
	envID, ok := estate.EE.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate's Business Edition server has no %q environment", harness.EnvironmentDocker)
	}

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")

			name := uniqueName("stackconvert")
			id := createStackFixture(t, "EE", envID, name)
			beforeType, _ := rawStack(t, "EE", id)["Type"].(float64)
			beforeFile := rawStackFile(t, "EE", id)

			out := callAction[map[string]any](t, session, surface, "stacks.convert", map[string]any{
				"id": id, "targetFormat": "kubernetes", "namespace": "default",
			})
			files, ok := out["files"].(map[string]any)
			if !ok || len(files) == 0 {
				t.Fatalf("stacks.convert returned no generated files: %v", out)
			}
			var combined strings.Builder
			for _, content := range files {
				text, _ := content.(string)
				combined.WriteString(text)
			}
			// The converted manifest describes the source stack's own
			// service. Measured against this estate, a one-service compose
			// stack converts to a Deployment carrying
			// "io.kompose.service: hello".
			if !strings.Contains(combined.String(), "hello") {
				t.Errorf("stacks.convert generated files that do not mention the source stack's own service:\n%s", combined.String())
			}

			afterType, _ := rawStack(t, "EE", id)["Type"].(float64)
			if int(afterType) != int(beforeType) {
				t.Errorf("stacks.convert changed the source stack's Type from %v to %v: this action previews and must not convert in place", beforeType, afterType)
			}
			if after := rawStackFile(t, "EE", id); after != beforeFile {
				t.Errorf("stacks.convert changed the source stack's stored file:\ngot  %q\nwant it unchanged at %q", after, beforeFile)
			}
		})
	}
}

// TestStacks_WebhookInvoke_RedeploysTheStackFromTheRemotesNewCommit covers
// stacks.webhook_invoke, and it is built so a commit is the only thing that
// can make it pass.
//
// A git-backed stack is created with an autoUpdate webhook UUID, a commit is
// pushed into the mutable fixture repository, and the webhook is invoked;
// the stored stack file must afterwards be the pushed content. That matters
// because this route answers 204 with an EMPTY BODY — there is nothing in
// the response to assert on at all — and measured against this estate,
// invoking it on a repository with no new commit also answers 204 and
// changes nothing.
//
// It also confirms live what the action's narrative says about the route
// being public: the invoke below goes through the MCP surface (which sends
// the API key), and the hand probe recorded in the task report showed the
// same UUID answering 204 with no credential at all. The UUID is the only
// thing protecting it.
//
// Sequential, for the same reason the git_redeploy test is: it pushes to the
// one mutable repository the whole suite shares.
func TestStacks_WebhookInvoke_RedeploysTheStackFromTheRemotesNewCommit(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: stacks.webhook_invoke is registered as an EE action")
	}
	envID, ok := estate.EE.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate's Business Edition server has no %q environment", harness.EnvironmentDocker)
	}

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			session := sessions.For(t, surface, "EE")

			name := uniqueName("stackwebhook")
			webhook := newWebhookUUID(t)
			created := callAction[map[string]any](t, session, surface, "stacks.create_docker_standalone_repository", map[string]any{
				"endpointId":    envID,
				"name":          name,
				"repositoryUrl": harness.GitFixtureMutableRepositoryURL,
				"composeFile":   harness.GitFixtureConfigFilePath,
				"autoUpdate":    map[string]any{"webhook": webhook},
			})
			id := stackID(t, created, "stacks.create_docker_standalone_repository")
			registerStackCleanup(t, "EE", envID, id)
			waitStackSettled(t, "EE", id)

			before := rawStackFile(t, "EE", id)
			pushed := stackFileFor(uniqueName("webhook-revision"))
			if pushed == before {
				t.Fatalf("the content about to be pushed is already what the stack stores: this test would prove nothing")
			}
			gitFixtureCommit(t, pushed)

			callAction[map[string]any](t, session, surface, "stacks.webhook_invoke", map[string]any{
				"webhookID": webhook,
			})
			waitStackFileEquals(t, "EE", id, pushed, "stacks.webhook_invoke")
			waitStackSettled(t, "EE", id)
		})
	}

	// The discriminating negative: a well-formed UUID the server never
	// issued must be refused, not silently accepted. Measured, the server
	// answers 404 "Unable to find the stack by webhook ID" — so it really
	// does look the UUID up.
	t.Run("unissued webhook uuid fails", func(t *testing.T) {
		assertActionFails(t, sessions.For(t, "dynamic", "EE"), "dynamic", "stacks.webhook_invoke", map[string]any{
			"webhookID": unissuedWebhookUUID,
		})
	})
}

// TestStacks_EdgeStackWebhookInvoke_UpdatesTheEdgeStackFromTheRemotesNewCommit
// covers the one action in this domain that is not under /stacks at all:
// POST /edge_stacks/webhooks/{webhookID}, which the vendored Business
// Edition document tags "stacks" (see internal/tools/stacks/stacks.go's
// package doc for why this domain owns it and the future edge_stacks domain
// will not).
//
// Its fixture — an edge group and a git-backed edge stack — is built through
// the raw Portainer API, because no domain in this catalog covers either
// resource yet. That is the same rule createStackFixture follows: a fixture
// for an action goes through the API, never through the action.
//
// The discriminating assertion is the edge stack's own Version counter and
// its stored file, both read back from the server. Measured against this
// estate: invoking the webhook with no new commit answers 204 and leaves
// Version at 1; invoking it after a commit answers 204, moves Version to 2
// and replaces the stored file. So a 204 alone proves nothing, and a test
// that redeployed an unchanged repository would pass against an
// implementation that fetched nothing.
//
// Sequential, like the two tests above, because it pushes to the shared
// mutable repository.
func TestStacks_EdgeStackWebhookInvoke_UpdatesTheEdgeStackFromTheRemotesNewCommit(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: stacks.edge_stack_webhook_invoke is EE-only")
	}
	if estate.EdgeEndpointID == 0 {
		t.Skip("this estate has no edge environment: an edge stack needs an edge group with an environment in it")
	}

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			session := sessions.For(t, surface, "EE")

			groupID := createEdgeGroupFixture(t, uniqueName("edgegroup"), estate.EdgeEndpointID)
			webhook := newWebhookUUID(t)
			edgeStackID := createEdgeStackFixture(t, uniqueName("edgestack"), groupID, webhook)

			before := rawEdgeStack(t, edgeStackID)
			beforeVersion, _ := before["Version"].(float64)
			pushed := stackFileFor(uniqueName("edge-revision"))
			if pushed == rawEdgeStackFile(t, edgeStackID) {
				t.Fatalf("the content about to be pushed is already what the edge stack stores: this test would prove nothing")
			}
			gitFixtureCommit(t, pushed)

			callAction[map[string]any](t, session, surface, "stacks.edge_stack_webhook_invoke", map[string]any{
				"webhookID": webhook,
			})

			waitEdgeStackFileEquals(t, edgeStackID, pushed, "stacks.edge_stack_webhook_invoke")

			after := rawEdgeStack(t, edgeStackID)
			afterVersion, _ := after["Version"].(float64)
			if int(afterVersion) <= int(beforeVersion) {
				t.Errorf("after stacks.edge_stack_webhook_invoke the edge stack's Version is %v, want it past %v: the webhook pulled nothing",
					after["Version"], before["Version"])
			}
		})
	}

	// Measured: the server answers 404 "Unable to find edge stack with the
	// specified webhook id" — a different message from the /stacks webhook's
	// own 404, which is what tells the two routes apart.
	t.Run("unissued webhook uuid fails", func(t *testing.T) {
		assertActionFails(t, sessions.For(t, "dynamic", "EE"), "dynamic", "stacks.edge_stack_webhook_invoke", map[string]any{
			"webhookID": unissuedWebhookUUID,
		})
	})
}

// unissuedWebhookUUID is a syntactically valid version-4 UUID Portainer
// never minted. It exists to prove the live server actually looks the
// identifier up rather than accepting any string that passes schema
// validation — the same role fabricatedContainerID plays in docker_test.go,
// and the same "cheat this is written down to forbid" docs/api-divergences.md
// §6.3 names.
const unissuedWebhookUUID = "00000000-0000-4000-8000-000000000000"

// newWebhookUUID mints a version-4 UUID for an autoUpdate webhook.
//
// A real UUID rather than one of uniqueName's readable names, because
// Portainer validates the shape: measured, a create carrying anything else
// is refused with 400 "Invalid request payload: Invalid Webhook format",
// and the /edge_stacks create route refuses it the same way. Two stacks may
// not share one either (409 "Stack name or webhook id is not unique"), and
// every test in this file runs against a server other tests are using at the
// same time, so these are drawn from crypto/rand rather than from a counter
// that two processes could both start at the same value.
func newWebhookUUID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("minting a webhook UUID: %v", err)
	}
	// Version 4, variant RFC 4122 — the two fields Portainer's own parser
	// checks beyond the hyphenated shape.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// createEdgeGroupFixture creates a static edge group holding one
// environment, through the raw Portainer API, and registers its removal.
func createEdgeGroupFixture(t *testing.T, name string, endpointID int) int {
	t.Helper()
	body := map[string]any{
		"name":      name,
		"dynamic":   false,
		"endpoints": []int{endpointID},
		"tagIds":    []int{},
	}
	var created struct {
		ID int `json:"Id"`
	}
	rawEEJSON(t, http.MethodPost, "/edge_groups", body, &created)
	if created.ID == 0 {
		t.Fatalf("creating edge group %q returned no id", name)
	}
	newLedger(t).Register("edge_group", strconv.Itoa(created.ID), func(ctx context.Context) error {
		return deleteEEResourceIfPresent(ctx, fmt.Sprintf("/edge_groups/%d", created.ID))
	})
	return created.ID
}

// createEdgeStackFixture creates a git-backed edge stack carrying an
// autoUpdate webhook, through the raw Portainer API, and registers its
// removal.
//
// deploymentType is 1 ("kubernetes"), and that is measured rather than
// chosen: against this estate's Docker edge environment, deploymentType 0
// ("compose") is refused with 500 "edge stack with config do not match the
// environment type" while 1 is accepted. Nothing in this test depends on the
// edge agent actually applying the manifest — what is under test is
// Portainer's own reaction to the webhook — so the accepted value is the one
// used. See docs/api-divergences.md §2.8.
func createEdgeStackFixture(t *testing.T, name string, edgeGroupID int, webhook string) int {
	t.Helper()
	body := map[string]any{
		"name":                 name,
		"edgeGroups":           []int{edgeGroupID},
		"deploymentType":       1,
		"repositoryURL":        harness.GitFixtureMutableRepositoryURL,
		"filePathInRepository": harness.GitFixtureConfigFilePath,
		"autoUpdate":           map[string]any{"webhook": webhook},
	}
	var created struct {
		ID int `json:"Id"`
	}
	rawEEJSON(t, http.MethodPost, "/edge_stacks/create/repository", body, &created)
	if created.ID == 0 {
		t.Fatalf("creating edge stack %q returned no id", name)
	}
	newLedger(t).Register("edge_stack", strconv.Itoa(created.ID), func(ctx context.Context) error {
		return deleteEEResourceIfPresent(ctx, fmt.Sprintf("/edge_stacks/%d", created.ID))
	})
	return created.ID
}

// rawEdgeStack reads one edge stack straight from the Business Edition
// server's API.
func rawEdgeStack(t *testing.T, id int) map[string]any {
	t.Helper()
	var out map[string]any
	rawEEJSON(t, http.MethodGet, fmt.Sprintf("/edge_stacks/%d", id), nil, &out)
	return out
}

// rawEdgeStackFile reads one edge stack's stored file straight from the
// Business Edition server's API.
func rawEdgeStackFile(t *testing.T, id int) string {
	t.Helper()
	var out struct {
		StackFileContent string
	}
	rawEEJSON(t, http.MethodGet, fmt.Sprintf("/edge_stacks/%d/file", id), nil, &out)
	return out.StackFileContent
}

// rawEEJSON issues one JSON request against the Business Edition server's
// API, bypassing every MCP surface, and decodes the response into out.
//
// It exists because edge groups and edge stacks belong to no domain this
// catalog declares yet, so there is no generated client method and no action
// to reach them through — the same reason gpu_test.go's dockerProxy is raw
// HTTP.
func rawEEJSON(t *testing.T, method, path string, body, out any) {
	t.Helper()
	client := fixtureClient(t, "EE")
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	// An io.Reader interface variable left nil, never a typed nil pointer:
	// portainer.Client.Do hands this straight to http.NewRequestWithContext,
	// which treats a non-nil interface wrapping a nil *bytes.Reader as a body
	// and a nil interface as no body at all.
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding the %s %s payload: %v", method, path, err)
		}
		payload = bytes.NewReader(encoded)
	}

	resp, err := client.Do(ctx, method, path, payload)
	if err != nil {
		t.Fatalf("raw %s %s: %v", method, path, err)
	}
	text := readBody(t, resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("raw %s %s: status %d, body %s", method, path, resp.StatusCode, text)
	}
	if out == nil || text == "" {
		return
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("decoding raw %s %s body %q: %v", method, path, text, err)
	}
}

// deleteEEResourceIfPresent removes one raw Business Edition resource by
// path, treating "already gone" as success.
func deleteEEResourceIfPresent(ctx context.Context, path string) error {
	client, err := rawClientFor("EE")
	if err != nil {
		return err
	}
	resp, err := client.Do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("delete %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete %s: status %d", path, resp.StatusCode)
	}
	return nil
}

// stacksSafeModeMutation is one row of the safe-mode matrix below.
//
// kind is asserted per action rather than ignored, because it is derived
// from the ActionSpec flags this domain argued over at length: the six
// destructive ones (delete, delete_kubernetes_by_name, git_redeploy,
// migrate, webhook_invoke, edge_stack_webhook_invoke) must preview as
// "destructive" and the other fifteen as "mutating".
//
// businessEdition marks the three mutating actions declared edition.EE,
// which need the Business Edition safe-mode session: they are absent from a
// Community Edition catalog entirely, so calling them there would fail as an
// unknown action rather than be intercepted.
type stacksSafeModeMutation struct {
	action          string
	kind            string
	businessEdition bool
	inputFor        func(f stacksSafeModeFixture) map[string]any
}

// stacksSafeModeFixture is what every row of the matrix is given: a real,
// existing stack on the server the preview would have written to, the
// environment it lives in, a name no stack has, and a credential whose
// absence from the preview is the property worth asserting.
type stacksSafeModeFixture struct {
	id       int
	envID    int
	swarmID  string
	name     string
	password string
	webhook  string
}

// stacksSafeModeMutations are the twenty-one mutating actions of this
// domain, each with an input good enough that safe mode is the only reason
// it does not execute. inputFor takes the id of a real, existing stack, so
// nothing here could be refused for naming a stack that does not exist — the
// interception has to be what stops it.
var stacksSafeModeMutations = []stacksSafeModeMutation{
	{
		action: "stacks.create_docker_standalone_string", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"endpointId": f.envID, "name": f.name, "stackFileContent": stackFileFor(f.name)}
		},
	},
	{
		action: "stacks.create_docker_standalone_file", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"endpointId": f.envID, "name": f.name, "file": stackFileFor(f.name)}
		},
	},
	{
		// One of the two rows carrying a credential, which is why the
		// preview's values-versus-names property is asserted at all.
		action: "stacks.create_docker_standalone_repository", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "name": f.name,
				"repositoryUrl":            harness.GitFixtureRepositoryURL,
				"composeFile":              harness.GitFixtureConfigFilePath,
				"repositoryAuthentication": true,
				"repositoryUsername":       "e2e-git-user",
				"repositoryPassword":       f.password,
			}
		},
	},
	{
		action: "stacks.create_docker_swarm_string", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "name": f.name, "swarmId": f.swarmID,
				"stackFileContent": stackFileFor(f.name),
			}
		},
	},
	{
		action: "stacks.create_docker_swarm_file", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "name": f.name, "swarmId": f.swarmID,
				"file": stackFileFor(f.name),
			}
		},
	},
	{
		action: "stacks.create_docker_swarm_repository", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "name": f.name, "swarmId": f.swarmID,
				"repositoryUrl": harness.GitFixtureRepositoryURL,
				"composeFile":   harness.GitFixtureConfigFilePath,
			}
		},
	},
	{
		action: "stacks.create_kubernetes_string", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "stackName": f.name, "namespace": "default",
				"stackFileContent": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + f.name + "\n",
			}
		},
	},
	{
		action: "stacks.create_kubernetes_git", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "stackName": f.name, "namespace": "default",
				"repositoryUrl": harness.GitFixtureRepositoryURL,
				"manifestFile":  harness.GitFixtureConfigFilePath,
			}
		},
	},
	{
		action: "stacks.create_kubernetes_url", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"endpointId": f.envID, "stackName": f.name, "namespace": "default",
				"manifestUrl": harness.GitFixtureRepositoryURL,
			}
		},
	},
	{
		action: "stacks.update", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"id": f.id, "endpointId": f.envID,
				"stackFileContent": stackFileFor(f.name),
			}
		},
	},
	{
		// The second credential-carrying row.
		action: "stacks.update_git", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{
				"id": f.id, "endpointId": f.envID,
				"repositoryUrl":            harness.GitFixtureRepositoryURL,
				"configFilePath":           harness.GitFixtureConfigFilePath,
				"repositoryAuthentication": true,
				"repositoryUsername":       "e2e-git-user",
				"repositoryPassword":       f.password,
			}
		},
	},
	{
		action: "stacks.start", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "endpointId": f.envID}
		},
	},
	{
		action: "stacks.stop", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "endpointId": f.envID}
		},
	},
	{
		action: "stacks.associate", kind: "mutating",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "endpointId": f.envID, "swarmId": 0, "orphanedRunning": true}
		},
	},
	{
		action: "stacks.delete", kind: "destructive",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "endpointId": f.envID}
		},
	},
	{
		action: "stacks.delete_kubernetes_by_name", kind: "destructive",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			// namespace is required and is declared by neither vendored
			// document: the server demands it and the action publishes it
			// so it can be sent at all (docs/api-divergences.md §3.9).
			// Omitting it here is refused by ValidateInput before safe mode
			// is ever reached.
			return map[string]any{"name": f.name, "endpointId": f.envID, "namespace": "default"}
		},
	},
	{
		action: "stacks.git_redeploy", kind: "destructive",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "endpointId": f.envID}
		},
	},
	{
		action: "stacks.migrate", kind: "destructive",
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "endpointId": f.envID, "name": f.name}
		},
	},
	{
		action: "stacks.convert", kind: "mutating", businessEdition: true,
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"id": f.id, "targetFormat": "kubernetes", "namespace": "default"}
		},
	},
	{
		action: "stacks.webhook_invoke", kind: "destructive", businessEdition: true,
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"webhookID": f.webhook}
		},
	},
	{
		action: "stacks.edge_stack_webhook_invoke", kind: "destructive", businessEdition: true,
		inputFor: func(f stacksSafeModeFixture) map[string]any {
			return map[string]any{"webhookID": f.webhook}
		},
	},
}

// TestSafeMode_Stacks_MutatingActionsArePreviewedAndNothingChanges covers all
// twenty-one mutating actions of this domain under safe mode, on all three
// surfaces.
//
// Three properties, and the third is what makes the first two worth
// anything:
//
//  1. the call comes back as a preview naming the action and its kind;
//  2. the preview lists the input's FIELD NAMES and never its values —
//     asserted against a real credential in the create_repository and
//     update_git rows, since that is the case the property exists for
//     (internal/tools/register.go's safeModePreview);
//  3. nothing was written: the stack named by the destructive inputs is
//     still there with its original name and stack file, still deployed in
//     its own environment, and no stack by the name the create inputs used
//     ever appeared.
//
// Property 3 is read back through the raw Portainer API, not through any
// surface, for the same reason assertTagAbsent is: a surface that hid or
// previewed a call and executed it anyway would satisfy 1 and 2 identically.
//
// The eighteen Community-Edition rows run against the Community Edition
// safe-mode session and the three Business-Edition-only rows against the
// Business Edition one: safe-mode interception has nothing to do with
// edition, but an action absent from an edition's catalog is refused as
// unknown before interception can happen, which would make those three rows
// pass for the wrong reason.
func TestSafeMode_Stacks_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	// One existing stack per edition, shared by every row, rather than one
	// per (surface, action) pair. That is a deliberate correction rather than
	// an optimisation: the first version of this test deployed a fresh stack
	// for each of its sixty-three subtests, and sixty-three real Compose
	// deployments landing on one Docker daemon at once made the estate itself
	// the thing under test — stacks sat in StackStatusDeploying past a
	// ninety-second deadline and deletes timed out, none of it telling anyone
	// anything about safe mode. Safe mode never writes, so one stack per
	// edition is all these rows can possibly need, and sharing it costs only
	// that a genuine leak fails every row of that edition instead of one.
	//
	// Created here, on the parent test, so the ledger tears them down once
	// after every parallel subtest has finished.
	fixtures := map[string]stacksSafeModeFixture{}
	baselineNames := map[string]string{}
	baselineFiles := map[string]string{}
	for _, ed := range []string{"CE", "EE"} {
		srv := estate.CE
		if ed == "EE" {
			if !estate.HasBusinessEdition() {
				continue
			}
			srv = estate.EE
		}
		envID, ok := srv.Environment(harness.EnvironmentDocker)
		if !ok {
			t.Fatalf("the %s server has no %q environment", ed, harness.EnvironmentDocker)
		}
		name := uniqueName("stack-safe-existing")
		id := createStackFixture(t, ed, envID, name)
		fixtures[ed] = stacksSafeModeFixture{
			id:      id,
			envID:   envID,
			swarmID: swarmClusterID(t, srv, envID),
			webhook: unissuedWebhookUUID,
		}
		baselineNames[ed] = name
		baselineFiles[ed] = rawStackFile(t, ed, id)
	}

	for _, surface := range surfaceNames {
		for _, mutation := range stacksSafeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()

				ed := "CE"
				session := sessions.SafeMode(t, surface)
				if mutation.businessEdition {
					ed = "EE"
					session = sessions.SafeModeEE(t, surface)
				}
				fixture, ok := fixtures[ed]
				if !ok {
					t.Skipf("no %s stack fixture in this estate: that edition is not provisioned", ed)
				}
				envID := fixture.envID
				existingID := fixture.id
				existingName := baselineNames[ed]
				existingContent := baselineFiles[ed]

				// The two per-row fields: a name no stack has, so a create
				// that leaked through is visible, and a credential whose
				// absence from the preview is the property worth asserting.
				fixture.name = uniqueName("stack-safe")
				fixture.password = "e2e-git-" + uniqueName("secret")
				input := mutation.inputFor(fixture)

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
				if strings.Contains(text, fixture.password) {
					t.Errorf("the safe-mode preview echoed the plaintext repositoryPassword back to the caller: %s", text)
				}

				// Nothing was written.
				assertStackAbsent(t, ed, fixture.name, "safe mode intercepting "+mutation.action)
				after := rawStack(t, ed, existingID)
				if got, _ := after["Name"].(string); got != existingName {
					t.Errorf("stack %d is now named %q, want %q: safe mode let %s through", existingID, got, existingName, mutation.action)
				}
				if got, _ := after["EndpointId"].(float64); int(got) != envID {
					t.Errorf("stack %d is now in environment %d, want %d: safe mode let %s through", existingID, int(got), envID, mutation.action)
				}
				if got := rawStackFile(t, ed, existingID); got != existingContent {
					t.Errorf("stack %d's stack file changed under safe mode's %s:\ngot  %q\nwant %q", existingID, mutation.action, got, existingContent)
				}
			})
		}
	}
}

// assertStackAbsent fails t if a stack named name exists on edition ed's
// server, read directly through the Portainer API rather than through any
// MCP surface — the one check a surface cannot satisfy by hiding, refusing
// or previewing a call and executing it anyway.
func assertStackAbsent(t *testing.T, ed, name, reason string) {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	stacks, err := listAllStacks(ctx, client)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, stack := range stacks {
		if stack.Name != nil && *stack.Name == name {
			t.Errorf("stack %q exists on the %s server despite %s", name, ed, reason)
		}
	}
}

// listAllStacks returns every stack on client's server, unfiltered.
//
// Unfiltered deliberately: stacks.list's `filters` parameter matches Compose
// stacks by environment and Swarm stacks by cluster id and never both at
// once (see TestStacks_List_FiltersComposeByEnvironmentAndSwarmBySwarmIdSeparately
// and docs/api-divergences.md §2.7), so any filter here would make this
// sweep silently blind to half the stacks it is supposed to find.
func listAllStacks(ctx context.Context, client *portainer.Client) ([]apigen.PortainereeStack, error) {
	resp, err := client.API.StackListWithResponse(ctx, &apigen.StackListParams{})
	if err != nil {
		return nil, fmt.Errorf("list stacks: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("list stacks: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, nil
	}
	return *resp.JSON200, nil
}

// deleteOrphanStacks is this domain's cross-run sweep, registered in
// fixtures_test.go's orphanSweeps. Every stack these tests create is named
// with uniqueName, so a run that died between creating one and its own
// cleanup leaves a stack the next run can recognise and remove.
//
// It deletes through each stack's own recorded EndpointID, which DELETE
// /stacks/{id} requires: a stack whose environment has since been removed
// (which TestStacks_Associate_ReparentsAStackOrphanedByARemovedEnvironment
// produces on purpose) cannot be deleted at all until it is re-associated,
// so a 404 from that route is reported rather than swallowed here — unlike
// in deleteStackIfPresent, where the test's own delete is the expected
// reason for it.
func deleteOrphanStacks(ctx context.Context, client *portainer.Client, now time.Time) error {
	stacks, err := listAllStacks(ctx, client)
	if err != nil {
		return err
	}

	var errs []error
	for _, stack := range stacks {
		if stack.Name == nil || stack.Id == nil || stack.EndpointId == nil || !strings.HasPrefix(*stack.Name, orphanPrefix) {
			continue
		}
		if !isOrphanEligible(*stack.Name, now) {
			continue
		}
		delResp, err := client.API.StackDeleteWithResponse(ctx, *stack.Id, &apigen.StackDeleteParams{EndpointId: *stack.EndpointId})
		if err != nil {
			errs = append(errs, fmt.Errorf("delete orphan stack %q: %w", *stack.Name, err))
			continue
		}
		if err := toolutil.Check(delResp); err != nil {
			errs = append(errs, fmt.Errorf("delete orphan stack %q: %w", *stack.Name, err))
		}
	}
	return errors.Join(errs...)
}
