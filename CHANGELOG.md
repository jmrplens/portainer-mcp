# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added
- **98-tool expansion**: Tags, Teams, Users, Registries, Custom Templates, Environments, Backup & Restore, Roles, Webhooks, MOTD, Edge Jobs, Edge Update Schedules, Settings, SSL, App Templates, Authentication, Helm, Docker Dashboard, Kubernetes Dashboard & Config
- Regular (non-edge) Docker Compose stack management: list, get, inspect file, delete, start, stop, git update, redeploy, migrate
- Full CRUD for: users, teams, registries, custom templates, environment groups, webhooks, edge jobs
- Access group management (list, create, update, add/remove environments)
- Environment management (delete, snapshots, tag/access updates)
- Backup/restore operations (local backup, S3 backup, S3 restore, status, settings)
- Settings and SSL management (read + update)
- System status and MOTD retrieval
- Helm chart management (repos, search, install, releases, history)
- Kubernetes dashboard, namespaces, kubeconfig retrieval
- Docker dashboard data retrieval
- Authentication tools (login, logout)
- Role listing with authorizations
- App template listing and file retrieval
- Comprehensive documentation (README, CONTRIBUTING, CHANGELOG, API reference)

### Fixed
- **tools.yaml schema keys**: Corrected 12 Helm/Edge tools using `inputSchema:` to `parameters:` — those tools were silently registered with zero parameters
- **Integer overflow in parameter parsing**: Added bounds checking in `GetInt()` and `parseArrayOfIntegers()` to prevent silent wraparound on extreme float64 values

### Changed
- Updated tools.yaml version to v1.2

## [v0.6.1] — 2025-05-16

### Added
- Regular Docker Compose stack operations support
- Raw HTTP scheme normalization for client

### Fixed
- Stack file content retrieval for non-edge stacks

## [v0.6.0] — 2025-05-15

### Added
- Kubernetes resource stripped operation (`getKubernetesResourceStripped`) — GET requests with automatic metadata field stripping
- `--disable-version-check` flag to skip Portainer version validation
- New Docker proxy operation for direct Docker API access

### Changed
- Updated mcp-go SDK to v0.32.0
- Updated Portainer client-api-go to v2.31.2

## [v0.5.0] — 2025-05-06

### Changed
- Updated Portainer client API to v2.30.0
- Aligned with Portainer 2.30.0 API changes

## [v0.4.1] — 2025-04-25

### Added
- Linux ARM64 binary support in release workflow

## [v0.4.0] — 2025-04-24

### Added
- Tool annotations (readOnlyHint, destructiveHint, idempotentHint, openWorldHint) for safer AI operation
- Portainer client refactoring with two-layer model architecture

### Changed
- Updated MCP SDK from v0.20.1 to v0.24.1 (via v0.23.1)
- Refactored client package with wrapper client pattern

## [v0.3.0] — 2025-04-15

### Added
- Kubernetes API proxy (`kubernetesProxy`)
- CI workflows for build and release automation
- Code coverage reporting
- `cloc.sh` script for lines-of-code metrics

## [v0.2.0] — 2025-04-09

### Added
- Docker API proxy (`dockerProxy`)
- LICENSE file

### Security
- Removed API token from log output

## [v0.1.0] — 2025-04-08

### Added
- Initial release
- MCP server with stdio transport
- Environment listing and details
- Edge stack management (list, get file, create, update)
- Environment group management
- Tag management
- Basic tool definition system via YAML

[Unreleased]: https://github.com/portainer/portainer-mcp/compare/v0.6.1...HEAD
[v0.6.1]: https://github.com/portainer/portainer-mcp/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/portainer/portainer-mcp/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/portainer/portainer-mcp/compare/v0.4.1...v0.5.0
[v0.4.1]: https://github.com/portainer/portainer-mcp/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/portainer/portainer-mcp/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/portainer/portainer-mcp/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/portainer/portainer-mcp/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/portainer/portainer-mcp/releases/tag/v0.1.0
