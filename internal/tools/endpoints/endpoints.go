// Package endpoints declares the Portainer environment actions.
//
// Twenty-seven actions, the last domain of wave 1 and the one with the most
// hand-written surface. Portainer calls these "endpoints" on the wire and
// "environments" in its own user interface; this domain keeps the wire name
// for the directory and the operation identifiers, because that is what the
// vendored specification and the generated client use, and says
// "environment" in every narrative a model reads.
//
// Twenty-two actions run on cmd/gen_action_inputs's generated code
// (actions.go). Five are hand-written in handlers.go, for three reasons that
// file records in full: two multipart-only bodies whose typed client method
// oapi-codegen named something clientMethodFor cannot find (EndpointCreate,
// EndpointDockerBrowsePut), two specification "number" parameters against a
// generated client's float32 (EndpointList, EndpointUpdate), and one path
// parameter the specification mistypes as an integer (SnapshotContainerInspect).
// A twenty-eighth operation, EndpointDeleteBatchDeprecated (DELETE
// /endpoints), is marked deprecated upstream, generates nothing, and carries
// its own api/coverage-allowlist.yaml entry.
//
// Three operations that a reader would look for in internal/tools/docker
// land here instead: snapshotInspect, snapshotContainersList and
// snapshotContainerInspect are all tagged ["endpoints", "docker"], and
// cmd/gen_action_inputs routes an operation by tags[0]. They read Portainer's
// own stored snapshot of a Docker environment, not the Docker daemon, which
// is what makes "endpoints" the right home for them rather than an accident
// of tag order.
//
// Nine redaction wrappers, below, are what let the generator emit a handler
// for the nine operations whose success response can carry a credential.
// Six of those nine are generated; the other three (list, create, update)
// are among the hand-written five, and nothing mechanical makes a
// hand-written handler call its wrapper — handlers_test.go is what does.
package endpoints

