//go:build e2e

package suite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// ledgerCleanupTimeout bounds CleanupAll's teardown calls, exactly like the
// create path's portainer.DefaultCallTimeout bounds fixture creation. Without
// it, an estate that stops responding during teardown blocks the whole test
// binary until go test's own timeout, and the resulting panic dump hides the
// real cause behind a wall of goroutine stacks.
const ledgerCleanupTimeout = portainer.DefaultCallTimeout

// ledgerEntry is one resource a ResourceLedger has been told to clean up.
type ledgerEntry struct {
	kind    string
	id      string
	cleanup func(context.Context) error
}

// ResourceLedger records every fixture a test created, in creation order, so
// they can be torn down in the reverse order when the test ends.
//
// Reverse order matters for exactly the reason a stack unwinds that way: a
// resource created inside a group (a tag assigned to an endpoint, say) must
// be gone before the group itself is removed, and the ledger is the only
// thing that knows the order they were created in. It is safe for
// concurrent use because fixtures are created from parallel subtests.
type ResourceLedger struct {
	mu      sync.Mutex
	entries []ledgerEntry
}

// Register records one resource and the function that removes it. kind and
// id are carried only for error messages; nothing else consults them.
func (l *ResourceLedger) Register(kind, id string, cleanup func(context.Context) error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, ledgerEntry{kind: kind, id: id, cleanup: cleanup})
}

// CleanupAll runs every registered cleanup in reverse registration order and
// returns every error encountered rather than stopping at the first: a
// resource that is already gone (deleted by an earlier, broader cleanup, for
// instance) must not strand the ones that were created after it.
func (l *ResourceLedger) CleanupAll(ctx context.Context) []error {
	l.mu.Lock()
	entries := l.entries
	l.entries = nil
	l.mu.Unlock()

	var errs []error
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if err := e.cleanup(ctx); err != nil {
			errs = append(errs, fmt.Errorf("clean up %s %s: %w", e.kind, e.id, err))
		}
	}
	return errs
}

// testLedgers memoizes the ResourceLedger built for each *testing.T, so
// every createTag/createRegistry call within one test registers against the
// same ledger instead of each fixture managing its own, unobservable
// t.Cleanup order.
var (
	testLedgersMu sync.Mutex
	testLedgers   = map[*testing.T]*ResourceLedger{}
)

// newLedger returns the ResourceLedger for t, creating it on first use and
// arranging for CleanupAll to run once, automatically, when t ends. A
// fixture helper (createTag, createRegistry) calls this itself and registers
// against the result, so a test that only ever calls those helpers never has
// to touch a ledger, or even a t.Cleanup, directly — and so never has the
// chance to create a resource without registering its cleanup.
func newLedger(t *testing.T) *ResourceLedger {
	t.Helper()

	testLedgersMu.Lock()
	defer testLedgersMu.Unlock()

	if l, ok := testLedgers[t]; ok {
		return l
	}

	l := &ResourceLedger{}
	testLedgers[t] = l
	t.Cleanup(func() {
		testLedgersMu.Lock()
		delete(testLedgers, t)
		testLedgersMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), ledgerCleanupTimeout)
		defer cancel()
		for _, err := range l.CleanupAll(ctx) {
			t.Errorf("fixture cleanup: %v", err)
		}
	})
	return l
}

func TestLedger_CleansUpInReverseOrder(t *testing.T) {
	t.Parallel()
	// Reverse order matters: a registry inside a group must be deleted before
	// the group, and the ledger is the only thing that knows the order they
	// were created in.
	//
	// Exercised directly against the ledger rather than through t.Cleanup: a
	// t.Cleanup func's execution order is real, but this test has no way to
	// observe it from inside the test that registered it.
	var order []string
	l := &ResourceLedger{}
	l.Register("a", "1", func(context.Context) error { order = append(order, "a"); return nil })
	l.Register("b", "2", func(context.Context) error { order = append(order, "b"); return nil })
	l.CleanupAll(context.Background())
	if !reflect.DeepEqual(order, []string{"b", "a"}) {
		t.Errorf("cleanup order = %v, want [b a]", order)
	}
}

func TestLedger_OneFailureDoesNotStopTheRest(t *testing.T) {
	t.Parallel()
	// A resource that is already gone must not strand the ones after it.
	var cleaned int
	l := &ResourceLedger{}
	l.Register("a", "1", func(context.Context) error { cleaned++; return nil })
	l.Register("b", "2", func(context.Context) error { return errors.New("already deleted") })
	l.Register("c", "3", func(context.Context) error { cleaned++; return nil })
	errs := l.CleanupAll(context.Background())
	if cleaned != 2 {
		t.Errorf("cleaned %d resources, want 2: a failure aborted the rest", cleaned)
	}
	if len(errs) != 1 {
		t.Errorf("errors = %v, want exactly the one failure reported", errs)
	}
}

func TestNewLedger_ReturnsTheSameLedgerForTheSameTest(t *testing.T) {
	t.Parallel()
	a := newLedger(t)
	b := newLedger(t)
	if a != b {
		t.Fatal("newLedger returned a different ledger for the same *testing.T")
	}
}

func TestNewLedger_RunsCleanupWhenTheTestEnds(t *testing.T) {
	t.Parallel()
	var cleaned bool
	t.Run("inner", func(t *testing.T) {
		l := newLedger(t)
		l.Register("kind", "id", func(context.Context) error { cleaned = true; return nil })
	})
	if !cleaned {
		t.Error("newLedger's registered cleanup did not run when the inner test ended")
	}
}
