//go:build e2e

// Package suite drives every tool surface and edition against a live,
// provisioned Portainer estate over the MCP protocol.
//
// Every file here carries the e2e build tag and is excluded from `go test
// ./...` and `make test`: it needs Docker, a running estate (see
// `make e2e-up`), and real HTTP round trips, none of which belong in the fast,
// Docker-free default test run.
package suite

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

var (
	estate   harness.Estate
	sessions *Sessions
)

// TestMain loads the estate and builds every session before any test runs,
// and fails the whole run rather than skipping when no estate is present.
//
// Running with -tags e2e is an explicit request for these tests: silently
// passing an unprovisioned run is exactly the decorative-suite failure this
// phase exists to prevent. A missing Community Edition leg is that case; a
// missing Business Edition leg is not (see Sessions.For), because a
// contributor without the licence must still be able to run the Community
// Edition suites.
func TestMain(m *testing.M) {
	path := os.Getenv(harness.EstateFileEnv)
	if path == "" {
		// harness.LoadEstate rejects a relative path that climbs above its
		// starting directory (rejectEscapingPath), by design: it exists to
		// reject a path escaping a sandbox, not to bless every ".." a caller
		// might write. Resolving the default to an absolute path here, rather
		// than passing the relative "../.estate.json" some readers might
		// expect, keeps this default on the same "absolute path, trusted as
		// given" footing that LoadEstate's own doc comment says every caller
		// in this codebase uses.
		abs, err := filepath.Abs("../.estate.json")
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e: resolve default estate path: %v\n", err)
			os.Exit(1)
		}
		path = abs
	}
	var err error
	estate, err = harness.LoadEstate(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\nrun `make e2e-up` first\n", err)
		os.Exit(1)
	}
	sessions, err = newSessions(estate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build sessions: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	sessions.Close()
	os.Exit(code)
}