import (
	"github.com/jmrplens/portainer-mcp/internal/edition"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every environment action: generatedSpecs()'s twenty-two
// entries (see actions.go) plus the five kept hand-written in handlers.go.
//
// All twenty-seven route through toolutil.WithNarrative rather than
// assigning Title/Description directly in an ActionSpec literal, which is
// what lets cmd/audit_spec_drift recognise each deliberate improvement on
// the vendored specification's own wording as deliberate
// (toolutil.ActionSpec.TitleOverridden/DescriptionOverridden) instead of
// gating on it as accidental drift, with no api/spec-drift-allowlist.yaml
// entry needed to say the same thing a second time.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.list", Domain: "endpoints", OperationID: "EndpointList",
			Title:       "List environments(endpoints)",
			Description: "List all environments(endpoints) based on the current user authorizations. Will\nreturn all environments(endpoints) if using an administrator or team leader account otherwise it will\nonly return authorized environments(endpoints).",
			Edition:     edition.CE,
			Handler:     endpointList,
			Input:       endpointListInput{},
		}, narrative("EndpointList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.create", Domain: "endpoints", OperationID: "EndpointCreate",
			Title:       "Create a new environment(endpoint)",
			Description: "Create a new environment(endpoint) that will be used to manage an environment(endpoint).",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     endpointCreate,
			Input:       endpointCreateInput{},
		}, narrative("EndpointCreate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.update", Domain: "endpoints", OperationID: "EndpointUpdate",
			Title:       "Update an environment(endpoint)",
			Description: "Update an environment(endpoint).",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointUpdate,
			Input:       endpointUpdateInput{},
		}, narrative("EndpointUpdate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.docker_browse_put", Domain: "endpoints", OperationID: "EndpointDockerBrowsePut",
			Title:       "Upload a file under a specific path on the file system of an environment (endpoint)",
			Description: "Use this environment(endpoint) to upload TLS files.",
			// CE, although not one of the thirty-eight vendored Community
			// specifications gives this operation an operationId. It is a
			// standing defect in Portainer's Community document — fourteen of
			// its 265 operations carry no operationId, against one of
			// Business's 442 — and every one of those routes is otherwise
			// fully documented and actually served.
			//
			// cmd/gen_applicability borrows the name from the edition that
			// does publish it, for any (method, path) this edition's own spans
			// table already proves it serves; see borrowIDsAcrossEditions
			// there. Without that, the edition index would read this operation
			// as Business-exclusive and actioncatalog.Build would refuse this
			// very line, for a name Community never published rather than a
			// route it never served.
			Edition:  edition.CE,
			Mutating: true,
			Handler:  endpointDockerBrowsePut,
			Input:    endpointDockerBrowsePutInput{},
		}, narrative("EndpointDockerBrowsePut")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.settings_update", Domain: "endpoints", OperationID: "EndpointSettingsUpdate",
			Title:       "Update settings for an environment(endpoint)",
			Description: "Update settings for an environment(endpoint).",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointSettingsUpdate,
			Input:       endpointSettingsUpdateInput{},
		}, narrative("EndpointSettingsUpdate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.association_delete", Domain: "endpoints", OperationID: "EndpointAssociationDelete",
			Title:       "De-association an edge environment(endpoint)",
			Description: "De-association an edge environment(endpoint).",
			Edition:     edition.CE,
			Mutating:    true,
			// The second hand-set Destructive, and the second operation
			// dangerMismatchWarnings flags: a PUT whose operationId ends
			// "Delete". It clears the environment's edge identity, so the
			// agent that was connected can no longer reach Portainer with the
			// key it holds and has to be re-enrolled with a new one. Calling
			// it again on an already-disassociated environment is a no-op,
			// which is why Idempotent stays true — idempotent and destructive
			// are independent, and this operation is both.
			Destructive: true,
			Idempotent:  true,
			Handler:     endpointAssociationDelete,
			Input:       endpointAssociationDeleteInput{},
		}, narrative("EndpointAssociationDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.snapshot_containers_list", Domain: "endpoints", OperationID: "SnapshotContainersList",
			Title:       "Fetch containers list from a snapshot",
			Description: "Fetch containers list from a snapshot",
			Edition:     edition.EE,
			Handler:     snapshotContainersList,
			Input:       snapshotContainersListInput{},
		}, narrative("SnapshotContainersList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoints.snapshot_container_inspect", Domain: "endpoints", OperationID: "SnapshotContainerInspect",
			Title:       "Fetch container from a snapshot",
			Description: "Fetch container from a snapshot",
			Edition:     edition.EE,
			Handler:     snapshotContainerInspect,
			Input:       snapshotContainerInspectInput{},
		}, narrative("SnapshotContainerInspect")),
	)
}

