# Contributing to Portainer MCP Server

Thank you for your interest in contributing! Whether you're fixing a bug, adding a feature, or improving documentation, this guide covers everything you need to know.

> 📖 **Full developer documentation** is available at [portainer.github.io/portainer-mcp/development/](https://portainer.github.io/portainer-mcp/development/contributing/)

## Quick Start

```bash
# Clone and build
git clone https://github.com/portainer/portainer-mcp.git
cd portainer-mcp
make build          # → dist/portainer-mcp

# Run tests
make test           # Unit tests
make test-all       # Unit + integration (requires Docker)

# Format and lint
gofmt -s -w .
go vet ./...
```

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/doc/install) | 1.24+ | Build and test |
| [Make](https://www.gnu.org/software/make/) | Any | Build automation |
| [Docker](https://docs.docker.com/get-docker/) | 20+ | Integration tests |
| [pnpm](https://pnpm.io/installation) | 9+ | Documentation site (optional) |
| [Portainer](https://www.portainer.io/) | 2.31.2 | Live testing (optional) |

## Project Structure

```
portainer-mcp/
├── cmd/portainer-mcp/         # CLI entry point (flags, startup)
├── internal/
│   ├── mcp/                   # MCP server core
│   │   ├── server.go          #   Server struct, interfaces, registration
│   │   ├── schema.go          #   Tool name constants
│   │   ├── metatool_*.go      #   Meta-tool infrastructure
│   │   ├── utils.go           #   Shared handler utilities
│   │   └── <domain>.go        #   Handler files per domain
│   ├── tooldef/               # Embedded tool definitions
│   └── k8sutil/               # Kubernetes response stripping
├── pkg/
│   ├── portainer/
│   │   ├── client/            # Portainer API wrapper client
│   │   └── models/            # Local models + conversion functions
│   └── toolgen/               # YAML tool loader + parameter parsing
├── tests/integration/         # Integration tests with real Portainer
├── tools.yaml                 # Tool definitions (embedded at build)
├── docs/                      # Starlight documentation site
└── .goreleaser.yaml           # Release automation
```

## How to Contribute

### Reporting Bugs

- Use [GitHub Issues](https://github.com/portainer/portainer-mcp/issues/new) with the **bug** label
- Include: Go version, Portainer version, steps to reproduce, expected vs actual behavior

### Suggesting Features

- Open an issue with the **enhancement** label
- Describe the use case and proposed approach

### Submitting Code

1. **Fork** the repository
2. **Branch** from `main` using a descriptive name: `feat/helm-rollback`, `fix/proxy-timeout`
3. **Implement** with tests — see the [developer guide](https://portainer.github.io/portainer-mcp/development/contributing/) for patterns
4. **Verify** — all checks must pass:
   ```bash
   go build ./...
   go vet ./...
   gofmt -d .       # Should produce no output
   make test-all
   ```
5. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/):
   ```
   feat: add Helm rollback support
   fix: correct proxy timeout for large responses
   docs: update architecture diagram
   test: add edge job integration tests
   refactor: simplify parameter validation
   ```
6. **Open a Pull Request** with a clear title and description

### PR Checklist

- [ ] All tests pass (`make test-all`)
- [ ] Code is formatted (`gofmt -s -w .`) and lint-clean (`go vet ./...`)
- [ ] New tools defined in `tools.yaml` with correct `parameters:` key
- [ ] Handlers wrap errors with context (`fmt.Errorf("failed to X: %w", err)`)
- [ ] Read-only annotations (`readOnlyHint`) set correctly
- [ ] New features have unit tests; breaking changes have integration tests
- [ ] Documentation updated for user-facing changes

## Key Conventions

| Area | Convention |
|------|-----------|
| Naming | `PascalCase` exported, `camelCase` private |
| Errors | `fmt.Errorf("context: %w", err)` — always wrap with context |
| Imports | Standard lib → external → internal (blank line separated) |
| Tests | Table-driven with descriptive case names |
| Models | Raw → Local conversion; never expose raw models to handlers |
| Logging | `zerolog` to stderr; never log to stdout (MCP transport) |

## Design Decisions

Architectural decisions are documented in `docs/design/`. Before making significant changes:

1. Review existing decisions in `docs/design_summary.md`
2. Create a new record if introducing an architectural change: `YYMMDD-N-short-description.md`

## Documentation

The documentation site uses [Starlight](https://starlight.astro.build/) (Astro):

```bash
cd docs
pnpm install
pnpm run dev      # Local development at localhost:4321
pnpm run build    # Production build
```

## Security

Please report vulnerabilities privately — see [SECURITY.md](SECURITY.md) for details.

## Code of Conduct

Be respectful, constructive, and inclusive. We follow the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).

## Questions?

Open an [issue](https://github.com/portainer/portainer-mcp/issues) or [discussion](https://github.com/portainer/portainer-mcp/discussions). We're happy to help!
