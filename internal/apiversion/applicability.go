// Package apiversion answers whether a given Portainer API operation exists on
// a given server version.
//
// Portainer maintains two release channels at once — LTS (2.39.x) and STS
// (2.4x) — so "supported version" cannot be a single number. The applicability
// table is derived from all 28 published specifications per edition, which is
// why it lives in generated code rather than a hand-maintained constant.
package apiversion

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// Operation identifies one API operation.
type Operation struct {
	Method string
	Path   string
}

// Span is one contiguous run of versions in which an operation exists. An
// empty MaxVersion means the operation is still present in the newest
// published specification.
type Span struct {
	MinVersion string
	MaxVersion string
}

// Applicability returns every span in which an operation exists.
//
// Almost every operation has exactly one span. A handful have two, because
// Portainer withdrew them and later reintroduced them — /cloud/gitcredentials
// is absent in 2.43.0 and back in 2.44.0. Collapsing those to a single
// first-to-last range would claim the operation exists on a version where the
// route returns 404, so the gap is represented rather than smoothed away.
func Applicability(e edition.Edition, op Operation) ([]Span, bool) {
	byOperation, ok := spans[e]
	if !ok {
		return nil, false
	}
	found, ok := byOperation[op]
	return found, ok
}

// Available reports whether an operation exists on the given server version.
//
// An operation absent from the table is unavailable: the table is generated
// from every published specification, so absence means Portainer never
// documented it. An unparseable server version falls back to available — a
// version string we cannot read is our problem, and hiding the whole API
// because of it would be worse than attempting a call that may fail.
func Available(e edition.Edition, op Operation, serverVersion string) bool {
	found, ok := Applicability(e, op)
	if !ok {
		return false
	}
	for _, span := range found {
		if withinSpan(span, serverVersion) {
			return true
		}
	}
	return false
}

// withinSpan reports whether serverVersion falls inside span, treating an
// unreadable version as inside.
func withinSpan(span Span, serverVersion string) bool {
	if span.MinVersion != "" {
		cmp, err := compareVersions(serverVersion, span.MinVersion)
		if err != nil {
			return true
		}
		if cmp < 0 {
			return false
		}
	}
	if span.MaxVersion != "" {
		cmp, err := compareVersions(serverVersion, span.MaxVersion)
		if err != nil {
			return true
		}
		if cmp > 0 {
			return false
		}
	}
	return true
}

// compareVersions orders two dotted numeric versions, comparing each component
// numerically so that 2.39.10 sorts above 2.39.9.
func compareVersions(a, b string) (int, error) {
	partsA, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	partsB, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < len(partsA) || i < len(partsB); i++ {
		var x, y int
		if i < len(partsA) {
			x = partsA[i]
		}
		if i < len(partsB) {
			y = partsB[i]
		}
		if x != y {
			if x < y {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(v string) ([]int, error) {
	fields := strings.Split(strings.TrimSpace(v), ".")
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("parse version %q: %w", v, err)
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("parse version %q: empty", v)
	}
	return parts, nil
}
