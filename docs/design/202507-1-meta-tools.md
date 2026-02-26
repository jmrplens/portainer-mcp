# 202507-1: Meta-Tools — Grouped Tool Interface for LLMs

**Date**: 14/07/2025

### Context

The Portainer MCP server exposes 98 individual tools (granular tools), each mapping one-to-one with a Portainer API operation. While this provides comprehensive coverage, most LLM tool-selection algorithms degrade significantly when presented with more than ~20-30 tools. Models spend excessive tokens evaluating irrelevant tools, hallucinate tool names, or simply fail to pick the right tool.

### Decision

Introduce a **meta-tools layer** that groups the 98 granular tools into **15 domain-based meta-tools**. Each meta-tool exposes a single `action` parameter (string enum) that routes to the corresponding granular handler. Meta-tools are the **default mode**; the original 98 tools are available via a `--granular-tools` CLI flag.

The 15 meta-tools are:
1. `manage_environments` — environments, environment groups, tags
2. `manage_stacks` — regular stacks, compose operations
3. `manage_access_groups` — access groups CRUD and access policies
4. `manage_users` — user CRUD and role management
5. `manage_teams` — teams and team membership
6. `manage_docker` — Docker proxy and dashboard
7. `manage_kubernetes` — Kubernetes proxy, namespaces, config, dashboard
8. `manage_helm` — Helm repos, charts, releases
9. `manage_registries` — container registry management
10. `manage_templates` — custom and app templates
11. `manage_backups` — backup, restore, S3 settings
12. `manage_webhooks` — webhook CRUD
13. `manage_edge` — edge jobs and update schedules
14. `manage_settings` — server settings and SSL
15. `manage_system` — version, status, MOTD, roles, auth

### Rationale

- **Better tool selection**: 15 tools are well within the sweet spot for LLMs.
- **Zero functionality loss**: Every granular tool action is accessible as a meta-tool action.
- **Backward compatibility**: `--granular-tools` preserves the original behavior.
- **Read-only mode compatibility**: Write actions are filtered from meta-tool action enums when `--read-only` is set. If all remaining actions are read-only, the meta-tool annotation is adjusted accordingly.
- **No parameter duplication**: Sub-action parameters are not re-declared on the meta-tool. Each handler extracts its own parameters from `request.Arguments` as before.

### Architecture

Meta-tools are defined programmatically in Go (not in tools.yaml) because:
1. Action enums are dynamic — they change based on `--read-only` mode
2. Handler references are Go method values, not serializable in YAML
3. Registration uses the same `server.MCPServer.AddTool()` API as granular tools

Key files:
- `internal/mcp/metatool_registry.go` — 15 group definitions with action→handler mappings
- `internal/mcp/metatool_handler.go` — registration and routing logic

### Trade-offs

**Benefits:**
- Dramatic improvement in LLM tool-selection accuracy
- Simpler mental model for users configuring AI assistants
- Clean separation — meta-tools layer is purely additive

**Challenges:**
- Parameters for each action are not visible in the meta-tool schema — the LLM must know (or discover) what parameters each action requires
- Action names must remain unique across all meta-tools
- Adding new granular tools requires updating the corresponding meta-tool definition
