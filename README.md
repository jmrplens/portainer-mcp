<div align="center">

# Portainer MCP Server

**Manage your entire Portainer infrastructure through AI assistants using the Model Context Protocol**

[![Go Report Card](https://goreportcard.com/badge/github.com/portainer/portainer-mcp)](https://goreportcard.com/report/github.com/portainer/portainer-mcp)
[![codecov](https://codecov.io/gh/portainer/portainer-mcp/graph/badge.svg?token=NHTQ5FIPFX)](https://codecov.io/gh/portainer/portainer-mcp)
![Go Version](https://img.shields.io/github/go-mod/go-version/portainer/portainer-mcp)
![License](https://img.shields.io/github/license/portainer/portainer-mcp)
![Portainer](https://img.shields.io/badge/Portainer-2.31.2-blue)
![MCP Tools](https://img.shields.io/badge/MCP_Tools-98-green)

[Documentation](https://portainer.github.io/portainer-mcp/) · [Quickstart](#quickstart) · [Configuration](#configuration) · [Contributing](CONTRIBUTING.md)

</div>

---

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) server that connects AI assistants to [Portainer](https://www.portainer.io/) — exposing **98 tools** covering the complete Portainer API. Manage environments, stacks, users, teams, registries, Kubernetes, Helm, Docker, edge computing, backups, and more through natural language.

<details open>
<summary><b>🖥️ System & Docker Dashboard</b></summary>

![System & Docker Dashboard demo](docs/demo-1-system-docker.webp)
</details>

<details>
<summary><b>👥 Users, Teams & Stacks</b></summary>

![Users, Teams & Stacks demo](docs/demo-2-users-stacks.webp)
</details>

<details>
<summary><b>🌐 Edge & Kubernetes</b></summary>

![Edge & Kubernetes demo](docs/demo-3-edge-helm.webp)
</details>

<details>
<summary><b>💾 Backup & Docker Proxy</b></summary>

![Backup & Docker Proxy demo](docs/demo-4-backup-proxy.webp)
</details>

## Quickstart

### 1. Install

**Homebrew** (macOS / Linux):
```bash
brew install portainer/tap/portainer-mcp
```

**Go install**:
```bash
go install github.com/portainer/portainer-mcp/cmd/portainer-mcp@latest
```

**Docker**:
```bash
docker pull ghcr.io/portainer/portainer-mcp:latest
```

Or download a pre-built binary from [Releases](https://github.com/portainer/portainer-mcp/releases/latest) (Linux, macOS, Windows — amd64/arm64, with SHA256 checksums).

### 2. Get a Portainer API Token

1. Log in to your Portainer instance → **My Account** → **API Keys**
2. Create a new key and copy the token

### 3. Configure your AI assistant

<details open>
<summary><b>Claude Desktop</b></summary>

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "portainer": {
      "command": "/path/to/portainer-mcp",
      "args": [
        "-server", "https://your-portainer:9443",
        "-token", "ptr_your_api_token"
      ]
    }
  }
}
```
</details>

<details>
<summary><b>VS Code (GitHub Copilot)</b></summary>

Create `.vscode/mcp.json` in your workspace:

```json
{
  "servers": {
    "portainer": {
      "type": "stdio",
      "command": "/path/to/portainer-mcp",
      "args": [
        "-server", "https://your-portainer:9443",
        "-token", "ptr_your_api_token"
      ]
    }
  }
}
```
</details>

<details>
<summary><b>Cursor</b></summary>

Go to **Cursor Settings → MCP** and add:

```json
{
  "mcpServers": {
    "portainer": {
      "command": "/path/to/portainer-mcp",
      "args": [
        "-server", "https://your-portainer:9443",
        "-token", "ptr_your_api_token"
      ]
    }
  }
}
```
</details>

### 4. Start asking

> "List all environments and their status"  
> "Create a new nginx stack from this compose file"  
> "Show me the Kubernetes dashboard for environment 3"

## Configuration

| Flag | Description | Required | Default |
|------|-------------|----------|---------|
| `-server` | Portainer server URL | **Yes** | — |
| `-token` | Portainer API token | **Yes** | — |
| `-tools` | Path to custom tools.yaml | No | Embedded |
| `-read-only` | Disable all write/delete operations | No | `false` |
| `-granular-tools` | Register all 98 individual tools instead of 15 grouped meta-tools | No | `false` |
| `-disable-version-check` | Skip Portainer version validation | No | `false` |
| `-skip-tls-verify` | Skip TLS certificate verification | No | `false` |

### Meta-Tools (Default Mode)

By default the server registers **15 grouped meta-tools** instead of the 98 individual granular tools. Each meta-tool covers a functional domain and exposes an `action` parameter (enum) that routes to the appropriate handler.

This dramatically reduces the tool-selection surface for LLMs while preserving 100% of the underlying functionality.

| Meta-Tool | Actions | Description |
|-----------|---------|-------------|
| `manage_environments` | 16 | Environments, environment groups, tags |
| `manage_stacks` | 13 | Regular and compose stacks |
| `manage_access_groups` | 7 | Access group CRUD and user/team access policies |
| `manage_users` | 5 | User CRUD and role management |
| `manage_teams` | 6 | Teams and team membership |
| `manage_docker` | 2 | Docker proxy and dashboard |
| `manage_kubernetes` | 5 | Kubernetes proxy, namespaces, config, dashboard |
| `manage_helm` | 8 | Helm repos, charts, releases |
| `manage_registries` | 5 | Container registry management |
| `manage_templates` | 7 | Custom and app templates |
| `manage_backups` | 5 | Backup, restore, S3 settings |
| `manage_webhooks` | 3 | Webhook CRUD |
| `manage_edge` | 6 | Edge jobs and update schedules |
| `manage_settings` | 5 | Server settings and SSL |
| `manage_system` | 5 | Version, status, MOTD, roles, auth |

To use the original 98 individual tools, pass `--granular-tools`. See the [Meta-Tools Guide](https://portainer.github.io/portainer-mcp/guides/meta-tools/) for the full action reference.

### Read-Only Mode

Run with `-read-only` to restrict to read-only operations. All write, update, and delete actions are disabled — ideal for monitoring and observation. Works with both meta-tools and granular tools modes.

### Version Compatibility

| MCP Server | Supported Portainer |
|------------|-------------------|
| v0.6.x | 2.31.2 |
| v0.5.x | 2.30.0 |
| v0.4.x | 2.27.4 |

## Documentation

📖 **[Full Documentation](https://portainer.github.io/portainer-mcp/)** — Installation, configuration, meta-tools guide, architecture, security, and API reference.

| Page | Description |
|------|-------------|
| [Getting Started](https://portainer.github.io/portainer-mcp/getting-started/) | Prerequisites, installation, AI assistant setup |
| [Configuration](https://portainer.github.io/portainer-mcp/configuration/) | CLI flags, tool modes, version compatibility |
| [Meta-Tools Guide](https://portainer.github.io/portainer-mcp/guides/meta-tools/) | All 15 meta-tools with complete action reference |
| [Tools Reference](https://portainer.github.io/portainer-mcp/reference/api-reference/) | All 98 granular tools with parameters |
| [Architecture](https://portainer.github.io/portainer-mcp/reference/architecture/) | Server layers, client model, project structure |
| [Security](https://portainer.github.io/portainer-mcp/guides/security/) | Authentication, TLS, read-only mode, proxy safety |
| [Contributing](https://portainer.github.io/portainer-mcp/development/contributing/) | Development setup, code style, adding new tools |

## Development

```bash
make build                    # Build binary
make test                     # Unit tests
make test-integration         # Integration tests (requires Docker)
make test-all                 # All tests
make inspector                # Launch MCP Inspector UI
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines. The [Developer Documentation](https://portainer.github.io/portainer-mcp/development/contributing/) covers project structure, adding tools, testing, dependencies, and CI/CD in detail.

### Security

To report a vulnerability, see [SECURITY.md](SECURITY.md). Please use **private disclosure** — do not open public issues for security bugs.

## License

Copyright (c) 2025 Portainer.io — See [LICENSE](LICENSE) for details.
