// Command portainer-mcp is an MCP server exposing the Portainer REST API.
//
// Configuration is resolved from a .env file, the process environment and CLI
// flags, in that order of increasing precedence. The server speaks JSON-RPC
// over standard input and output, so nothing but the transport may write to
// standard output.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/version"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "portainer-mcp: %v\n", err)
		os.Exit(1)
	}
}

// run parses flags, resolves configuration and serves the MCP server until the
// context is cancelled. It returns an error rather than exiting so that it
// stays testable.
//
// The catalog and server are built through internal/wiring rather than
// locally: that package is also what the e2e harness calls to build its
// in-memory servers, so both share the exact same construction path and a
// harness built a different way cannot hide a defect that only the real
// wiring has.
func run(args []string) error {
	fs := flag.NewFlagSet("portainer-mcp", flag.ContinueOnError)
	var (
		showVersion   = fs.Bool("version", false, "print the version and exit")
		serverURL     = fs.String("server", "", "Portainer server URL (overrides PORTAINER_URL)")
		token         = fs.String("token", "", "Portainer API token (overrides PORTAINER_TOKEN)")
		skipTLSVerify = fs.Bool("skip-tls-verify", false, "skip TLS certificate verification")
		toolSurface   = fs.String("tool-surface", "", "tool surface: dynamic, meta or individual")
		readOnly      = fs.Bool("read-only", false, "disable every mutating tool")
		safeMode      = fs.Bool("safe-mode", false, "intercept mutating tools and return a preview")
	)
	if err := fs.Parse(args); err != nil {
		// flag already printed usage to stderr for -h/-help; that is a
		// successful, expected exit, not a failure worth an error prefix.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse flags: %w", err)
	}

	if *showVersion {
		fmt.Fprintln(os.Stderr, version.String())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// Flags win over the environment, but only when explicitly provided:
	// applying a flag's zero value unconditionally would erase env settings.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "server":
			cfg.URL = *serverURL
		case "token":
			cfg.Token = *token
		case "skip-tls-verify":
			cfg.SkipTLSVerify = *skipTLSVerify
		case "tool-surface":
			cfg.ToolSurface = config.ToolSurface(*toolSurface)
		case "read-only":
			cfg.ReadOnly = *readOnly
		case "safe-mode":
			cfg.SafeMode = *safeMode
		}
	})

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}

	logger := logging.New(cfg.LogLevel)
	logger.Info("starting portainer-mcp",
		"version", version.String(),
		"url", cfg.URL,
		"surface", cfg.ToolSurface,
		"read_only", cfg.ReadOnly,
		"safe_mode", cfg.SafeMode,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := portainer.New(cfg)
	if err != nil {
		return fmt.Errorf("build portainer client: %w", err)
	}

	// Bound the startup round trip. The client sets no http.Client.Timeout on
	// purpose — that would cap long operations such as a stack redeploy that
	// pulls multi-gigabyte images — so the deadline belongs on the context, and
	// DefaultCallTimeout exists for exactly this. Without it an MCP host that
	// starts this server before the network is up gets a silent, hung process.
	detectCtx, cancelDetect := context.WithTimeout(ctx, portainer.DefaultCallTimeout)
	defer cancelDetect()

	catalog, resolvedEdition, serverVersion, err := wiring.BuildCatalog(detectCtx, cfg, client, logger)
	if err != nil {
		return fmt.Errorf("build action catalog: %w", err)
	}
	logger.Info("action catalog built",
		"edition", resolvedEdition,
		"server_version", serverVersion,
		"action_count", len(catalog.Actions()),
	)

	deps := tools.Deps{Client: client, Logger: logger, SafeMode: cfg.SafeMode}

	srv, err := wiring.NewServer(cfg, catalog, deps, resolvedEdition, serverVersion)
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("serve stdio: %w", err)
	}
	return nil
}
