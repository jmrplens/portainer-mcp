//go:build e2e

package suite

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// fixtureClient is the single Portainer client every fixture helper talks
// through, built once against the Community Edition leg.
//
// Community Edition, specifically, not "whichever edition the calling test
// happens to be exercising": tags and registries are server-wide resources,
// there is one Community Edition server in every estate this harness
// provisions, and ReadOnly/SafeMode sessions above are already built against
// it for the identical reason — it is the leg guaranteed present regardless
// of licensing. A fixture built against it is reachable from every session
// in the matrix, since the tool call under test talks to whichever server
// backs the session, not to this client.
var (
	fixtureClientOnce sync.Once
	fixtureClientInst *portainer.Client
	fixtureClientErr  error
)

func fixtureClient(t *testing.T) *portainer.Client {
	t.Helper()
	fixtureClientOnce.Do(func() {
		fixtureClientInst, fixtureClientErr = portainer.New(&config.Config{
			URL:   estate.CE.BaseURL,
			Token: estate.CE.Creds.APIKey,
		})
	})
	if fixtureClientErr != nil {
		t.Fatalf("build fixture client: %v", fixtureClientErr)
	}
	return fixtureClientInst
}

// createTag creates an environment tag named name and returns its id. The
// caller is expected to have built name with uniqueName so orphan cleanup
// can find it if this test's own cleanup never runs.
func createTag(t *testing.T, name string) int {
	t.Helper()
	client := fixtureClient(t)

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
		resp, err := client.API.TagDeleteWithResponse(ctx, id)
		if err != nil {
			return err
		}
		return toolutil.Check(resp)
	})
	return id
}

// createRegistry creates a custom-type registry named name and returns its
// id. Like createTag, the caller is expected to have built name with
// uniqueName.
//
// It uses the generic "custom registry" type rather than a real registry
// kind (Docker Hub, ECR, ...) because nothing here ever pulls an image
// through it: the fixture only needs a registry record to exist so a suite
// can exercise list/inspect/update/delete against it, and a custom registry
// is the one type that stores whatever URL it is given without validating
// that anything is listening there.
func createRegistry(t *testing.T, name string) int {
	t.Helper()
	client := fixtureClient(t)

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
		resp, err := client.API.RegistryDeleteWithResponse(ctx, id)
		if err != nil {
			return err
		}
		return toolutil.Check(resp)
	})
	return id
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

// cleanupOrphans deletes every tag and registry, on every provisioned leg of
// e, whose name starts with orphanPrefix. It is the net for a run that died
// between creating a resource and its own cleanup running: nothing else
// distinguishes one test's tag from another's on a server every session in
// the matrix shares, so a run that dies mid-test leaves its fixtures behind
// for the next run to trip over unless something like this sweeps first.
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
		if err := deleteOrphanTags(ctx, client); err != nil {
			return fmt.Errorf("cleanupOrphans: %s: %w", name, err)
		}
		if err := deleteOrphanRegistries(ctx, client); err != nil {
			return fmt.Errorf("cleanupOrphans: %s: %w", name, err)
		}
	}
	return nil
}

func deleteOrphanTags(ctx context.Context, client *portainer.Client) error {
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

func deleteOrphanRegistries(ctx context.Context, client *portainer.Client) error {
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
