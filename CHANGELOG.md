# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

The server is being rewritten and has no releases yet. This file starts from the rewrite; the
release history of the project this repository grew out of lives in that project's own repository.

### Added

- MCP server over stdio on the official Go SDK, with three tool surfaces projected from one action
  catalog: `dynamic` (default), `meta` and `individual`.
- An action catalog generated from Portainer's vendored OpenAPI documents, with per-action edition
  gating (Community / Business) and per-field edition pruning.
- Domains: `system`, `tags`, `registries`, `docker`, `custom_templates` — 36 actions covering 35 of
  441 Business Edition operations and 27 of 251 Community Edition operations.
- Read-only mode, and a safe mode that previews a mutating call's field names without sending it.
- Credential redaction on every response that can carry one, enforced by the generator: it refuses
  to emit a handler for such an operation until a concretely-typed redactor exists.
- An end-to-end estate — Community and Business servers, agent, edge agent, Swarm, a git server and
  a k3d Kubernetes leg with GPU support — that every action is exercised against on both editions.
- Audits that gate CI: 1:1 coverage ratchet, specification drift, generated-code freshness, and a
  guard that nothing but the transport writes to stdout.

### Changed

- Relicensed under MIT. The repository began as a fork and is no longer one; none of the original
  project's code remains.