// narrative supplies every action's ActionSpec narrative fields, generated
// and hand-written alike. Every operationId this domain declares appears
// here; there is no default case that silently returns the vendored
// wording, because every one of the twenty-seven was judged and most were
// improved. The vendored text for this tag is unusually weak — five
// operations have no description at all beyond the stripped
// "**Access policy**:" boilerplate, and nine more have one that restates
// the summary in different capitalisation — and two are actively wrong
// about their own route.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "EndpointList":
		return toolutil.ActionNarrative{
			Title: "List environments",
			Description: "Returns the environments — Docker hosts, Swarm clusters, Kubernetes clusters, Azure ACI subscriptions and edge agents — this account may see. " +
				"An administrator or team leader sees all of them; anyone else sees only the ones they have been granted. " +
				"The filters compose: name and nameFilter match on the name, groupIds and tagIds on membership, types on the environment kind, status on up/down, and outdated on the agent version. " +
				"excludeSnapshots omits the stored Docker or Kubernetes snapshot from each entry, which is what makes the response large; ask for it when a fleet-wide list is slow. " +
				"Every environment's id here is what endpointId and environmentId mean everywhere else in this catalog. " +
				"An empty list is a legitimate answer, not an error: a Portainer deployed into a Kubernetes cluster does not enrol that cluster as an environment by itself. " +
				"Credentials are stripped from every entry: an environment's Azure key, edge key and TLS certificate paths are removed before you see it.",
		}
	case "EndpointInspect":
		return toolutil.ActionNarrative{
			Title: "Inspect an environment",
			Description: "Returns one environment in full: its connection details, group and tag membership, access policies, and the stored snapshot of what runs on it. " +
				"excludeSnapshot omits that snapshot, which is the bulk of the answer. " +
				"id is the environment's own numeric identifier from endpoints.list, not the UUID an edge agent carries as its EDGE_ID. " +
				"Azure credentials, the edge key and the TLS certificate paths are stripped from the answer.",
		}
	case "EndpointCreate":
		return toolutil.ActionNarrative{
			Title: "Register an environment",
			Description: "Registers a new environment with Portainer and returns it. endpointCreationType decides which of the other fields apply and is the field to get right first: " +
				"1 local Docker (no url), 2 remote Docker or Swarm behind a Portainer agent, 3 Azure ACI (needs the three azure* fields), 4 edge agent, 5 local Kubernetes, 6 a cluster reached through a supplied kubeConfig. " +
				"Registering an agent environment (type 2) needs all three of tls, tlsSkipVerify and tlsSkipClientVerify set true, or Portainer answers 400 naming a certificate problem rather than a missing flag — the agent presents its own certificate and Portainer will not accept it otherwise. " +
				"Creating an edge environment (type 4) fails with \"API server URL not set in Edge Compute settings\" unless Edge Compute has already been enabled on the server, which is a settings change this catalog does not yet expose. " +
				"gpus and tagIds are JSON documents passed as strings on this route, not real arrays — that is the multipart body's own shape, and endpoints.update takes the same two as real structures. " +
				"The three tls*File fields take certificate and key content in PEM form, not a path on any filesystem. " +
				"Answers with the created environment, credentials stripped.",
		}
	case "EndpointUpdate":
		return toolutil.ActionNarrative{
			Title: "Update an environment",
			Description: "Changes an existing environment's name, url, group, tags, access policies or TLS settings and returns it. " +
				"Every field is optional and an omitted field is left alone, so this is a partial update rather than a replacement. " +
				"status moves the environment between 1 (up) and 2 (down) by hand, which is not the same as the environment actually being reachable — Portainer's own health check overwrites it on the next snapshot. " +
				"The kubernetes subtree carries the cluster's stored configuration and snapshots; sending it back unchanged is normal, and its four performanceMetrics values are refused rather than silently rounded if they cannot be sent exactly. " +
				"changeWindow, deploymentOptions, edge and statusMessage require Business Edition and are absent from a Community catalog. " +
				"Answers with the updated environment, credentials stripped.",
		}
	case "EndpointDelete":
		return toolutil.ActionNarrative{
			Title: "Delete an environment",
			Description: "Permanently removes one environment from Portainer, together with the access policies, tag assignments and stack records that referenced it. " +
				"This cannot be undone, and it is not a request to the host: containers, volumes and workloads on the environment itself keep running, Portainer simply stops managing them. " +
				"Re-registering the same host afterwards creates a new environment with a new id. " +
				"Use endpoints.association_delete instead to detach an edge agent while keeping the environment.",
		}
	case "EndpointDeleteBatch":
		return toolutil.ActionNarrative{
			Title: "Delete several environments",
			Description: "Permanently removes each environment named in endpoints, the same way endpoints.delete removes one, and cannot be undone. " +
				"deleteCluster additionally asks the cloud provider to tear down a cluster Portainer itself provisioned — for any other environment it does nothing, and for a provisioned one it destroys the cluster and everything on it. " +
				"Partial success is possible: the answer reports per-environment outcomes rather than failing the whole call, so read it rather than assuming every id in the request was removed.",
		}
	case "EndpointSettingsUpdate":
		return toolutil.ActionNarrative{
			Title: "Update an environment's settings",
			Description: "Changes the per-environment security and GPU settings — which container features non-administrators may use on this environment, and which GPUs Portainer exposes. " +
				"The security fields are the point of the action and are where Portainer's two editions genuinely disagree: Business Edition groups all ten under securitySettings, while Community Edition takes the identical ten as top-level fields. " +
				"This catalog is generated from the Business Edition specification, so a Community server sees only the fields the two editions share. " +
				"enableGPUManagement and gpus apply to both. changeWindow, deploymentOptions and enableImageNotification require Business Edition.",
		}
	case "EndpointAssociationDelete":
		return toolutil.ActionNarrative{
			Title: "Disassociate an edge environment",
			Description: "Detaches an edge agent from its environment and returns the environment to the waiting room, keeping the environment record, its group, tags and access policies intact. " +
				"It clears the edge identity: the agent still running on the remote host holds a key that no longer works, and reconnecting it means enrolling it again and trusting it through endpoints.trust_edge_endpoints. " +
				"Calling it twice is harmless — the second call finds nothing to detach — but the first call is not reversible from here. " +
				"Use endpoints.delete to remove the environment itself.",
		}
	case "EndpointCreateGlobalKey":
		return toolutil.ActionNarrative{
			Title: "Claim an environment for an edge ID",
			Description: "The enrolment route an edge agent calls on itself: it looks up the environment matching the agent's own EDGE_ID and creates one in the waiting room if none exists. " +
				"Portainer reads that identity from a request header the agent sets, which this action does not send and has no field for, so calling it from here reaches the route but cannot identify an agent. " +
				"It is published for completeness of the API surface. To enrol an edge agent, register the environment with endpoints.create (endpointCreationType 4) and trust it with endpoints.trust_edge_endpoints.",
		}
	case "TrustEdgeEndpoints":
		return toolutil.ActionNarrative{
			Title: "Trust edge environments in the waiting room",
			Description: "Grants trust to edge environments sitting in the waiting room, which is what lets their agents move from checking in to actually being managed. " +
				"Until an edge environment is trusted, Portainer records that it exists and refuses to deploy anything to it. Business Edition only. " +
				"The environments named here are ordinary environment identifiers from endpoints.list, not edge IDs.",
		}
	case "EndpointUpdateRelations":
		return toolutil.ActionNarrative{
			Title: "Reassign environments to groups, tags and edge groups",
			Description: "Changes group membership, tag assignments and edge-group membership for several environments in one call, keyed by environment identifier. " +
				"It replaces each named environment's relations rather than merging into them, so send the full intended set, not just the additions. " +
				"Environments not named in the request are untouched.",
		}
	case "EndpointSnapshot":
		return toolutil.ActionNarrative{
			Title: "Refresh one environment's snapshot",
			Description: "Asks Portainer to poll this one environment now and store a fresh snapshot of what runs on it, instead of waiting for the next scheduled poll. " +
				"It changes nothing on the environment itself; what it changes is what endpoints.inspect and endpoints.snapshot_inspect report afterwards. " +
				"A slow or unreachable host makes this call slow rather than making it fail quickly. " +
				"endpoints.snapshot_all is the same operation across every environment on the server.",
		}
	case "EndpointSnapshots":
		return toolutil.ActionNarrative{
			Title: "Refresh every environment's snapshot",
			Description: "Re-polls EVERY environment registered on this Portainer server and stores a fresh snapshot for each — not one environment, and it takes no identifier to narrow it. " +
				"On a large or partly-unreachable fleet this is slow and puts a request on every registered host at once. " +
				"endpoints.snapshot is the single-environment form and is almost always the one wanted.",
		}
	case "EndpointForceUpdateService":
		return toolutil.ActionNarrative{
			Title: "Force a Docker service to redeploy",
			Description: "Forces one Docker Swarm service to redeploy its tasks, the equivalent of `docker service update --force`. " +
				"With pullImage true it re-pulls the image tag first, which is how a service picks up a rebuilt image published under a tag it already uses. " +
				"serviceId is Docker's own service identifier, not a Portainer id — read it from the environment's snapshot. " +
				"Running tasks are replaced, so the service is briefly disrupted according to its own update configuration.",
		}
	case "EndpointRegistriesList":
		return toolutil.ActionNarrative{
			Title: "List the registries available on an environment",
			Description: "Returns the container registries this account may use when deploying to the named environment, which is a subset of registries.list narrowed by per-environment access. " +
				"Registry passwords, access tokens and management TLS material are stripped from every entry before you see them.",
		}
	case "EndpointRegistryAccess":
		return toolutil.ActionNarrative{
			Title: "Set which users and teams may use a registry on an environment",
			Description: "Replaces the per-environment access policies for one registry, naming the users and teams allowed to pull from it when deploying to this environment. " +
				"It replaces rather than merges: whoever is not in the request loses access. " +
				"registryId is a registry identifier from registries.list, and id is the environment. " +
				"This grants access to the registry on this environment only; the registry itself is unchanged.",
		}
	case "EndpointDockerhubStatus":
		return toolutil.ActionNarrative{
			Title: "Check Docker Hub pull-rate limits",
			Description: "Reports how many Docker Hub image pulls remain in the current rate-limit window, as seen from the named environment. " +
				"registryId names the Docker Hub registry configured in Portainer; 0 is the documented value for anonymous, unauthenticated Docker Hub and is the one that always resolves, which is why this parameter — alone among this catalog's registry identifiers — accepts it.",
		}
	case "EndpointSummaryCounts":
		return toolutil.ActionNarrative{
			Title: "Count environments by status",
			Description: "Returns counts rather than environments: how many are up, down, outdated and unassigned to any group, with breakdowns by group, type and health. " +
				"Cheaper than endpoints.list when the question is \"how is the fleet\" rather than \"which environments\".",
		}
	case "EndpointSetPolicyStatuses":
		return toolutil.ActionNarrative{
			Title: "Report edge policy reconciliation status",
			Description: "The route an edge agent calls to report back how each policy it was given reconciled on its own host. Business Edition only. " +
				"It is an agent-facing callback, not an administrative action: calling it from here writes the statuses Portainer will display for that environment, without anything on the host having changed. " +
				"Read the statuses rather than write them unless you are standing in for an agent.",
		}
	case "NamespacesAccessUpdate":
		return toolutil.ActionNarrative{
			Title: "Grant or revoke namespace access",
			Description: "Adds and removes the users and teams allowed to work in one Kubernetes namespace of one environment. Business Edition only. " +
				"Unlike most access actions in this catalog it is a delta, not a replacement: the four lists add and remove, and anyone not named keeps what they had. " +
				"A user or team must already have access to the environment before it can be given access to a namespace within it; otherwise Portainer refuses. " +
				"rpn is the namespace, and it is a number rather than the namespace's name.",
		}
	case "EndpointMTLSCertificate":
		return toolutil.ActionNarrative{
			Title: "Check an environment's mTLS certificate",
			Description: "Reports whether Portainer holds a mutual-TLS certificate for this environment's agent. Business Edition only. " +
				"The certificate itself is not returned: its fields are x509 key identifiers and usage flags that no caller here can act on, and the redaction this catalog applies to certificate-shaped responses admits no exceptions. " +
				"What survives is whether there is one.",
		}
	case "EndpointMTLSAgentCertificateError":
		return toolutil.ActionNarrative{
			Title: "Read the environment's mTLS certificate error",
			Description: "Reports the mutual-TLS certificate error recorded against this environment's agent, if there is one — this is the diagnostic route, and endpoints.mtls_certificate is the one that reports the certificate. Business Edition only. " +
				"The vendored specification gives this route the same summary and description as its sibling, which is wrong about which of the two this is; the route itself is /mtls_certificate_error. " +
				"As with the sibling, the certificate material is stripped from the answer.",
		}
	case "AgentVersions":
		return toolutil.ActionNarrative{
			Title: "List the agent versions in use",
			Description: "Returns the distinct Portainer agent versions currently reported across the environments this account can see. Business Edition only. " +
				"Use it to find out whether a fleet is on mixed versions; endpoints.list with outdated true is what then names the environments running an old one.",
		}
	case "SnapshotInspect":
		return toolutil.ActionNarrative{
			Title: "Read a Docker environment's stored snapshot",
			Description: "Returns Portainer's own stored snapshot of a Docker environment — the container, image, volume and network counts and the daemon information captured at the last poll. Business Edition only. " +
				"It reads what Portainer recorded, not the Docker daemon, so it is fast and can be stale; endpoints.snapshot refreshes it first. " +
				"Tagged both endpoints and docker upstream, and declared here because it reports on an environment rather than acting on a daemon.",
		}
	case "SnapshotContainersList":
		return toolutil.ActionNarrative{
			Title: "List containers from a stored snapshot",
			Description: "Returns the containers recorded in a Docker environment's last stored snapshot, without contacting the daemon. Business Edition only. " +
				"Fast and possibly stale, for the reason endpoints.snapshot_inspect gives. " +
				"edgeStackId narrows the list to the containers one edge stack deployed.",
		}
	case "SnapshotContainerInspect":
		return toolutil.ActionNarrative{
			Title: "Read one container from a stored snapshot",
			Description: "Returns one container out of a Docker environment's last stored snapshot, without contacting the daemon. Business Edition only. " +
				"containerId is Docker's own container identifier — the 64-character hex string, or the short form the daemon accepts — and is a string despite the vendored specification declaring it an integer, which it never is. " +
				"Fast and possibly stale, for the reason endpoints.snapshot_inspect gives.",
		}
	case "EndpointDockerBrowsePut":
		return toolutil.ActionNarrative{
			Title: "Upload a file into an environment's filesystem",
			Description: "Writes one file to a path on the environment's own filesystem, through the Portainer agent. Its documented purpose is placing TLS material where the agent can read it. " +
				"path is the destination on the host, and an existing file there is overwritten without warning. " +
				"volumeId writes into a named Docker volume instead of the host filesystem. " +
				"file carries the file's content, not a path on any machine this catalog can see, so only text can be uploaded through it.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}

// redactEndpoint strips every credential-shaped field from one environment.
//
// Three clusters, and the reason each is dropped whole rather than field by
// field is the same one internal/tools/registries's own redact records: a
// nested object whose fields are enumerated here invites the identical
// omission the next time Portainer adds one.
//
//   - AzureCredentials carries AuthenticationKey, the shared secret an Azure
//     ACI environment authenticates with. ApplicationID and TenantID beside it
//     are not secret, but they are useless without it and identify the
//     subscription, so the pointer is dropped rather than emptied.
//   - EdgeKey is the enrolment key an edge agent presents to claim this
//     environment. Anyone holding it can enrol an agent as this environment.
//   - TLSConfig's three cert/key *paths* disclose the server's filesystem
//     layout. TLS itself is a bool saying whether TLS is on, which is not
//     credential-shaped and is worth keeping.
func redactEndpoint(e *apigen.PortainereeEndpoint) *apigen.PortainereeEndpoint {
	if e == nil {
		return nil
	}
	scrubbed := *e
	scrubbed.AzureCredentials = nil
	scrubbed.EdgeKey = ""
	scrubbed.TLSConfig.TLSCACert = nil
	scrubbed.TLSConfig.TLSCert = nil
	scrubbed.TLSConfig.TLSKey = nil
	return &scrubbed
}

// redactEndpointResponse is redactEndpoint for the other environment shape.
//
// GET /endpoints answers with endpoints.endpointResponse where every other
// environment route answers with portainer.Endpoint. The two carry the same
// three credential clusters under the same names but are distinct generated
// types, so neither wrapper can be expressed in terms of the other without
// reflection.
func redactEndpointResponse(e *apigen.EndpointsEndpointResponse) *apigen.EndpointsEndpointResponse {
	if e == nil {
		return nil
	}
	scrubbed := *e
	scrubbed.AzureCredentials = nil
	scrubbed.EdgeKey = ""
	scrubbed.TLSConfig.TLSCACert = nil
	scrubbed.TLSConfig.TLSCert = nil
	scrubbed.TLSConfig.TLSKey = nil
	return &scrubbed
}

// redactRegistry strips the credentials from one registry as it appears on an
// environment's registry list.
//
// GET /endpoints/{id}/registries answers with the same portainer.Registry
// objects GET /registries does, credentials and all — this is the P2 registry
// leak re-exposed under an environment route, and it is redacted here the same
// way internal/tools/registries redacts it there. The two implementations are
// deliberately separate: an unexported helper cannot cross packages, and a
// shared one would tie two domains' redaction to a single edit.
func redactRegistry(r *apigen.PortainereeRegistry) *apigen.PortainereeRegistry {
	if r == nil {
		return nil
	}
	scrubbed := *r
	scrubbed.Password = nil
	scrubbed.AccessToken = nil
	if scrubbed.ManagementConfiguration != nil {
		config := *scrubbed.ManagementConfiguration
		config.Password = nil
		config.AccessToken = nil
		config.TLSConfig = nil
		scrubbed.ManagementConfiguration = &config
	}
	return &scrubbed
}

// redactEndpointList, redactEndpointInspect, redactEndpointCreate,
// redactEndpointUpdate, redactEndpointAssociationDelete,
// redactEndpointSettingsUpdate, redactEndpointRegistriesList,
// redactEndpointMTLSCertificate and redactEndpointMTLSAgentCertificateError
// are the nine redaction wrappers cmd/gen_action_inputs requires by name
// before it will emit a handler for any of these operations at all.
//
// Each takes the operation's real success-body type rather than `any`:
// docs/api-divergences.md §9.4 records that an `any`-typed wrapper satisfies
// its own generated guard vacuously — reflect.New of an interface type
// produces a nil interface, PopulateForCredentialAudit has nothing to fill,
// and AssertRedacted then finds nothing to complain about — and
// audit_spec_drift does not catch that either.
func redactEndpointList(es *[]apigen.EndpointsEndpointResponse) any {
	if es == nil {
		return nil
	}
	out := make([]apigen.EndpointsEndpointResponse, len(*es))
	for i := range *es {
		out[i] = *redactEndpointResponse(&(*es)[i])
	}
	return &out
}

func redactEndpointInspect(e *apigen.PortainereeEndpoint) any           { return redactEndpoint(e) }
func redactEndpointCreate(e *apigen.PortainereeEndpoint) any            { return redactEndpoint(e) }
func redactEndpointUpdate(e *apigen.PortainereeEndpoint) any            { return redactEndpoint(e) }
func redactEndpointAssociationDelete(e *apigen.PortainereeEndpoint) any { return redactEndpoint(e) }
func redactEndpointSettingsUpdate(e *apigen.PortainereeEndpoint) any    { return redactEndpoint(e) }

func redactEndpointRegistriesList(rs *[]apigen.PortainereeRegistry) any {
	if rs == nil {
		return nil
	}
	out := make([]apigen.PortainereeRegistry, len(*rs))
	for i := range *rs {
		out[i] = *redactRegistry(&(*rs)[i])
	}
	return &out
}

// redactMTLSCert drops the whole certificate rather than any field of it.
//
// The generator flags AuthorityKeyId, ExtendedKeyUsages,
// IssuingCertificateURLs, KeyUsages, PublicKey and SubjectKeyId on this
// response. Every one of them is ordinary x509 metadata off a *public*
// certificate, not a secret — SubjectKeyId and PublicKey in particular are
// exactly what a certificate exists to publish — so this is a
// name-rule false positive, six times over, not a real leak.
//
// It is still dropped whole, for a mechanical reason rather than a security
// one: toolutil.AssertRedacted (internal/toolutil/credential.go) walks the
// response with a nil skip map and admits no per-field exception, so the
// generated guard in redaction_test.go fails while any of the six is
// populated. Emptying them one by one would satisfy it today and silently
// stop satisfying it the moment the SslCertificate type gains a seventh
// credential-shaped name. Nothing of value is lost: a model has no use for
// the DER-encoded key identifiers of an agent's mTLS certificate, and
// endpoints.mtls_certificate_error's own job is to report *whether* there is
// a certificate error, which survives.
func redactMTLSCert(c *apigen.EndpointsEndpointMTLSCertResponse) *apigen.EndpointsEndpointMTLSCertResponse {
	if c == nil {
		return nil
	}
	scrubbed := *c
	scrubbed.MTLSCertificate = nil
	return &scrubbed
}

func redactEndpointMTLSCertificate(c *apigen.EndpointsEndpointMTLSCertResponse) any {
	return redactMTLSCert(c)
}

func redactEndpointMTLSAgentCertificateError(c *apigen.EndpointsEndpointMTLSCertResponse) any {
	return redactMTLSCert(c)
}
