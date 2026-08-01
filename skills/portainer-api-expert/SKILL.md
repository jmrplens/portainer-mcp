# Portainer API Expert

**When to use**: Working with the Portainer REST API, adding a new API domain, understanding how the Go SDK maps to API endpoints, or debugging API call failures.
**Triggers on**: portainer api, sdk, client, adapter, endpoint, raw model, apimodels, ConvertXxx, HTTP client, API domain
**Covers**: SDK client structure, adapter file layout, domain table, model conversion pattern, Docker/K8s proxy, authentication, version compatibility

---

Deep knowledge of the Portainer REST API, its Go SDK, and how this project wraps them.

## Portainer API Overview

Portainer exposes a REST API (typically at `https://<host>:9443/api/`). Authentication uses an API token passed via the `X-API-Key` header. This project uses the official Go SDK `github.com/portainer/client-api-go/v2` to communicate with the API.

## SDK Client (`pkg/portainer/client/`)

The raw SDK client is created in `client.go` and communicates directly with Portainer:

```go
import (
    "github.com/portainer/client-api-go/v2/client"
    apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)
```

Each domain has its own file implementing methods that:
1. Call the raw SDK method
2. Transform the raw response model (`apimodels.*`) into a local model (`models.*`)
3. Return the local model

Example pattern:
```go
func (c *PortainerClient) GetUsers() ([]models.User, error) {
    rawUsers, err := c.cli.ListUsers()
    if err != nil {
        return nil, fmt.Errorf("failed to list users: %w", err)
    }
    return models.ConvertUsers(rawUsers), nil
}
```

### Adapter File Layout

The client package has two naming conventions — **new domains must use `adapter_<domain>.go`**. A small set of legacy files use plain `<domain>.go`; reuse that naming only when extending an existing legacy file, never for new domains.

| File | Domain |
|------|--------|
| `adapter.go` | Shared/utility client setup |
| `adapter_auth.go` | Authentication |
| `adapter_backups.go` | Backups + S3 |
| `adapter_docker.go` | Docker proxy + dashboard |
| `adapter_edge.go` | Edge jobs + update schedules |
| `adapter_environments.go` | Environments (endpoints) |
| `adapter_helm.go` | Helm repos, charts, releases |
| `adapter_kubernetes.go` | Kubernetes proxy + dashboard |
| `adapter_registries.go` | Container registries |
| `adapter_settings.go` | Settings + SSL |
| `adapter_stacks.go` | Edge stacks + regular stacks |
| `adapter_system.go` | System status + version |
| `adapter_tags.go` | Environment tags |
| `adapter_teams.go` | Teams + membership |
| `adapter_templates.go` | App templates + custom templates |
| `adapter_users.go` | User accounts |
| `adapter_webhooks.go` | Webhooks |
| `access_group.go` | Access groups (edge groups) — uses plain naming |
| `group.go` | Environment groups — uses plain naming |

> **Note**: `access_group.go` and `group.go` predate the `adapter_` naming convention. They implement the same pattern but are not prefixed.

## API Domains

| Domain | API Prefix | Go Files |
|--------|-----------|----------|
| Environments | `/api/endpoints` | `environment.go` |
| Stacks | `/api/stacks`, `/api/edge_stacks` | `stack.go` |
| Users | `/api/users` | `user.go` |
| Teams | `/api/teams` | `team.go` |
| Docker | `/api/endpoints/{id}/docker/...` | `docker.go` |
| Kubernetes | `/api/endpoints/{id}/kubernetes/...` | `kubernetes.go` |
| Registries | `/api/registries` | `registry.go` |
| Settings | `/api/settings` | `settings.go` |
| Templates | `/api/templates` | `app_template.go`, `custom_template.go` |
| Helm | `/api/endpoints/{id}/kubernetes/helm` | `helm.go` |
| Backups | `/api/backup` | `backup.go` |
| Tags | `/api/tags` | `tag.go` |
| Roles | `/api/roles` | `role.go` |
| Webhooks | `/api/webhooks` | `webhook.go` |
| Edge Jobs | `/api/edge_jobs` | `edge_job.go` |
| Auth | `/api/auth` | `auth.go` |
| System | `/api/system/status`, `/api/motd` | `system.go`, `motd.go` |

## Model Conversion Pattern

Raw API models (verbose, deeply nested) are converted to local models (flat, relevant fields only):

```go
// In pkg/portainer/models/user.go
type User struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Role     string `json:"role"`
}

func ConvertUser(raw *apimodels.PortainereeUser) User {
    return User{
        ID:       int(*raw.ID),
        Username: raw.Username,
        Role:     convertUserRole(int(*raw.Role)),
    }
}
```

## Docker/Kubernetes Proxy

For Docker and Kubernetes operations, the MCP server acts as a proxy. The client passes through raw API requests/responses with a 10MB response size limit. These don't use model conversion — they return raw JSON.

## Authentication

API token is passed via CLI `--token` flag. The SDK client sets it as the `X-API-Key` header on every request. Token creation/management is done via the Portainer API's `/api/auth` endpoint.

## Version Compatibility

This server validates that the connected Portainer instance matches `SupportedPortainerVersion` (currently 2.39.1). Major and minor version must match; patch version is flexible. Use `--disable-version-check` to bypass.
