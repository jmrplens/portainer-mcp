---
title: Tools Reference
description: Complete parameter reference for all 98 Portainer MCP tools.
---

# Tools Reference

Complete reference for all 98 granular MCP tools provided by the Portainer MCP Server.

Each tool is exposed via the [Model Context Protocol](https://modelcontextprotocol.io/) over stdio transport using JSON-RPC 2.0.

## Legend

| Icon | Meaning |
|------|---------|
| 🔒 | Read-only — available in `-read-only` mode |
| ✏️ | Write — modifies resources |
| ⚠️ | Destructive — deletes resources or performs irreversible operations |

## Table of Contents

- [Access Groups](#access-groups)
- [Environments](#environments)
- [Environment Groups](#environment-groups)
- [Stacks — Edge](#stacks--edge)
- [Stacks — Regular](#stacks--regular)
- [Tags](#tags)
- [Teams](#teams)
- [Users](#users)
- [Docker](#docker)
- [Kubernetes](#kubernetes)
- [Helm](#helm)
- [Registries](#registries)
- [Custom Templates](#custom-templates)
- [Webhooks](#webhooks)
- [Settings & SSL](#settings--ssl)
- [Backup & Restore](#backup--restore)
- [Edge Computing](#edge-computing)
- [App Templates](#app-templates)
- [Authentication](#authentication)
- [System](#system)

## Access Groups

### `listAccessGroups` 🔒

List all available access groups

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createAccessGroup` ✏️

Create a new access group. Use access groups when you want to define accesses on more than one environment. Otherwise, define the accesses on the environment level.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | The name of the access group |
| `environmentIds` | array\<number\> | — | The IDs of the environments that are part of the access group. Must include all the environment IDs that are part of the group - this includes new environments and the existing environments that ar... |

---

### `updateAccessGroupName` ✏️

Update the name of an existing access group.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the access group to update |
| `name` | string | ✅ | The name of the access group |

**Annotations:** `idempotentHint: true`

---

### `updateAccessGroupUserAccesses` ✏️

Update the user accesses of an existing access group.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the access group to update |
| `userAccesses` | array\<object\> | ✅ | The user accesses that are associated with all the environments in the access group. The ID is the user ID of the user in Portainer. Example: [{id: 1, access: 'environment_administrator'}, {id: 2, ... |

**Annotations:** `idempotentHint: true`

---

### `updateAccessGroupTeamAccesses` ✏️

Update the team accesses of an existing access group.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the access group to update |
| `teamAccesses` | array\<object\> | ✅ | The team accesses that are associated with all the environments in the access group. The ID is the team ID of the team in Portainer. Example: [{id: 1, access: 'environment_administrator'}, {id: 2, ... |

**Annotations:** `idempotentHint: true`

---

### `addEnvironmentToAccessGroup` ✏️

Add an environment to an access group.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the access group to update |
| `environmentId` | number | ✅ | The ID of the environment to add to the access group |

**Annotations:** `idempotentHint: true`

---

### `removeEnvironmentFromAccessGroup` ⚠️

Remove an environment from an access group.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the access group to update |
| `environmentId` | number | ✅ | The ID of the environment to remove from the access group |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

## Environments

### `listEnvironments` 🔒

List all available environments

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getEnvironment` 🔒

Get detailed information about a specific environment by its ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment to retrieve |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `deleteEnvironment` ⚠️

Delete an environment by its ID. This action cannot be undone.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

### `snapshotEnvironment` ✏️

Trigger a snapshot for a specific environment. A snapshot captures the current state of the environment including containers, images, volumes, and networks.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment to snapshot |

**Annotations:** `idempotentHint: true`

---

### `snapshotAllEnvironments` ✏️

Trigger a snapshot for all environments. A snapshot captures the current state of each environment including containers, images, volumes, and networks.

*No parameters required.*

**Annotations:** `idempotentHint: true`

---

### `updateEnvironmentTags` ✏️

Update the tags associated with an environment

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment to update |
| `tagIds` | array\<number\> | ✅ | The IDs of the tags that are associated with the environment. Must include all the tag IDs that should be associated with the environment - this includes new tags and existing tags. Providing an em... |

**Annotations:** `idempotentHint: true`

---

### `updateEnvironmentUserAccesses` ✏️

Update the user access policies of an environment

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment to update |
| `userAccesses` | array\<object\> | ✅ | The user accesses that are associated with the environment. The ID is the user ID of the user in Portainer. Must include all the access policies for all users that should be associated with the env... |

**Annotations:** `idempotentHint: true`

---

### `updateEnvironmentTeamAccesses` ✏️

Update the team access policies of an environment

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment to update |
| `teamAccesses` | array\<object\> | ✅ | The team accesses that are associated with the environment. The ID is the team ID of the team in Portainer. Must include all the access policies for all teams that should be associated with the env... |

**Annotations:** `idempotentHint: true`

---

## Environment Groups

### `listEnvironmentGroups` 🔒

List all available environment groups. Environment groups are the equivalent of Edge Groups in Portainer.

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createEnvironmentGroup` ✏️

Create a new environment group. Environment groups are the equivalent of Edge Groups in Portainer.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | The name of the environment group |
| `environmentIds` | array\<number\> | ✅ | The IDs of the environments to add to the group |

---

### `updateEnvironmentGroupName` ✏️

Update the name of an environment group. Environment groups are the equivalent of Edge Groups in Portainer.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment group to update |
| `name` | string | ✅ | The new name for the environment group |

**Annotations:** `idempotentHint: true`

---

### `updateEnvironmentGroupEnvironments` ✏️

Update the environments associated with an environment group. Environment groups are the equivalent of Edge Groups in Portainer.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment group to update |
| `environmentIds` | array\<number\> | ✅ | The IDs of the environments that should be part of the group. Must include all environment IDs that should be associated with the group. Providing an empty array will remove all environments from t... |

**Annotations:** `idempotentHint: true`

---

### `updateEnvironmentGroupTags` ✏️

Update the tags associated with an environment group. Environment groups are the equivalent of Edge Groups in Portainer.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment group to update |
| `tagIds` | array\<number\> | ✅ | The IDs of the tags that should be associated with the group. Must include all tag IDs that should be associated with the group. Providing an empty array will remove all tags from the group. Exampl... |

**Annotations:** `idempotentHint: true`

---

## Stacks — Edge

### `listStacks` 🔒

List all edge stacks. Edge stacks are deployed to Edge environments via Edge Groups. For regular Docker Compose or Swarm stacks deployed to specific environments, use listRegularStacks instead.

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getStackFile` 🔒

Get the compose file for a specific stack ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to get the compose file for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createStack` ✏️

Create a new stack

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | Name of the stack. Stack name must only consist of lowercase alpha characters, numbers, hyphens, or underscores as well as start with a lowercase character or number |
| `file` | string | ✅ | Content of the stack file. The file must be a valid docker-compose.yml file. example: services:  web:    image:nginx |
| `environmentGroupIds` | array\<number\> | ✅ | The IDs of the environment groups that the stack belongs to. Must include at least one environment group ID. Example: [1, 2, 3] |

---

### `updateStack` ✏️

Update an existing stack

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to update |
| `file` | string | ✅ | Content of the stack file. The file must be a valid docker-compose.yml file. example: version: 3  services:    web:      image:nginx |
| `environmentGroupIds` | array\<number\> | ✅ | The IDs of the environment groups that the stack belongs to. Must include at least one environment group ID. Example: [1, 2, 3] |

**Annotations:** `idempotentHint: true`

---

## Stacks — Regular

### `listRegularStacks` 🔒

List all regular (non-edge) stacks. These are Docker Compose or Swarm stacks deployed directly to specific environments. Returns stack ID, name, type, status, endpoint ID, entry point, creation info, and filesystem path. For edge stacks deployed via Edge Groups, use listStacks instead.

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getStack` 🔒

Get a specific stack by ID. Returns detailed information about a regular (non-edge) stack including name, type, status, and environment.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to inspect |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `inspectStackFile` 🔒

Get the compose file content for a specific regular (non-edge) stack by its ID. Returns the raw compose file as text.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to get the compose file for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `deleteStack` ⚠️

Delete a regular (non-edge) stack permanently. This removes the stack and all its associated containers from the environment.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to delete |
| `environmentId` | number | ✅ | The ID of the environment where the stack is deployed |
| `removeVolumes` | boolean | — | Whether to remove associated volumes when deleting the stack |

**Annotations:** `destructiveHint: true`

---

### `startStack` ✏️

Start a stopped regular (non-edge) stack. Brings up all containers defined in the stack's compose file.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to start |
| `environmentId` | number | ✅ | The ID of the environment where the stack is deployed |

**Annotations:** `idempotentHint: true`

---

### `stopStack` ✏️

Stop a running regular (non-edge) stack. Stops all containers defined in the stack's compose file.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to stop |
| `environmentId` | number | ✅ | The ID of the environment where the stack is deployed |

**Annotations:** `idempotentHint: true`

---

### `updateStackGit` ✏️

Update the git configuration of a regular (non-edge) stack. Allows changing the git reference (branch/tag) and prune settings.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to update |
| `environmentId` | number | ✅ | The ID of the environment where the stack is deployed |
| `referenceName` | string | — | The git reference name (branch or tag) to use |
| `prune` | boolean | — | Whether to prune services that are no longer in the compose file |

**Annotations:** `idempotentHint: true`

---

### `redeployStackGit` ✏️

Trigger a git-based redeployment of a regular (non-edge) stack. Pulls the latest changes from the git repository and redeploys the stack.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to redeploy |
| `environmentId` | number | ✅ | The ID of the environment where the stack is deployed |
| `pullImage` | boolean | — | Whether to pull the latest images before redeploying |
| `prune` | boolean | — | Whether to prune services that are no longer in the compose file |

---

### `migrateStack` ✏️

Migrate a regular (non-edge) stack to another environment. Moves the stack from one environment to another, optionally renaming it.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the stack to migrate |
| `environmentId` | number | ✅ | The current environment ID where the stack is deployed |
| `targetEnvironmentId` | number | ✅ | The target environment ID to migrate the stack to |
| `name` | string | — | Optional new name for the migrated stack |

---

## Tags

### `listEnvironmentTags` 🔒

List all available environment tags

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createEnvironmentTag` ✏️

Create a new environment tag

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | The name of the tag |

---

### `deleteEnvironmentTag` ⚠️

Delete an environment tag by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the environment tag to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

## Teams

### `listTeams` 🔒

List all available teams

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getTeam` 🔒

Get details of a specific team by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the team to retrieve |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createTeam` ✏️

Create a new team

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | The name of the team |

---

### `deleteTeam` ⚠️

Delete a team by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the team to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

### `updateTeamName` ✏️

Update the name of an existing team

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the team to update |
| `name` | string | ✅ | The new name of the team |

**Annotations:** `idempotentHint: true`

---

### `updateTeamMembers` ✏️

Update the members of an existing team

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the team to update |
| `userIds` | array\<number\> | ✅ | The IDs of the users that are part of the team. Must include all the user IDs that are part of the team - this includes new users and the existing users that are already associated with the team. E... |

**Annotations:** `idempotentHint: true`

---

## Users

### `listUsers` 🔒

List all available users

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getUser` 🔒

Get details of a specific user by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the user to retrieve |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createUser` ✏️

Create a new user

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `username` | string | ✅ | The username of the new user |
| `password` | string | ✅ | The password of the new user |
| `role` | string | ✅ | The role of the user. Can be admin, user or edge_admin |

---

### `deleteUser` ⚠️

Delete a user by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the user to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

### `updateUserRole` ✏️

Update an existing user

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the user to update |
| `role` | string | ✅ | The role of the user. Can be admin, user or edge_admin |

**Annotations:** `idempotentHint: true`

---

## Docker

### `dockerProxy` 🔒

Proxy Docker requests to a specific Portainer environment. This tool can be used with any Docker API operation as documented in the Docker Engine API specification (https://docs.docker.com/reference/api/engine/version/v1.48/).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment to proxy Docker requests to |
| `method` | string | ✅ | The HTTP method to use to proxy the Docker API operation |
| `dockerAPIPath` | string | ✅ | The route of the Docker API operation to proxy. Must include the leading slash. Example: /containers/json |
| `queryParams` | array\<object\> | — | The query parameters to include in the Docker API operation. Must be an array of key-value pairs. Example: [{key: 'all', value: 'true'}, {key: 'filter', value: 'dangling'}] |
| `headers` | array\<object\> | — | The headers to include in the Docker API operation. Must be an array of key-value pairs. Example: [{key: 'Content-Type', value: 'application/json'}] |
| `body` | string | — | The body of the Docker API operation to proxy. Must be a JSON string. Example: {'Image': 'nginx:latest', 'Name': 'my-container'} |

**Annotations:** `readOnlyHint: true` · `destructiveHint: true` · `idempotentHint: true`

---

### `getDockerDashboard` 🔒

Get Docker dashboard data for a specific Portainer environment. Returns container, image, network, volume, stack, and service counts and status summary.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment to get Docker dashboard data for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

## Kubernetes

### `kubernetesProxy` 🔒

Proxy Kubernetes requests to a specific Portainer environment. This tool can be used with any Kubernetes API operation as documented in the Kubernetes API specification (https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/).

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment to proxy Kubernetes requests to |
| `method` | string | ✅ | The HTTP method to use to proxy the Kubernetes API operation |
| `kubernetesAPIPath` | string | ✅ | The route of the Kubernetes API operation to proxy. Must include the leading slash. Example: /api/v1/namespaces/default/pods |
| `queryParams` | array\<object\> | — | The query parameters to include in the Kubernetes API operation. Must be an array of key-value pairs. Example: [{key: 'watch', value: 'true'}, {key: 'fieldSelector', value: 'metadata.name=my-pod'}] |
| `headers` | array\<object\> | — | The headers to include in the Kubernetes API operation. Must be an array of key-value pairs. Example: [{key: 'Content-Type', value: 'application/json'}] |
| `body` | string | — | The body of the Kubernetes API operation to proxy. Must be a JSON string. Example: {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': 'my-pod'}} |

**Annotations:** `readOnlyHint: true` · `destructiveHint: true` · `idempotentHint: true`

---

### `getKubernetesResourceStripped` 🔒

Proxy GET requests to a specific Portainer environment for Kubernetes resources, and automatically strips verbose metadata fields (such as 'managedFields') from the API response to reduce its size. This tool is intended for retrieving Kubernetes resource information where a leaner payload is desired. This tool can be used with any GET Kubernetes API operation as documented in the Kubernetes API specification (https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/). For other methods (POST, PUT, DELETE, HEAD), use the 'kubernetesProxy' tool.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment to proxy Kubernetes GET requests to |
| `kubernetesAPIPath` | string | ✅ | The route of the Kubernetes API GET operation to proxy. Must include the leading slash. Example: /api/v1/namespaces/default/pods |
| `queryParams` | array\<object\> | — | The query parameters to include in the Kubernetes API operation. Must be an array of key-value pairs. Example: [{key: 'watch', value: 'true'}, {key: 'fieldSelector', value: 'metadata.name=my-pod'}] |
| `headers` | array\<object\> | — | The headers to include in the Kubernetes API operation. Must be an array of key-value pairs. Example: [{key: 'Accept', value: 'application/json'}] |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getKubernetesDashboard` 🔒

Get a summary dashboard for a Kubernetes environment showing counts of key resources including applications, config maps, ingresses, namespaces, secrets, services, and volumes.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the Kubernetes environment to get the dashboard for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `listKubernetesNamespaces` 🔒

List all Kubernetes namespaces in a specific environment. Returns namespace details including name, creation date, owner, and whether the namespace is a default or system namespace.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the Kubernetes environment to list namespaces for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getKubernetesConfig` 🔒

Get the kubeconfig for a specific Kubernetes environment. Returns the kubeconfig content that can be used to connect to the cluster.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the Kubernetes environment to get the kubeconfig for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

## Helm

### `listHelmRepositories` 🔒

List all Helm repositories configured for a user

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `userId` | number | ✅ | The ID of the user |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `addHelmRepository` ✏️

Add a Helm repository for a user

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `userId` | number | ✅ | The ID of the user |
| `url` | string | ✅ | The URL of the Helm repository to add |

---

### `removeHelmRepository` ⚠️

Remove a Helm repository for a user

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `userId` | number | ✅ | The ID of the user |
| `repositoryId` | number | ✅ | The ID of the Helm repository to remove |

**Annotations:** `destructiveHint: true`

---

### `searchHelmCharts` 🔒

Search for Helm charts in a repository

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `repo` | string | ✅ | The URL of the Helm repository to search |
| `chart` | string | — | The name of the chart to search for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `installHelmChart` ✏️

Install a Helm chart on an environment

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment to install the chart on |
| `chart` | string | ✅ | The name of the chart to install |
| `name` | string | ✅ | The release name for the installed chart |
| `repo` | string | ✅ | The URL of the Helm repository containing the chart |
| `namespace` | string | — | The Kubernetes namespace to install the chart in |
| `values` | string | — | Custom values for the chart in YAML format |
| `version` | string | — | The version of the chart to install |

---

### `listHelmReleases` 🔒

List all Helm releases on an environment

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment |
| `namespace` | string | — | Filter releases by Kubernetes namespace |
| `filter` | string | — | Filter releases by name pattern |
| `selector` | string | — | Filter releases by label selector |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `deleteHelmRelease` ⚠️

Delete a Helm release from an environment

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment |
| `release` | string | ✅ | The name of the release to delete |
| `namespace` | string | — | The Kubernetes namespace of the release |

**Annotations:** `destructiveHint: true`

---

### `getHelmReleaseHistory` 🔒

Get the revision history of a Helm release

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environmentId` | number | ✅ | The ID of the environment |
| `name` | string | ✅ | The name of the Helm release |
| `namespace` | string | — | The Kubernetes namespace of the release |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

## Registries

### `listRegistries` 🔒

List all available registries

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getRegistry` 🔒

Get details of a specific registry by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the registry to retrieve |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createRegistry` ✏️

Create a new registry. Registry types: 1 = Quay.io, 2 = Azure Container Registry, 3 = Custom registry, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = Amazon ECR.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | The name of the registry |
| `type` | number | ✅ | The registry type: 1 = Quay.io, 2 = Azure Container Registry, 3 = Custom registry, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = Amazon ECR |
| `url` | string | ✅ | The URL of the registry (e.g. docker.io, myregistry.example.com) |
| `authentication` | boolean | ✅ | Whether the registry requires authentication |
| `username` | string | — | The username for authentication |
| `password` | string | — | The password for authentication |
| `baseURL` | string | — | The base URL of the registry (used for registries that require a separate API endpoint) |

---

### `updateRegistry` ✏️

Update an existing registry. Only the provided fields will be updated, other fields will retain their current values.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the registry to update |
| `name` | string | — | The new name of the registry |
| `url` | string | — | The new URL of the registry |
| `authentication` | boolean | — | Whether the registry requires authentication |
| `username` | string | — | The new username for authentication |
| `password` | string | — | The new password for authentication |
| `baseURL` | string | — | The new base URL of the registry |

**Annotations:** `idempotentHint: true`

---

### `deleteRegistry` ⚠️

Delete a registry by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the registry to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

## Custom Templates

### `listCustomTemplates` 🔒

List all available custom templates

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getCustomTemplate` 🔒

Get details of a specific custom template by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the custom template to retrieve |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getCustomTemplateFile` 🔒

Get the file content of a specific custom template by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the custom template to get the file content for |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createCustomTemplate` ✏️

Create a new custom template from file content

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `title` | string | ✅ | The title of the custom template |
| `description` | string | ✅ | The description of the custom template |
| `fileContent` | string | ✅ | The file content for the custom template (e.g. docker-compose.yml content) |
| `type` | number | ✅ | The template type: 1 for swarm, 2 for compose, 3 for kubernetes |
| `platform` | number | ✅ | The platform type: 1 for linux, 2 for windows |
| `note` | string | — | An optional note for the custom template |
| `logo` | string | — | An optional logo URL for the custom template |

---

### `deleteCustomTemplate` ⚠️

Delete a custom template by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the custom template to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

## Webhooks

### `listWebhooks` 🔒

List all webhooks configured in Portainer

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createWebhook` ✏️

Create a new webhook for a service or container

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `resourceId` | string | ✅ | The resource ID associated with the webhook (e.g., service ID) |
| `endpointId` | number | ✅ | The ID of the environment to deploy the webhook to |
| `webhookType` | number | ✅ | The type of webhook (1: service webhook) |

---

### `deleteWebhook` ⚠️

Delete a webhook by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the webhook to delete |

**Annotations:** `destructiveHint: true` · `idempotentHint: true`

---

## Settings & SSL

### `getSettings` 🔒

Get the settings of the Portainer instance

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `updateSettings` ✏️

Update the Portainer settings. Accepts a JSON string containing the settings fields to update (partial update supported). Fields include authenticationMethod, enableEdgeComputeFeatures, edge configuration, and more.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `settings` | string | ✅ | A JSON string containing the settings fields to update |

**Annotations:** `idempotentHint: true`

---

### `getPublicSettings` 🔒

Get the public settings of the Portainer instance. These settings are available without authentication and include authentication method, logo URL, OAuth configuration, and feature flags.

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getSSLSettings` 🔒

Get the SSL settings of the Portainer instance, including certificate paths, HTTP enabled status, and self-signed flag.

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `updateSSLSettings` ✏️

Update the SSL settings of the Portainer instance. Allows updating the SSL certificate, key, and whether HTTP is enabled.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `cert` | string | — | The SSL certificate content (PEM format) |
| `key` | string | — | The SSL private key content (PEM format) |
| `httpEnabled` | boolean | — | Whether HTTP is enabled (true/false) |

**Annotations:** `idempotentHint: true`

---

## Backup & Restore

### `getBackupStatus` 🔒

Get the status of the last Portainer backup, including whether it failed and the timestamp

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getBackupS3Settings` 🔒

Get the current S3 backup settings configured in Portainer

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createBackup` ✏️

Create a backup of the Portainer server

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `password` | string | — | Optional password to encrypt the backup |

---

### `backupToS3` ✏️

Backup the Portainer server to an S3-compatible storage

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `accessKeyID` | string | ✅ | The AWS access key ID for S3 authentication |
| `secretAccessKey` | string | ✅ | The AWS secret access key for S3 authentication |
| `bucketName` | string | ✅ | The name of the S3 bucket to store the backup |
| `region` | string | — | The AWS region of the S3 bucket |
| `s3CompatibleHost` | string | — | The S3-compatible host URL (for non-AWS S3 services) |
| `password` | string | — | Optional password to encrypt the backup |
| `cronRule` | string | — | A cron expression for scheduled S3 backups |

---

### `restoreFromS3` ⚠️

Restore the Portainer server from an S3-compatible storage backup

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `accessKeyID` | string | ✅ | The AWS access key ID for S3 authentication |
| `secretAccessKey` | string | ✅ | The AWS secret access key for S3 authentication |
| `bucketName` | string | ✅ | The name of the S3 bucket containing the backup |
| `filename` | string | ✅ | The filename of the backup to restore |
| `password` | string | — | The password to decrypt the backup (if encrypted) |
| `region` | string | — | The AWS region of the S3 bucket |
| `s3CompatibleHost` | string | — | The S3-compatible host URL (for non-AWS S3 services) |

**Annotations:** `destructiveHint: true`

---

## Edge Computing

### `listEdgeJobs` 🔒

List all edge jobs configured in Portainer. Returns job ID, name, cron expression, recurring status, and edge groups

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getEdgeJob` 🔒

Get details of a specific edge job by ID, including name, cron expression, recurring status, and edge groups

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the edge job |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getEdgeJobFile` 🔒

Get the script file content of a specific edge job by ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the edge job |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `createEdgeJob` ✏️

Create a new edge job with script content, cron schedule, and target environments or edge groups

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ | The name of the edge job |
| `cronExpression` | string | ✅ | The cron expression for job scheduling |
| `fileContent` | string | ✅ | The script content of the edge job |
| `recurring` | boolean | — | Whether the job should run on a recurring schedule |
| `endpoints` | array\<any\> | — | Array of environment IDs to target |
| `edgeGroups` | array\<any\> | — | Array of edge group IDs to target |

---

### `deleteEdgeJob` ⚠️

Delete an edge job by its ID

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the edge job to delete |

**Annotations:** `destructiveHint: true`

---

### `listEdgeUpdateSchedules` 🔒

List all edge update schedules configured in Portainer. Returns schedule ID, name, type, status, scheduled time, and edge groups

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

## App Templates

### `listAppTemplates` 🔒

List all available application templates in Portainer. Returns template ID, title, description, type, image, categories, platform, and other metadata.

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getAppTemplateFile` 🔒

Get the file content (e.g., docker-compose.yml) of a specific application template by its ID.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `id` | number | ✅ | The ID of the application template |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

## Authentication

### `authenticate` 🔒

Authenticate a user against Portainer using a username and password. Returns a JWT token that can be used for subsequent API calls

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `username` | string | ✅ | The username for authentication |
| `password` | string | ✅ | The password for authentication |

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `logout` ✏️

Log out the current user session from Portainer

*No parameters required.*

**Annotations:** `idempotentHint: true`

---

## System

### `getSystemStatus` 🔒

Get the system status of the Portainer instance, including version and instance ID

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `getMOTD` 🔒

Get the Portainer message of the day (MOTD), including title, message, and style information

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---

### `listRoles` 🔒

List all available roles in Portainer, including their authorizations and priority

*No parameters required.*

**Annotations:** `readOnlyHint: true` · `idempotentHint: true`

---


*Generated from `tools.yaml` — 98 tools documented.*