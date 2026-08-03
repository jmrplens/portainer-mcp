//go:build e2e

package suite

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	var srv harness.Server
	switch ed {
	case "CE":
		srv = estate.CE
	case "EE":
		srv = estate.EE
	default:
		return nil, fmt.Errorf("rawClientFor: unknown edition %q, want CE or EE", ed)
	}
	if srv.BaseURL == "" {
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

// isHarnessProvisioned reports whether srv carries the fixed administrator
// credentials harness.Provision writes into every ephemeral estate this
// harness stands up (see provision.go's AdminUsername/AdminPassword).
//
// It is the one signal cleanupOrphans is allowed to act on before deleting
// anything: an estate's URL and API key are just data in a JSON file, and
// nothing stops that file from being pointed, by mistake or by hand, at a
// real Portainer instead of a disposable one. A real instance's
// administrator password is not this fixed, published string — a
// disposable estate's always is, because Provision always sets it. Anything
// else is treated as "cannot confirm this is ours" and refused, not deleted.
func isHarnessProvisioned(srv harness.Server) bool {
	return srv.Creds.Username == harness.AdminUsername && srv.Creds.Password == harness.AdminPassword
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

// cleanupOrphans deletes every tag and registry, on every provisioned leg of
// e, whose name starts with orphanPrefix and is old enough per
// isOrphanEligible. It is the net for a run that died between creating a
// resource and its own cleanup running: nothing else distinguishes one
// test's tag from another's on a server every session in the matrix shares,
// so a run that dies mid-test leaves its fixtures behind for the next run to
// trip over unless something like this sweeps first — and the age check is
// what keeps that same sweep from also devouring a concurrently running
// sibling's fixtures.
//
// It refuses to touch a leg that does not pass isHarnessProvisioned,
// returning an error instead of silently skipping it: skipping would look
// identical to "nothing to clean up" from the caller's side, and the whole
// point of the guard is to fail loudly rather than delete when unsure.
func cleanupOrphans(ctx context.Context, e harness.Estate) error {
	legs := map[string]harness.Server{"CE": e.CE}
	if e.HasBusinessEdition() {
		legs["EE"] = e.EE
	}

	now := time.Now()
	for name, srv := range legs {
		if !isHarnessProvisioned(srv) {
			return fmt.Errorf("cleanupOrphans: %s server %s does not carry this harness's fixed "+
				"provisioning credentials; refusing to delete e2e- resources against a server this "+
				"harness cannot confirm it provisioned", name, srv.BaseURL)
		}

		client, err := portainer.New(&config.Config{URL: srv.BaseURL, Token: srv.Creds.APIKey})
		if err != nil {
			return fmt.Errorf("cleanupOrphans: build %s client: %w", name, err)
		}
		if err := deleteOrphanTags(ctx, client, now); err != nil {
			return fmt.Errorf("cleanupOrphans: %s: %w", name, err)
		}
		if err := deleteOrphanRegistries(ctx, client, now); err != nil {
			return fmt.Errorf("cleanupOrphans: %s: %w", name, err)
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

func TestIsHarnessProvisioned_RequiresExactAdminCredentials(t *testing.T) {
	base := harness.Server{BaseURL: "https://example.invalid"}

	tests := []struct {
		name string
		srv  harness.Server
		want bool
	}{
		{
			name: "exact harness credentials",
			srv: func() harness.Server {
				s := base
				s.Creds.Username = harness.AdminUsername
				s.Creds.Password = harness.AdminPassword
				return s
			}(),
			want: true,
		},
		{
			name: "wrong password",
			srv: func() harness.Server {
				s := base
				s.Creds.Username = harness.AdminUsername
				s.Creds.Password = "hunter2"
				return s
			}(),
			want: false,
		},
		{
			name: "wrong username",
			srv: func() harness.Server {
				s := base
				s.Creds.Username = "root"
				s.Creds.Password = harness.AdminPassword
				return s
			}(),
			want: false,
		},
		{
			name: "empty credentials",
			srv:  base,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isHarnessProvisioned(tt.srv); got != tt.want {
				t.Errorf("isHarnessProvisioned() = %v, want %v", got, tt.want)
			}
		})
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

func TestCleanupOrphans_RefusesAnEstateWithoutHarnessCredentials(t *testing.T) {
	t.Parallel()
	// BaseURL points nowhere reachable on purpose: if the credential guard
	// were bypassed, cleanupOrphans would still fail here, but for an
	// unrelated reason (a network error dialing an invalid host) that would
	// let this test pass even with the guard broken. Asserting on the
	// guard's own wording, rather than merely "err != nil", is what makes
	// this test fail if the guard itself stops firing.
	e := harness.Estate{CE: harness.Server{BaseURL: "https://not-ours.example.invalid"}}
	err := cleanupOrphans(context.Background(), e)
	if err == nil {
		t.Fatal("cleanupOrphans accepted an estate whose CE credentials do not match this harness's provisioning")
	}
	const wantSubstring = "cannot confirm it provisioned"
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("cleanupOrphans error = %q, want it to contain %q (the credential guard's own reason, "+
			"not a network failure reaching an unreachable host)", err.Error(), wantSubstring)
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
