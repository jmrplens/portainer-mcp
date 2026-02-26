# Security Considerations

## Date: 2025-02

## Context

The portainer-mcp server acts as a bridge between MCP clients and the Portainer API, exposing powerful infrastructure management capabilities. This document enumerates known security considerations and the mitigations in place.

## Proxy Handlers (Docker & Kubernetes)

The Docker proxy (`HandleDockerProxy`) and Kubernetes proxy (`HandleKubernetesProxy`, `HandleKubernetesProxyStripped`) allow callers to invoke **any** API path on the target engine. There is no allowlist of permitted endpoints.

**Mitigations:**
- The `readOnly` server flag prevents proxy write operations from being registered.
- The Portainer API token used by the MCP server limits what the caller can actually do — operations are constrained by the token's role and endpoint access policies within Portainer.
- Response body size is capped at 10 MB via `io.LimitReader` to prevent memory exhaustion.
- HTTP method validation prevents non-standard methods.

**Recommendation:** Operators should use a least-privilege Portainer API token and enable `--read-only` mode when write operations are not needed.

## Settings Update

The `HandleUpdateSettings` handler passes a caller-supplied JSON map directly to the Portainer settings API without restricting which fields can be modified.

**Mitigations:**
- Only registered when `readOnly` is `false`.
- Portainer's own RBAC enforces that the API token must have admin privileges.

**Recommendation:** Use read-only mode or a non-admin token when full settings access is unnecessary.

## Credential Handling

Several handlers accept sensitive parameters (passwords, access keys, secret keys):
- `HandleCreateUser` — user password
- `HandleCreateRegistry` / `HandleUpdateRegistry` — registry password
- `HandleAuthenticateUser` — login password
- `HandleCreateBackup` — encryption password
- `HandleBackupToS3` / `HandleRestoreFromS3` — AWS credentials

**Mitigations:**
- Passwords are never logged by handler code. The only `log.Printf` in the package is for tool registration and does not include parameter values.
- Sensitive values are passed directly to the Portainer API client without intermediate storage.

**Recommendation:** Ensure that any logging middleware or MCP transport layer configured on top of this server does **not** log raw request parameters, as they may contain credentials. Future work could add a `sensitive` annotation to tool parameter definitions to signal that values should be redacted in logs.

## TLS Verification

TLS certificate verification is enabled by default. The `--skip-tls-verify` flag must be explicitly passed to disable it for development/testing against self-signed certificates.

**Recommendation:** Never use `--skip-tls-verify` in production environments.

## Decisions

| #  | Decision                                          | Rationale                                              |
|----|---------------------------------------------------|--------------------------------------------------------|
| D1 | No proxy path allowlist                           | Portainer RBAC provides access control; an allowlist would limit legitimate use cases |
| D2 | No settings field whitelist                       | Same rationale as D1; admin tokens already have full access |
| D3 | Document credential handling rather than mask      | No logging occurs in handlers; masking adds complexity without current benefit |
| D4 | TLS verify on by default                          | Secure-by-default; opt-out via explicit flag |
