# Live Integration Tests

These tests run against a real Portainer instance to verify all MCP tools work correctly
with actual API responses. They are designed to be **completely non-destructive**:

- **Read-only tools**: Simply call and verify the response is valid JSON with expected fields
- **Write tools (CRUD)**: Create a temporary resource → verify → clean up (delete)
- **Update tools**: Save original value → modify → verify → restore original
- **Dangerous tools (backup/restore)**: Only tested where safe, or skipped with explanation

## Prerequisites

- A running Portainer instance (EE or CE, ideally with Pro license for full coverage)
- An API token with admin privileges
- Network access from the test runner to the Portainer instance

## Running

```bash
# Set environment variables
export PORTAINER_LIVE_URL="192.168.0.40:31015"     # No protocol prefix!
export PORTAINER_LIVE_TOKEN="ptr_xxx..."

# Run all live tests
go test -v -tags=live ./tests/live/...

# Run a specific test group
go test -v -tags=live ./tests/live/... -run TestLive_ReadOnly
go test -v -tags=live ./tests/live/... -run TestLive_CRUD
go test -v -tags=live ./tests/live/... -run TestLive_Settings

# Run with timeout (some operations can be slow)
go test -v -tags=live -timeout 300s ./tests/live/...
```

## Test Categories

| Category | What it tests | Safety |
|----------|--------------|--------|
| `TestLive_ReadOnly` | All list/get tools | Completely safe, no mutations |
| `TestLive_CRUD_Tags` | Create/delete tag lifecycle | Creates temp tag, deletes it |
| `TestLive_CRUD_Teams` | Create/get/delete team | Creates temp team, deletes it |
| `TestLive_CRUD_Users` | Create/get/delete user | Creates temp user, deletes it |
| `TestLive_CRUD_Registries` | Create/update/delete registry | Creates temp registry, deletes it |
| `TestLive_CRUD_CustomTemplates` | Full template lifecycle | Creates temp template, deletes it |
| `TestLive_CRUD_Webhooks` | Webhook lifecycle | Creates temp webhook, deletes it |
| `TestLive_Settings` | Get/update/restore settings | Saves original, modifies, restores |
| `TestLive_SSL` | Get SSL settings | Read-only (too dangerous to modify) |
| `TestLive_Auth` | Authenticate + logout | Creates a temp session, logs it out |
| `TestLive_DockerProxy` | Docker API proxy | Read-only GET requests only |
| `TestLive_Stacks` | Stack inspection | Read-only on existing stacks |
| `TestLive_Environments` | Snapshot trigger | Triggers snapshot (safe, non-destructive) |
| `TestLive_Kubernetes` | K8s dashboard/namespaces | Read-only, requires K8s endpoint |

## Design Principles

1. **Non-destructive by default**: Tests never modify existing resources
2. **Self-cleaning**: Any created resource is cleaned up in `t.Cleanup()`
3. **Idempotent**: Tests can be run multiple times without side effects
4. **Skip-friendly**: Tests skip gracefully when features aren't available
5. **Parallelizable**: Read-only tests can run in parallel
6. **CI-ready**: Protected by build tag, controlled by env vars
