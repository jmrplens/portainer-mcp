// Package logging builds the application logger.
//
// The logger writes to standard error, never standard output: in stdio mode
// standard output carries the MCP JSON-RPC stream, and any other write to it
// corrupts the protocol.
package logging

import (
	"io"
	"log/slog"
	"os"
)

// defaultWriter is the destination New uses. It is a package variable rather
// than an inline os.Stderr so that the guardrail test can assert the choice
// without reaching into slog handler internals.
var defaultWriter io.Writer = os.Stderr

// New returns a logger that writes to standard error at the given level.
func New(level slog.Level) *slog.Logger {
	return NewTo(defaultWriter, level)
}

// NewTo returns a logger that writes to w at the given level.
func NewTo(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
