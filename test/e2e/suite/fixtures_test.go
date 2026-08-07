//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// createTag and createRegistry are the only way Task 8's domain suites bring
// a tag or registry into existence: both mint a unique, e2e--prefixed name,
// retry the handful of spurious failures a shared, concurrently-hammered
// estate produces, and register their own teardown with the calling test's
// ledger before returning. There is deliberately no lower-level "create and
// register separately" path, so a suite cannot create one of these without
// also registering its cleanup.

// fixtureRetries is how many times createTag/createRegistry retry a create
// that failed for a reason that looks transient. Every session in the
// matrix can create fixtures concurrently against the same Community
// Edition server, and that concurrency itself produces two kinds of failure
// that have nothing to do with the fixture being wrong: Portainer's own
// uniqueness check races under concurrent creates and answers "already been
// taken" for a name nothing else has used, and the connection between two
// concurrent callers occasionally resets. Five is the sibling project's
// number for the same problem, arrived at the same way: empirically, against
// a real, loaded server.
const fixtureRetries = 5

// fixtureRetryBackoff is the delay between retries, scaled by attempt
// number. It is short because the failures being retried are transient by
// nature; a fixed, small backoff spreads out concurrent retries without
// meaningfully slowing a suite that never needs to retry at all.
const fixtureRetryBackoff = 100 * time.Millisecond

// fixtureClient is the Portainer client every fixture helper talks through,
// one per edition rather than fixed to Community Edition: tags and
// registries are server-wide resources, but Community and Business Edition
// are different servers in this harness's estate (estate.CE and estate.EE),
// so a fixture exercised through a Business Edition session must be created
// against the Business Edition server, not Community's. Task 7 built this
// CE-only; extended here rather than duplicated, because closing that gap is
// this task's job, not a reason to grow a second copy of fixture plumbing.
//
// Built lazily and cached per edition: most test binaries never touch the
// Business Edition leg (a contributor without the licence never asks for
// "EE"), so a client for it is never constructed unless something actually
// requests one.
var (
	fixtureClientsMu sync.Mutex
	fixtureClients   = map[string]*portainer.Client{}
)

// rawClientFor returns the fixture client for ed ("CE" or "EE") without a
// *testing.T, for the handful of callers — ledger cleanup closures — that
// only have a context. fixtureClient below is the *testing.T-friendly
// wrapper every test calls directly.
func rawClientFor(ed string) (*portainer.Client, error) {
	fixtureClientsMu.Lock()
	defer fixtureClientsMu.Unlock()

	if c, ok := fixtureClients[ed]; ok {
		return c, nil
	}

	// Looked up by name from estate.Legs() rather than a switch naming "CE"
	// and "EE" as its only two cases: the derived leg list is the one place
	// task 7b's edition/platform axis is defined, and a name this estate
	// never provisioned (an unlicensed "EE", a typo, or "Kubernetes" today,
	// since no fixture here builds a client for it) is "not found", exactly
	// like a name this estate really never heard of.
	var srv harness.Server
	var found bool
	for _, leg := range estate.Legs() {
		if leg.Name == ed {
			srv, found = leg.Server, true
			break
		}
	}
	if !found || srv.BaseURL == "" {
		return nil, fmt.Errorf("rawClientFor: no %s server in this estate", ed)
	}

	client, err := portainer.New(&config.Config{URL: srv.BaseURL, Token: srv.Creds.APIKey})
	if err != nil {
		return nil, fmt.Errorf("build %s fixture client: %w", ed, err)
	}
	fixtureClients[ed] = client
	return client, nil
}

func fixtureClient(t *testing.T, ed string) *portainer.Client {
	t.Helper()
	if ed == "EE" && !estate.HasBusinessEdition() {
		t.Skipf("no Business Edition server in this estate")
	}
	client, err := rawClientFor(ed)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return client
}

// createTag creates an environment tag named name on edition ed's server and
// returns its id. The caller is expected to have built name with uniqueName
// so orphan cleanup can find it if this test's own cleanup never runs.
//
// Its registered cleanup deletes the tag only if it is still present:
// tags.delete is exactly the mutation Task 8's own tests exercise against
// this fixture, so by the time a test ends the tag it created may already be
// gone. A cleanup that deleted unconditionally would then fail on an
// already-deleted tag and report a spurious failure for a test that passed.
func createTag(t *testing.T, ed, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create tag %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		resp, err := client.API.TagCreateWithResponse(ctx, apigen.TagCreateJSONRequestBody{Name: name})
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.ID == nil {
			return fmt.Errorf("create tag %q: response carried no id", name)
		}
		id = *resp.JSON200.ID
		return nil
	})

	newLedger(t).Register("tag", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteTagIfPresent(ctx, ed, id)
	})
	return id
}

