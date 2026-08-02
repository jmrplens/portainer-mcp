//go:build e2e

package suite

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// runID identifies this test process, so names minted by two concurrent `go
// test -tags e2e` invocations (or two runs in quick succession) never
// collide even when both pick the same prefix and counter value. It is
// derived once, from the wall clock and the process id, rather than per
// call: uniqueName only needs to be unique within a run, and computing it
// once keeps every name built from it cheap.
var runID = fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())

// nameCounter makes uniqueName unique per call within this run, on top of
// runID making it unique across runs. It is an atomic.Uint64 rather than a
// mutex-guarded int because fixtures are created concurrently across
// parallel subtests, and a counter is the cheapest thing that stays correct
// under that.
var nameCounter atomic.Uint64

// uniqueName returns a name no other call in this process has returned or
// will return, carrying the e2e- prefix that cleanupOrphans keys on to find
// and remove resources left behind by a run that died before its own
// cleanup ran.
func uniqueName(prefix string) string {
	n := nameCounter.Add(1)
	return fmt.Sprintf("e2e-%s-%s-%d", prefix, runID, n)
}

func TestUniqueName_IsUniquePerCallAndScopedToTheRun(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		name := uniqueName("tag")
		if seen[name] {
			t.Fatalf("uniqueName produced %q twice", name)
		}
		seen[name] = true
		if !strings.HasPrefix(name, "e2e-tag-") {
			t.Fatalf("name %q lacks the e2e- prefix that orphan cleanup keys on", name)
		}
	}
}
