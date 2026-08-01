// Command portainer-mcp is an MCP server exposing the Portainer REST API.
//
// Configuration is resolved from a .env file, the process environment and CLI
// flags, in that order of increasing precedence. The server speaks JSON-RPC
// over standard input and output, so nothing but the transport may write to
// standard output.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
	"github.com/jmrplens/portainer-mcp/internal/version"
)

const projectRepository = "https://github.com/jmrplens/portainer-mcp"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "portainer-mcp: %v\n", err)
		os.Exit(1)
	}
}

// run parses flags, resolves configuration and serves the MCP server until the
// context is cancelled. It returns an error rather than exiting so that it
// stays testable.
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
		return fmt.Errorf("parse flags: %w", err)
	}

	if *showVersion {
		fmt.Fprintln(os.Stderr, version.String())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
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
		return err
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

	if err := newServer(cfg).Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("serve stdio: %w", err)
	}
	return nil
}

// newServer builds the MCP server for the given configuration.
func newServer(cfg *config.Config) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:       "portainer-mcp",
		Title:      "Portainer MCP Server",
		Version:    version.Version,
		WebsiteURL: projectRepository,
	}, &mcp.ServerOptions{
		Instructions: "portainer-mcp exposes the Portainer REST API as MCP tools for managing " +
			"container environments, stacks, registries, users, teams and edge deployments.\n\n" +
			"Call portainer_mcp_status to discover the active tool surface and whether read-only " +
			"or safe mode is enabled before attempting any mutating operation.",
	})
	addStatusTool(server, cfg)
	return server
}