// deleteTagIfPresent deletes tag id on edition ed's server only if a
// TagList still reports it, so a caller that already deleted it (through the
// action under test, not through this helper) does not turn a passing test
// into a spurious cleanup failure.
func deleteTagIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.TagListWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list tags before cleanup: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list tags before cleanup: %w", err)
	}
	if resp.JSON200 != nil {
		present := false
		for _, tag := range *resp.JSON200 {
			if tag.ID != nil && *tag.ID == id {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
	}
	delResp, err := client.API.TagDeleteWithResponse(ctx, id)
	if err != nil {
		return err
	}
	return toolutil.Check(delResp)
}

// createRegistry creates a custom-type registry named name on edition ed's
// server and returns its id. Like createTag, the caller is expected to have
// built name with uniqueName, and its registered cleanup only deletes the
// registry if it is still present, for the identical reason.
//
// It uses the generic "custom registry" type rather than a real registry
// kind (Docker Hub, ECR, ...) because nothing here ever pulls an image
// through it: the fixture only needs a registry record to exist so a suite
// can exercise list/inspect/update/delete against it, and a custom registry
// is the one type that stores whatever URL it is given without validating
// that anything is listening there.
func createRegistry(t *testing.T, ed, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create registry %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		body := apigen.RegistryCreateJSONRequestBody{
			Name: name,
			URL:  fmt.Sprintf("%s.example.invalid", name),
			Type: apigen.PortainerRegistryTypeCustomRegistry,
		}
		resp, err := client.API.RegistryCreateWithResponse(ctx, body)
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			return fmt.Errorf("create registry %q: response carried no id", name)
		}
		id = *resp.JSON200.Id
		return nil
	})

	newLedger(t).Register("registry", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteRegistryIfPresent(ctx, ed, id)
	})
	return id
}

// deleteRegistryIfPresent is deleteTagIfPresent's counterpart for registries.
func deleteRegistryIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.RegistryListWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list registries before cleanup: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list registries before cleanup: %w", err)
	}
	if resp.JSON200 != nil {
		present := false
		for _, reg := range *resp.JSON200 {
			if reg.Id != nil && *reg.Id == id {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
	}
	delResp, err := client.API.RegistryDeleteWithResponse(ctx, id)
	if err != nil {
		return err
	}
	return toolutil.Check(delResp)
}

// retryFixture calls fn up to fixtureRetries times, retrying only when the
// failure looks transient (see isRetryableFixtureError), and fails t with
// every attempt's error once retries are exhausted or a non-transient error
// is seen.
func retryFixture(t *testing.T, label string, fn func() error) {
	t.Helper()

	var lastErr error
	for attempt := 1; attempt <= fixtureRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return
		}
		if !isRetryableFixtureError(lastErr) {
			t.Fatalf("%s: %v", label, lastErr)
		}
		if attempt < fixtureRetries {
			time.Sleep(time.Duration(attempt) * fixtureRetryBackoff)
		}
	}
	t.Fatalf("%s: failed after %d attempts: %v", label, fixtureRetries, lastErr)
}

