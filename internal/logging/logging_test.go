package logging

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestNewTo_InfoLevel_WritesRecord(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	NewTo(&buf, slog.LevelInfo).Info("hello", "key", "value")

	got := buf.String()
	if !strings.Contains(got, "hello") || !strings.Contains(got, "key=value") {
		t.Errorf("output = %q, want it to contain the message and the attribute", got)
	}
}

func TestNewTo_BelowLevel_WritesNothing(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	NewTo(&buf, slog.LevelWarn).Info("suppressed")

	if buf.Len() != 0 {
		t.Errorf("output = %q, want empty because Info is below Warn", buf.String())
	}
}

// TestDefaultWriter_IsStderr is the guardrail for the hardest constraint in
// this project: stdout carries the MCP JSON-RPC stream, so a single stray
// write corrupts the protocol.
func TestDefaultWriter_IsStderr(t *testing.T) {
	t.Parallel()
	if defaultWriter != os.Stderr {
		t.Error("the default logger destination must be stderr; stdout carries the MCP transport")
	}
}
