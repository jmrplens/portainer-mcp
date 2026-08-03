package main

import (
	"os"
	"strings"
	"testing"
)

// TestRun_AppliesTheStartupDeadline guards against the deadline being declared
// and never wired, which is the state a past fix corrected.
//
// This is an interim source-level guard, not a behavioral test: it greps
// main.go for the identifier portainer.DefaultCallTimeout, so it passes if
// that identifier merely appears in a comment, and it would fail to catch the
// same deadline being applied through a differently named constant or moved
// to another file. internal/wiring's BuildCatalog tests already prove a
// context deadline bounds detection at that level; a behavioral version of
// this test would need the startup sequence in run extracted so it can be
// called with a stub clock or a short deadline, which is out of scope here
// and belongs to P3, once run is restructured for injection.
func TestRun_AppliesTheStartupDeadline(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(source), "portainer.DefaultCallTimeout") {
		t.Error("run does not apply portainer.DefaultCallTimeout to the startup detection; an unresponsive server would hang startup indefinitely")
	}
}