// isRetryableFixtureError reports whether err looks like the transient noise
// a shared, concurrently-hammered estate produces rather than a genuine
// failure: Portainer's own name-uniqueness check racing under concurrent
// creates ("already been taken" for a name nothing else used), or a
// connection reset/EOF from two callers hitting the server at once.
func isRetryableFixtureError(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	for _, sig := range []string{"already been taken", "connection reset", "EOF"} {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

// isHarnessProvisioned reports whether the server client talks to is the
// same one this harness recorded srv as being, by re-reading its live
// Server Instance ID (GET /system/status) and comparing it against
// srv.InstanceID, the value cmd/provision/main.go's provisionServer recorded
// there at provisioning time (see harness.Server.InstanceID and
// harness.WaitReady's ReadyStatus).
//
// This replaced an earlier guard that compared an estate's stored
// credentials against a fixed, published administrator password. That guard
// stopped being possible the moment harness.Provision began minting a random
// password every run (see provision.go's generateAdminPassword): nothing
// about the credentials sitting in a JSON file can prove anything once the
// password is no longer a constant to compare against. InstanceID is
// different in kind, not just in value — Portainer persists it in its own
// /data volume and it is stable across restarts, but it can only be
// produced by asking a running server, live, right now. An estate's JSON
// file recording the right InstanceID proves nothing by itself; only a
// server that answers with the same one, at sweep time, does.
//
// It is the one signal cleanupOrphans is allowed to act on before deleting
// anything: an estate's URL and API key are just data in a JSON file, and
// nothing stops that file from being pointed, by mistake or by hand, at a
// real Portainer instead of a disposable one. Every failure mode — no
// InstanceID was ever recorded, the live read errors out, or the live and
// recorded values simply disagree — is treated identically: "cannot confirm
// this is ours", reported as an error rather than resolved to false, so a
// transient read failure can never look like "confirmed not ours" to a
// caller checking only a boolean.
func isHarnessProvisioned(ctx context.Context, client *portainer.Client, srv harness.Server) error {
	if srv.InstanceID == "" {
		return errors.New("no instance id was recorded for this server when it was provisioned")
	}
	resp, err := client.API.SystemStatusWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("read live system status: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("read live system status: %w", err)
	}
	if resp.JSON200 == nil || resp.JSON200.InstanceID == nil || *resp.JSON200.InstanceID == "" {
		return errors.New("live system status carried no instance id")
	}
	if *resp.JSON200.InstanceID != srv.InstanceID {
		return fmt.Errorf("live instance id %q does not match %q recorded at provisioning time",
			*resp.JSON200.InstanceID, srv.InstanceID)
	}
	return nil
}

// orphanPrefix is what cleanupOrphans searches for. It is the fixed part of
// every name uniqueName produces, regardless of prefix or run.
const orphanPrefix = "e2e-"

// orphanNamePattern extracts the process id and unix-nanosecond timestamp
// uniqueName embeds in every name it mints: e2e-<prefix>-<pid>-<unixnano>-
// <counter> (see names_test.go's runID). cleanupOrphans uses the timestamp
// to tell a genuinely orphaned fixture — left behind by a run that crashed
// some time ago — apart from one a concurrently running invocation created
// moments ago and has not gotten around to cleaning up yet. "starts with
// e2e-" alone cannot make that distinction, and names_test.go's own runID
// doc states that two concurrent `go test -tags e2e` invocations are
// expected to coexist: a sweep that ignores age would delete a live sibling
// run's in-flight fixtures out from under it.
//
// The prefix between "e2e-" and the trailing pid/timestamp/counter (for
// example "tag", or "registry-renamed" once TestRegistries_* has renamed
// one) can itself contain dashes, so the pattern anchors the three numeric
// groups from the end of the string via "$" rather than assuming a fixed
// number of leading segments.
var orphanNamePattern = regexp.MustCompile(`^e2e-.+-([0-9]+)-([0-9]+)-([0-9]+)$`)

// orphanMinAge is how long a name must have existed, by its own embedded
// timestamp, before cleanupOrphans is willing to delete it. A concurrent
// run's own fixtures are always younger than this at the moment the sweep
// runs — TestMain calls cleanupOrphans once, before that run's own tests
// have had a chance to create anything, and the whole matrix in this
// package finishes in well under this threshold. A run dead long enough to
// leave real orphans behind is dead by an order of magnitude more.
const orphanMinAge = 30 * time.Minute

// isOrphanEligible reports whether name is old enough, by its own embedded
// timestamp, for cleanupOrphans to delete it. A name cleanupOrphans cannot
// parse the standard shape from is treated as eligible rather than
// permanently protected: uniqueName is the only thing that mints e2e--
// prefixed names in this suite, so one that does not match its shape is
// unlikely, and refusing to ever sweep it would leave a permanent orphan
// nothing can reach.
func isOrphanEligible(name string, now time.Time) bool {
	m := orphanNamePattern.FindStringSubmatch(name)
	if m == nil {
		return true
	}
	nanos, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return true
	}
	return now.Sub(time.Unix(0, nanos)) >= orphanMinAge
}

// composeLegs returns e's ordinary compose-provisioned legs (CE always, EE
// when licensed) — Estate.Legs() filtered to drop the Kubernetes leg, when
// present.
//
// The Kubernetes leg is excluded here, not in Estate.Legs() itself: it
// deploys a self-signed certificate that this test process has no pinned CA
// to verify (see buildSession's skipTLSVerify doc and
// kubernetes_test.go), so a plain portainer.New(&config.Config{URL: ...})
// against it — what both of this function's callers (cleanupOrphans and
// newSessions) build — would fail on every call with a certificate error,
// not the specific TLS handling that leg actually needs. This is a genuine
// technical constraint on how this leg is reached, not a re-introduction of
// the hardcoded-axis problem Estate.Legs() itself exists to close: fixing it
// (teaching this function to skip TLS verification for the Kubernetes leg
// specifically) is orthogonal to task 7's three gaps and left for whenever a
// domain suite actually needs cross-run fixture cleanup or a matrix session
// against that leg.
func composeLegs(e harness.Estate) []harness.Leg {
	var legs []harness.Leg
	for _, leg := range e.Legs() {
		if leg.Name == "Kubernetes" {
			continue
		}
		legs = append(legs, leg)
	}
	return legs
}

// orphanSweep is one resource kind's cross-run cleanup: given a client
// already proven (by cleanupOrphans, via isHarnessProvisioned) to be talking
// to a server this harness provisioned, list what still exists and delete
// whatever is eligible.
//
// cleanupOrphans is deliberately ignorant of what a sweep function actually
// checks. deleteOrphanTags and deleteOrphanRegistries below both key on a
// name prefix and an embedded timestamp (orphanPrefix, isOrphanEligible), but
// nothing about this type requires that: a sweep for containers or volumes
// created *through* Portainer, or namespaces, could match a Docker/Kubernetes
// label instead of a name; one for a numerically identified resource
// (schedules, resource controls, access policies) could match against ids a
// fixture wrote to some other cache entirely. Every one of those decisions
// lives inside the sweep function itself, not in cleanupOrphans.
type orphanSweep func(ctx context.Context, client *portainer.Client, now time.Time) error

// namedOrphanSweep pairs an orphanSweep with a name used only to identify
// which sweep failed in a wrapped error — cleanupOrphans itself never
// branches on it.
type namedOrphanSweep struct {
	name  string
	sweep orphanSweep
}

// orphanSweeps is the registration point every fixture helper that needs
// cross-run cleanup adds itself to, instead of cleanupOrphans hardcoding a
// call to it by name.
//
// This is the fix for the gap task 7c's brief names directly: "every new
// resource type needs a bespoke pair of functions, about twenty of them."
// Adding tags and registries this way — as two entries here, in a slice
// cleanupOrphans merely ranges over — means the ~20 resource kinds P3
// domains will eventually need cross-run cleanup for each add one entry to
// this slice (or, if that resource's own file prefers to keep its cleanup
// logic local, append to this slice from an init in that file) rather than
// teaching cleanupOrphans a twentieth bespoke pair of functions apiece. See
// TestCleanupOrphans_CallsEveryRegisteredSweeper for the proof that
// cleanupOrphans genuinely does not need to know what "tags" or "registries"
// are to sweep them.
var orphanSweeps = []namedOrphanSweep{
	{name: "tags", sweep: deleteOrphanTags},
	{name: "registries", sweep: deleteOrphanRegistries},
}

// cleanupOrphans runs every registered orphanSweep (see orphanSweeps) against
// every compose-provisioned leg of e. It is the net for a run that died
// between creating a resource and its own cleanup running: nothing else
// distinguishes one test's fixture from another's on a server every session
// in the matrix shares, so a run that dies mid-test leaves its fixtures
// behind for the next run to trip over unless something like this sweeps
// first.
//
// It refuses to touch a leg that does not pass isHarnessProvisioned,
// returning an error instead of silently skipping it: skipping would look
// identical to "nothing to clean up" from the caller's side, and the whole
// point of the guard is to fail loudly rather than delete when unsure. This
// guard is unconditional and untouched by the registration mechanism above:
// every sweep, whatever it matches on, only ever runs against a client this
// check has already approved.
func cleanupOrphans(ctx context.Context, e harness.Estate) error {
	now := time.Now()
	for _, leg := range composeLegs(e) {
		client, err := portainer.New(&config.Config{URL: leg.Server.BaseURL, Token: leg.Server.Creds.APIKey})
		if err != nil {
			return fmt.Errorf("cleanupOrphans: build %s client: %w", leg.Name, err)
		}

		if err := isHarnessProvisioned(ctx, client, leg.Server); err != nil {
			return fmt.Errorf("cleanupOrphans: %s server %s does not carry this harness's own "+
				"provisioning identity; refusing to delete e2e- resources against a server this "+
				"harness cannot confirm it provisioned: %w", leg.Name, leg.Server.BaseURL, err)
		}

		for _, s := range orphanSweeps {
			if err := s.sweep(ctx, client, now); err != nil {
				return fmt.Errorf("cleanupOrphans: %s: %s: %w", leg.Name, s.name, err)
			}
		}
	}
	return nil
}

func deleteOrphanTags(ctx context.Context, client *portainer.Client, now time.Time) error {
	resp, err := client.API.TagListWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	if resp.JSON200 == nil {
		return nil
	}

	var errs []error
	for _, tag := range *resp.JSON200 {
		if tag.Name == nil || tag.ID == nil || !strings.HasPrefix(*tag.Name, orphanPrefix) {
			continue
		}
		if !isOrphanEligible(*tag.Name, now) {
			continue
		}
		delResp, err := client.API.TagDeleteWithResponse(ctx, *tag.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("delete orphan tag %q: %w", *tag.Name, err))
			continue
		}
		if err := toolutil.Check(delResp); err != nil {
			errs = append(errs, fmt.Errorf("delete orphan tag %q: %w", *tag.Name, err))
		}
	}
	return errors.Join(errs...)
}

