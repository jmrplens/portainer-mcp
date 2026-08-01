package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestBinary_NonTransportPaths_WriteNothingToStdout builds the real binary
// and runs it through every code path that is not the MCP stdio transport,
// asserting stdout receives zero bytes in each. Standard output carries the
// JSON-RPC stream once the server starts serving; any stray write anywhere
// else in the process (a flag error, a config failure, a version print)
// would corrupt that stream for a real client, so this is a black-box
// regression test for the project's hardest constraint rather than something
// a unit test on individual functions can catch.
func TestBinary_NonTransportPaths_WriteNothingToStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary build/run test in -short mode")
	}

	binary := filepath.Join(t.TempDir(), "portainer-mcp-under-test")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	build := exec.CommandContext(context.Background(), "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	tests := []struct {
		name string
		args []string
		env  []string
	}{
		{
			name: "version flag",
			args: []string{"-version"},
			env:  []string{},
		},
		{
			name: "unknown flag",
			args: []string{"-nonexistent"},
			env:  []string{},
		},
		{
			name: "startup config failure (no URL or token)",
			args: []string{},
			env:  []string{},
		},
		{
			name: "normal stdio start with closed stdin",
			args: []string{},
			env: []string{
				"PORTAINER_URL=https://example.com",
				"PORTAINER_TOKEN=ptr_x",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.CommandContext(context.Background(), binary, tt.args...)
			// Set the environment explicitly (not append to os.Environ())
			// so the developer's ambient shell — which may export
			// PORTAINER_URL, PORTAINER_TOKEN or any variable a later phase
			// adds — cannot influence which path the binary takes.
			cmd.Env = tt.env

			// Closed stdin: the binary must not block waiting for input on
			// any of these paths, including the stdio-start path, whose
			// transport reads from stdin until EOF.
			cmd.Stdin = bytes.NewReader(nil)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			_ = cmd.Run() // exit code is irrelevant here; only stdout purity is asserted

			if stdout.Len() != 0 {
				t.Errorf("mode %q wrote to stdout, want empty; got %q (stderr was %q)",
					tt.name, stdout.String(), stderr.String())
			}
		})
	}
}
