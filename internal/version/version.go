// Package version exposes build metadata injected at link time.
//
// The values are set with -ldflags "-X github.com/jmrplens/portainer-mcp/internal/version.Version=..."
// and fall back to placeholders in development builds.
package version

import "fmt"

// Build metadata, overridden at link time by the release pipeline.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String renders the build metadata as "<version> (<commit>, <date>)".
func String() string {
	return fmt.Sprintf("%s (%s, %s)", Version, Commit, BuildDate)
}
