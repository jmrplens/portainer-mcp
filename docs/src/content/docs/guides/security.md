---
title: Security
description: Security considerations and best practices for running Portainer MCP.
---

import { Aside } from '@astrojs/starlight/components';

## Authentication

The server authenticates to Portainer using an **API token** passed via the `-token` flag. This token has the same permissions as the user that created it.

### Best Practices

- **Use a dedicated service account** — create a Portainer user specifically for MCP access, rather than using your admin token
- **Apply least privilege** — if you only need read access, create a read-only user in Portainer and additionally pass `-read-only` to the MCP server
- **Rotate tokens regularly** — regenerate the API token periodically
- **Never commit tokens** — use environment variables or secret managers to pass the token

```bash
# Good: token from environment variable
./portainer-mcp -server "https://portainer:9443" -token "$PORTAINER_TOKEN"

# Bad: token hardcoded in config
./portainer-mcp -server "https://portainer:9443" -token "ptr_abc123..."
```

## TLS / Certificate Verification

By default, the server **validates TLS certificates** when connecting to Portainer. This protects against man-in-the-middle attacks.

The `-skip-tls-verify` flag disables certificate validation. Only use this for:
- Development environments with self-signed certificates
- Testing and debugging

<Aside type="caution">
Never use `-skip-tls-verify` in production. It makes the connection vulnerable to interception.
</Aside>

## Read-Only Mode

The `-read-only` flag restricts the MCP server to observation-only operations:

- **No create, update, or delete operations** are available
- In meta-tools mode, write actions are removed from the action enum
- In granular mode, only tools with `readOnlyHint: true` are registered

This is ideal for:
- Monitoring dashboards where you want the AI to observe but not modify
- Onboarding new team members safely
- Audit and compliance workflows
- Any scenario where accidental modifications are unacceptable

## MCP Tool Annotations

Every tool includes safety annotations that help AI assistants make informed decisions:

| Annotation | Effect |
|:-----------|:-------|
| `readOnlyHint: true` | Tool only reads data — safe to call without confirmation |
| `destructiveHint: true` | Tool can delete or irreversibly modify — AI should confirm with user |
| `idempotentHint: true` | Safe to retry — calling twice produces the same result |
| `openWorldHint: true` | Interacts with external systems (Docker/K8s APIs) — results may vary |

These annotations follow the [MCP specification](https://modelcontextprotocol.io/docs/concepts/tools#annotations-optional) and are used by well-behaved AI assistants to decide when to ask for user confirmation before executing a tool.

## Proxy Tools

The `docker_proxy` and `kubernetes_proxy` tools allow proxying arbitrary API calls to Docker and Kubernetes through Portainer. These are powerful tools that can:

- Execute any Docker Engine API call
- Execute any Kubernetes API call
- Perform operations not covered by the other tools

### Mitigations

- **Response size limits** — proxy responses are capped at 10 MB to prevent memory exhaustion
- **HTTP method validation** — only standard HTTP methods (GET, POST, PUT, DELETE, HEAD, PATCH) are accepted
- **Path validation** — API paths must start with `/`
- **Read-only filtering** — in read-only mode, proxy tools are not registered

### Recommendations

- If you don't need proxy access, consider customizing which tools are available
- Monitor Portainer audit logs for unexpected API calls
- Use Portainer's RBAC to limit what the API token can access

## Settings Update

The `update_settings` and `update_ssl_settings` tools can modify Portainer server configuration. These accept JSON payloads that are passed to the Portainer API.

### Recommendations

- Use read-only mode if settings changes are not needed
- Review the [Portainer API documentation](https://docs.portainer.io/api/docs) for available settings fields
- Use Portainer's audit log to track configuration changes

## Sensitive Data

The following tools handle sensitive data:

| Tool | Sensitive Data | Mitigation |
|:-----|:--------------|:-----------|
| `create_user` | Password | Passed as plaintext to Portainer API (HTTPS encrypted in transit) |
| `create_registry` / `update_registry` | Registry credentials | Credentials sent to Portainer for storage |
| `backup_to_s3` / `restore_from_s3` | AWS credentials | S3 access keys sent to Portainer |
| `authenticate` | Username/password | Authentication credentials |

Ensure your Portainer instance uses HTTPS to protect these values in transit.

## Version Compatibility

The server validates the Portainer version at startup. Running against an unsupported version may result in:

- Missing API endpoints causing errors
- Changed response formats causing data loss
- New fields not being captured

Use `-disable-version-check` only when you understand the risks of version mismatch.

## Network Security

The MCP server communicates:

1. **With the AI assistant** — via stdio (stdin/stdout), no network involved
2. **With Portainer** — via HTTPS to the configured server URL

Ensure the machine running the MCP server has network access to the Portainer instance and that this connection is secured with TLS.
