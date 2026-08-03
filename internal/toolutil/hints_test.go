package toolutil

import (
	"strings"
	"testing"
)

func TestWriteHints_RoundTripsThroughTheMarkdownMarker(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	WriteHints(&b, "Use tags.list to see the result")

	got := ExtractHints(b.String())
	if len(got) != 1 || got[0] != "Use tags.list to see the result" {
		t.Errorf("ExtractHints() = %v, want the hint written by WriteHints", got)
	}
	// A body with no hints must yield none rather than an empty string entry,
	// which would serialise as a blank next step.
	if h := ExtractHints("no hints here"); len(h) != 0 {
		t.Errorf("ExtractHints() = %v, want none", h)
	}
}