func deleteOrphanRegistries(ctx context.Context, client *portainer.Client, now time.Time) error {
	resp, err := client.API.RegistryListWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list registries: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list registries: %w", err)
	}
	if resp.JSON200 == nil {
		return nil
	}

	var errs []error
	for _, reg := range *resp.JSON200 {
		if reg.Name == nil || reg.Id == nil || !strings.HasPrefix(*reg.Name, orphanPrefix) {
			continue
		}
		if !isOrphanEligible(*reg.Name, now) {
			continue
		}
		delResp, err := client.API.RegistryDeleteWithResponse(ctx, *reg.Id)
		if err != nil {
			errs = append(errs, fmt.Errorf("delete orphan registry %q: %w", *reg.Name, err))
			continue
		}
		if err := toolutil.Check(delResp); err != nil {
			errs = append(errs, fmt.Errorf("delete orphan registry %q: %w", *reg.Name, err))
		}
	}
	return errors.Join(errs...)
}

// fakeSystemStatusServer stands in for a live Portainer's GET
// /api/system/status, answering the fixed instanceID every call.
func fakeSystemStatusServer(t *testing.T, instanceID string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"Version":"2.44.0","InstanceID":%q}`, instanceID)
	}))
	t.Cleanup(server.Close)
	return server
}

// TestIsHarnessProvisioned_LiveInstanceIDMatchesRecorded_Permits and
// TestIsHarnessProvisioned_LiveInstanceIDDiffersFromRecorded_Refuses are the
// mutation-tested proof, in both directions, for the guard that replaced
// comparing against harness.AdminPassword: a random per-run password (see
// provision.go's generateAdminPassword) makes that comparison impossible, so
// the guard now re-reads the server's live Server Instance ID and requires
// it to match what was recorded in the estate at provisioning time. A guard
// that always refuses would pass the "differs" case here while failing the
// "matches" case below; a guard that always permits is the reverse — only
// exercising both catches either defect.
func TestIsHarnessProvisioned_LiveInstanceIDMatchesRecorded_Permits(t *testing.T) {
	t.Parallel()
	server := fakeSystemStatusServer(t, "same-instance-id")
	client, err := portainer.New(&config.Config{URL: server.URL, Token: "irrelevant"})
	if err != nil {
		t.Fatalf("portainer.New() error = %v", err)
	}
	srv := harness.Server{BaseURL: server.URL, InstanceID: "same-instance-id"}

	if err := isHarnessProvisioned(context.Background(), client, srv); err != nil {
		t.Errorf("isHarnessProvisioned() error = %v, want nil: the live and recorded instance ids match", err)
	}
}

func TestIsHarnessProvisioned_LiveInstanceIDDiffersFromRecorded_Refuses(t *testing.T) {
	t.Parallel()
	server := fakeSystemStatusServer(t, "a-different-instance-id")
	client, err := portainer.New(&config.Config{URL: server.URL, Token: "irrelevant"})
	if err != nil {
		t.Fatalf("portainer.New() error = %v", err)
	}
	srv := harness.Server{BaseURL: server.URL, InstanceID: "recorded-instance-id"}

	if err := isHarnessProvisioned(context.Background(), client, srv); err == nil {
		t.Error("isHarnessProvisioned() error = nil, want an error: the live instance id does not match " +
			"the one recorded at provisioning time")
	}
}

// TestIsHarnessProvisioned_NoRecordedInstanceID_Refuses covers the estate a
// provisioning run never finished writing (or one from before this field
// existed): with nothing recorded to compare against, the guard must refuse
// rather than treat the absence as "nothing to check".
func TestIsHarnessProvisioned_NoRecordedInstanceID_Refuses(t *testing.T) {
	t.Parallel()
	// BaseURL points nowhere reachable on purpose: the empty-InstanceID check
	// must short-circuit before any network call, so this failing for the
	// wrong reason (a dial error) would be its own bug.
	srv := harness.Server{BaseURL: "https://not-ours.example.invalid"}
	client, err := portainer.New(&config.Config{URL: srv.BaseURL, Token: "irrelevant"})
	if err != nil {
		t.Fatalf("portainer.New() error = %v", err)
	}

	if err := isHarnessProvisioned(context.Background(), client, srv); err == nil {
		t.Error("isHarnessProvisioned() error = nil, want an error when no instance id was ever recorded")
	}
}

// TestIsOrphanEligible_OnlySweepsNamesOlderThanTheThreshold proves the fix
// for cleanupOrphans' concurrency hazard: names_test.go's runID exists
// specifically so two concurrent `go test -tags e2e` invocations coexist,
// but a sweep keyed only on the "e2e-" prefix would delete a live sibling
// run's in-flight fixtures the moment this run's own TestMain started.
// uniqueName's embedded unix-nanosecond timestamp is what lets cleanupOrphans
// tell the two apart; this test exercises that decision directly, without
// a live estate.
func TestIsOrphanEligible_OnlySweepsNamesOlderThanTheThreshold(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	freshRunID := fmt.Sprintf("12345-%d", now.Add(-time.Minute).UnixNano())
	staleRunID := fmt.Sprintf("12345-%d", now.Add(-2*orphanMinAge).UnixNano())

	tests := []struct {
		name      string
		fixture   string
		wantSwept bool
	}{
		{
			name:      "fresh: a concurrently running sibling's own fixture",
			fixture:   fmt.Sprintf("e2e-tag-%s-1", freshRunID),
			wantSwept: false,
		},
		{
			name:      "stale: left behind by a run that died long ago",
			fixture:   fmt.Sprintf("e2e-tag-%s-1", staleRunID),
			wantSwept: true,
		},
		{
			name:      "dashes in the fixture's own prefix do not confuse the timestamp extraction",
			fixture:   fmt.Sprintf("e2e-registry-renamed-%s-7", staleRunID),
			wantSwept: true,
		},
		{
			name:      "unparseable shape is swept rather than permanently protected",
			fixture:   "e2e-not-shaped-like-uniqueName-produces",
			wantSwept: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isOrphanEligible(tt.fixture, now); got != tt.wantSwept {
				t.Errorf("isOrphanEligible(%q) = %v, want %v", tt.fixture, got, tt.wantSwept)
			}
		})
	}
}

func TestCleanupOrphans_RefusesAnEstateWithoutARecordedInstanceID(t *testing.T) {
	t.Parallel()
	// BaseURL points nowhere reachable on purpose: if the identity guard were
	// bypassed, cleanupOrphans would still fail here, but for an unrelated
	// reason (a network error dialing an invalid host) that would let this
	// test pass even with the guard broken. Asserting on the guard's own
	// wording, rather than merely "err != nil", is what makes this test fail
	// if the guard itself stops firing. No InstanceID is recorded on this
	// Server, which the guard rejects before ever attempting to reach the
	// (unreachable) host.
	e := harness.Estate{CE: harness.Server{BaseURL: "https://not-ours.example.invalid"}}
	err := cleanupOrphans(context.Background(), e)
	if err == nil {
		t.Fatal("cleanupOrphans accepted an estate whose CE server carries no recorded instance id")
	}
	const wantSubstring = "cannot confirm it provisioned"
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("cleanupOrphans error = %q, want it to contain %q (the credential guard's own reason, "+
			"not a network failure reaching an unreachable host)", err.Error(), wantSubstring)
	}
}

// TestCleanupOrphans_CallsEveryRegisteredSweeper is task 7c's proof:
// cleanupOrphans must not need to know what "tags" or "registries" are to
// sweep them. It temporarily replaces the package-level orphanSweeps
// registry with two fakes that record their own invocation, points
// cleanupOrphans at a fake server that satisfies isHarnessProvisioned, and
// requires both fakes to have run.
//
// This is the test the brief's own trap warns against writing trivially: a
// cleanupOrphans that quietly kept calling deleteOrphanTags and
// deleteOrphanRegistries directly — reverting the registration mechanism
// while leaving this test's shape unchanged — would leave "called" empty,
// because neither of those functions is one of the two fakes this test
// registered. Only a cleanupOrphans that genuinely iterates orphanSweeps can
// pass this.
//
// Not run with t.Parallel(): it swaps the package-level orphanSweeps var for
// the duration of the call, and TestCleanupOrphans_RefusesAnEstateWithoutARecordedInstanceID
// is the only other test that calls cleanupOrphans — it returns before ever
// reading orphanSweeps (isHarnessProvisioned fails first on that estate), but
// keeping this one serial avoids relying on that ordering under -race.
func TestCleanupOrphans_CallsEveryRegisteredSweeper(t *testing.T) {
	server := fakeSystemStatusServer(t, "matching-instance-id")

	original := orphanSweeps
	t.Cleanup(func() { orphanSweeps = original })

	var called []string
	orphanSweeps = []namedOrphanSweep{
		{name: "fake-a", sweep: func(context.Context, *portainer.Client, time.Time) error {
			called = append(called, "fake-a")
			return nil
		}},
		{name: "fake-b", sweep: func(context.Context, *portainer.Client, time.Time) error {
			called = append(called, "fake-b")
			return nil
		}},
	}

	e := harness.Estate{CE: harness.Server{BaseURL: server.URL, InstanceID: "matching-instance-id"}}
	if err := cleanupOrphans(context.Background(), e); err != nil {
		t.Fatalf("cleanupOrphans() error = %v", err)
	}
	if !reflect.DeepEqual(called, []string{"fake-a", "fake-b"}) {
		t.Errorf("sweeps called = %v, want [fake-a fake-b]: cleanupOrphans must invoke every "+
			"registered sweeper without knowing its type", called)
	}
}

// TestComposeLegs_ExcludesKubernetes proves composeLegs' one deliberate
// deviation from Estate.Legs(): a fully provisioned estate's Kubernetes leg
// must not appear, because neither of composeLegs' callers can reach it
// (self-signed certificate, no pinned CA in this process — see composeLegs'
// own doc), while both compose legs must.
func TestComposeLegs_ExcludesKubernetes(t *testing.T) {
	t.Parallel()
	e := harness.Estate{
		CE:         harness.Server{Edition: "CE", BaseURL: "http://ce"},
		EE:         harness.Server{Edition: "EE", BaseURL: "http://ee", Creds: harness.Credentials{APIKey: "ee-key"}},
		Kubernetes: harness.Server{Edition: "Kubernetes", BaseURL: "https://k8s", Creds: harness.Credentials{APIKey: "k8s-key"}},
	}
	legs := composeLegs(e)
	if len(legs) != 2 {
		t.Fatalf("composeLegs() returned %d legs, want 2 (CE and EE): %+v", len(legs), legs)
	}
	for _, leg := range legs {
		if leg.Name == "Kubernetes" {
			t.Errorf("composeLegs() included the Kubernetes leg: %+v", legs)
		}
	}
}

func TestRetryFixture_RetriesOnlyTransientErrors(t *testing.T) {
	t.Parallel()

	t.Run("retries a transient error then succeeds", func(t *testing.T) {
		attempts := 0
		retryFixture(t, "op", func() error {
			attempts++
			if attempts < 3 {
				return errors.New("tag management: name has already been taken")
			}
			return nil
		})
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})
}

func TestIsRetryableFixtureError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"already taken", errors.New(`name "e2e-tag-1" has already been taken`), true},
		{"connection reset", errors.New("read tcp: connection reset by peer"), true},
		{"eof", io.EOF, true},
		{"unrelated failure", errors.New("http 500: internal server error"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableFixtureError(tt.err); got != tt.want {
				t.Errorf("isRetryableFixtureError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// kubectlRunError formats an error from running kubectl for
// kubernetesNodeCapacity's own diagnostic, surfacing the process's stderr
// when the failure was a non-zero exit.
//
// *exec.ExitError's own Error() method — and therefore a bare "%v" — reports
// only "exit status 1": it never includes anything the process itself wrote
// to explain why. exec.Cmd.Output() already captures that text into
// ExitError.Stderr whenever the caller left Cmd.Stderr nil (kubernetesNodeCapacity's
// own case, via a bare .Output() call), so the information was always
// present, just discarded at the point the message was built. A wrong
// E2E_K3D_CLUSTER or a stale/deleted context both fail this exact way, and
// without this, both looked identical from the test output: "reading node
// capacity through kubectl: exit status 1", with no way to tell "no such
// context" apart from "no such cluster" apart from anything else kubectl
// might have said.
//
// Factored out as a pure function (no *testing.T, no subprocess of its own)
// specifically so it can be pinned by TestKubectlRunError_SurfacesStderr
// without needing kubectl, a context, or a cluster at all.
func kubectlRunError(action string, err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return fmt.Sprintf("%s: %v: %s", action, err, exitErr.Stderr)
	}
	return fmt.Sprintf("%s: %v", action, err)
}

// kubernetesNodeCapacity returns the first node's capacity map, read through
// kubectl against the context k3d-up.sh created. The suite has no Kubernetes
// client library and adding one for a single map would be a large dependency
// for a small need; kubectl is already a hard requirement of the Kubernetes
// leg (k3d-up.sh, k3d-down.sh and lib.sh's fetch_k8s_ca all shell out to it).
func kubernetesNodeCapacity(ctx context.Context, t *testing.T) map[string]string {
	t.Helper()
	cluster := os.Getenv("E2E_K3D_CLUSTER")
	if cluster == "" {
		cluster = "portainer-mcp-e2e"
	}
	out, err := exec.CommandContext(ctx, "kubectl", "--context", "k3d-"+cluster,
		"get", "node", "-o", "jsonpath={.items[0].status.capacity}").Output()
	if err != nil {
		t.Fatal(kubectlRunError("reading node capacity through kubectl", err))
	}
	var capacity map[string]string
	if err := json.Unmarshal(out, &capacity); err != nil {
		t.Fatalf("decoding node capacity %q: %v", out, err)
	}
	return capacity
}

// TestKubectlRunError_SurfacesStderr proves the stderr fix directly, with no
// kubectl, no context, and no cluster involved: a real subprocess (any shell
// exits non-zero after writing to stderr) reproduces exec.Cmd.Output()'s own
// failure shape — an *exec.ExitError whose bare Error() text is only "exit
// status 1", with the actually-useful line sitting unread in its Stderr
// field. This is the exact gap a live review found: kubernetesNodeCapacity's
// original "%v"-only message would report nothing beyond "exit status 1" for
// a wrong E2E_K3D_CLUSTER or a stale context, indistinguishable from any
// other kubectl failure.
func TestKubectlRunError_SurfacesStderr(t *testing.T) {
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), "sh", "-c", "echo custom-diagnostic-text >&2; exit 1")
	_, err := cmd.Output()
	if err == nil {
		t.Fatal("test fixture command unexpectedly succeeded")
	}
	got := kubectlRunError("reading node capacity through kubectl", err)
	if !strings.Contains(got, "custom-diagnostic-text") {
		t.Errorf("kubectlRunError(...) = %q, want it to contain the process's own stderr %q",
			got, "custom-diagnostic-text")
	}
	if !strings.Contains(got, "exit status 1") {
		t.Errorf("kubectlRunError(...) = %q, want it to still contain the underlying error too", got)
	}
}
