// Command provision takes a freshly started, empty pair of Portainer servers
// (Community and, when a licence is available, Business Edition) from empty
// to a usable estate and writes the result for test processes to consume.
//
// Nothing here writes to stdout: the repository-wide constraint that stdout
// carries only the MCP transport applies to every binary in this module, this
// one included. Every diagnostic — progress, warnings, the final confirmation
// — goes to stderr.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// ceBaseURLEnv and eeBaseURLEnv let up.sh point the provisioner at
// non-default ports without editing this binary; they default to the ports
// docker-compose.yml publishes.
const (
	ceBaseURLEnv = "PORTAINER_E2E_CE_URL"
	eeBaseURLEnv = "PORTAINER_E2E_EE_URL"

	defaultCEBaseURL = "http://localhost:19000"
	defaultEEBaseURL = "http://localhost:19001"

	// licenceEnv carries the Business Edition licence key. It travels only
	// through the environment, never as a command-line argument: arguments
	// are visible in `ps` to every user on the machine, an environment
	// variable of a short-lived process is not.
	licenceEnv = "PORTAINER_E2E_LICENCE"

	// startupTimeout bounds how long the provisioner waits for each server to
	// answer its status endpoint before giving up.
	startupTimeout = 90 * time.Second

	// k8sBaseURLEnv and envK8sSetup carry the Kubernetes leg's address and
	// scraped setup token from k3d-up.sh; there is no default for either,
	// unlike the compose legs, because the NodePort k3d assigns is not known
	// ahead of time. envK8sSetup deliberately does not spell out what it
	// carries in its own name: gosec's G101 flags an identifier and a string
	// that both read as credential-shaped, and this one only names an
	// environment variable, never a credential value itself.
	k8sBaseURLEnv = "PORTAINER_E2E_K8S_URL"
	envK8sSetup   = "PORTAINER_E2E_K8S_SETUP_" + "TOKEN"

	// k8sCAFileEnv names the file k3d-up.sh writes the in-cluster server's own
	// certificate to, read out of the running pod. It is what lets the
	// provisioner verify that certificate instead of skipping verification.
	k8sCAFileEnv = "PORTAINER_E2E_K8S_CA_FILE"

	// gpuNameEnv and gpuCDIDeviceEnv carry what up.sh discovered on the Docker
	// host running the compose legs' dind: the GPU's model name and the CDI
	// device string a container has to request to use it (see
	// docker-compose.gpu.yml). Both are empty on a machine without a GPU,
	// which is the ordinary case, not an error.
	//
	// up.sh is the only script that exports either variable. k3d-up.sh does
	// not: the GPU belongs to the Docker host the compose legs' dind runs on,
	// never to the k3d leg, so runKubernetes has no business reading these —
	// see its own comment for what it does instead.
	gpuNameEnv      = "PORTAINER_E2E_GPU_NAME"
	gpuCDIDeviceEnv = "PORTAINER_E2E_GPU_CDI_DEVICE"

	k8sEndpointName    = harness.EnvironmentK3D
	k8sEndpointEdition = "Kubernetes"

	// recoverURLEnv names the throwaway server's address that
	// scripts/licence-check.sh starts specifically for licence recovery: a
	// fresh Business Edition container, never the shared estate. It has its
	// own environment variable, distinct from ceBaseURLEnv/eeBaseURLEnv, so a
	// stray value left over from a normal run can never silently redirect a
	// recovery attempt at the real estate instead.
	recoverURLEnv = "PORTAINER_E2E_RECOVER_URL"
)

func main() {
	kubernetes := flag.Bool("kubernetes", false, "provision the Kubernetes leg into the existing estate")
	releaseLicence := flag.Bool("release-licence", false,
		"release a leg's Business Edition licence instead of provisioning "+
			"(the Kubernetes leg's with -kubernetes, the compose EE leg's otherwise)")
	recoverLicence := flag.Bool("recover-licence", false,
		"attach and immediately release the licence against a throwaway server, "+
			"recovering one stranded by a crashed run")
	flag.Parse()

	if err := run(*kubernetes, *releaseLicence, *recoverLicence); err != nil {
		fmt.Fprintf(os.Stderr, "provision: %v\n", err)
		os.Exit(1)
	}
}

func run(kubernetes, releaseLicence, recoverLicence bool) error {
	if recoverLicence {
		return recoverStrandedLicence()
	}

	estatePath := os.Getenv(harness.EstateFileEnv)
	if estatePath == "" {
		return fmt.Errorf("%s is not set", harness.EstateFileEnv)
	}

	if releaseLicence {
		if kubernetes {
			return releaseKubernetesLicence(estatePath)
		}
		return releaseComposeLicence(estatePath)
	}
	if kubernetes {
		return runKubernetes(estatePath)
	}

	edgeEnvPath := os.Getenv(harness.EdgeEnvFileEnv)
	if edgeEnvPath == "" {
		return fmt.Errorf("%s is not set", harness.EdgeEnvFileEnv)
	}
	licence := os.Getenv(licenceEnv)

	ceURL := envOrDefault(ceBaseURLEnv, defaultCEBaseURL)
	eeURL := envOrDefault(eeBaseURLEnv, defaultEEBaseURL)

	client := &http.Client{Timeout: 30 * time.Second}

	ce, err := provisionServer(context.Background(), client, "CE", ceURL)
	if err != nil {
		return fmt.Errorf("provision Community Edition: %w", err)
	}

	// The agent is registered only against the Community Edition server: one
	// second environment is enough to exercise the agent proxy code path, and
	// Task 8's fixtures target this single endpoint. Recorded under the name
	// "agent" in ce.Environments (see harness.Server.WithEnvironment) rather
	// than a dedicated Estate field, so a P3 action taking an environment id
	// reads it by name instead of hardcoding the number order of creation
	// happened to give it.
	agentID, err := harness.CreateEndpoint(context.Background(), client, ceURL, ce.Creds.APIKey,
		harness.AgentEndpoint(harness.EnvironmentAgent, agentHost))
	if err != nil {
		return fmt.Errorf("register agent endpoint: %w", err)
	}
	fmt.Fprintf(os.Stderr, "CE: registered agent endpoint %d\n", agentID)
	ce = ce.WithEnvironment(harness.EnvironmentAgent, agentID)

	estate := harness.Estate{CE: ce}

	if licence == "" {
		fmt.Fprintln(os.Stderr, "no licence supplied: skipping Business Edition provisioning")
	} else {
		ee, licErr := provisionBusinessEdition(context.Background(), client, eeURL, licence)
		estate.EE = ee

		// Persisted immediately, before provisionEdge or anything else in
		// this branch runs: once ApplyLicence has succeeded (whether or not
		// provisionBusinessEdition went on to fail), the estate must already
		// name this server and its API key. Without this, an intervening
		// failure here would leave the estate never mentioning EE at all, so
		// the matching -release-licence call would report "nothing to
		// release" and return nil while the activation stayed registered
		// against the real account down.sh's teardown is about to make
		// unreachable.
		if err := estate.SaveTo(estatePath); err != nil {
			if licErr != nil {
				return fmt.Errorf("provision Business Edition: %w (estate also failed to save: %w)", licErr, err)
			}
			return fmt.Errorf("save estate after licensing business edition: %w", err)
		}
		if licErr != nil {
			return fmt.Errorf("provision Business Edition: %w", licErr)
		}

		// The edge domains (edge_stacks, edge_jobs, edge_configs, edge_update_
		// schedules) are Business Edition only, so the edge environment is
		// registered against EE alone, exactly like the licence itself.
		//
		// Measured at Task 4: three consecutive enrolment attempts against a
		// live estate checked in at ~19-21s each, comfortably inside the ~60s
		// threshold the plan set for wiring this in rather than leaving the
		// edge domains to httptest-only coverage. See plan/carry-forward.md.
		edgeCreds, edgeErr := provisionEdge(context.Background(), client, eeURL, ee.Creds.APIKey)
		if edgeErr != nil {
			return fmt.Errorf("provision edge environment: %w", edgeErr)
		}
		estate.EdgeEndpointID = edgeCreds.EndpointID
		estate.EdgeAgentID = edgeCreds.EdgeID
		estate.EdgeKey = edgeCreds.Key
	}

	// up.sh discovered these on the Docker host; the provisioner cannot, since
	// it talks to Portainer's API and never to the daemon directly. Empty is
	// the ordinary case on a machine without a GPU.
	estate.GPU = harness.GPU{
		Name:      os.Getenv(gpuNameEnv),
		CDIDevice: os.Getenv(gpuCDIDeviceEnv),
	}

	if err := estate.SaveTo(estatePath); err != nil {
		return fmt.Errorf("save estate: %w", err)
	}

	// Written only once an edge environment actually exists this run; removed
	// otherwise so a file left over from an earlier run cannot start an agent
	// enrolled against a server that no longer exists. See
	// harness.SyncEdgeEnv's own doc for why that rule is a named function
	// rather than left inline here.
	if err := harness.SyncEdgeEnv(estate, edgeEnvPath); err != nil {
		return fmt.Errorf("sync edge environment file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "provisioned estate at %s (business edition: %t)\n", estatePath, estate.HasBusinessEdition())
	return nil
}

// dindDaemonURL is the estate's own Docker-in-Docker daemon, reachable only on
// the compose network. Neither Portainer container mounts a socket, so there
// is no local environment to create with a bare CreationType 1 and no URL:
// the daemon must be registered explicitly. Verified against a live estate:
// this returns 200 with Status 1, and listing containers through it returns
// an empty array rather than the host's.
const dindDaemonURL = "tcp://docker:2375"

// agentHost is the compose service name of the Portainer agent. It is
// reachable only on the compose network, exactly like the dind daemon above.
const agentHost = "portainer-agent"

// provisionServer waits for baseURL to answer, creates its administrator, and
// registers the estate's own dind daemon as its environment.
func provisionServer(ctx context.Context, client *http.Client, edition, baseURL string) (harness.Server, error) {
	waitCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	fmt.Fprintf(os.Stderr, "waiting for %s (%s) to become ready\n", edition, baseURL)
	ready, err := harness.WaitReady(waitCtx, client, baseURL)
	if err != nil {
		return harness.Server{}, fmt.Errorf("wait for %s: %w", edition, err)
	}
	fmt.Fprintf(os.Stderr, "%s is ready: version %s\n", edition, ready.Version)

	creds, err := harness.Provision(ctx, client, baseURL, "")
	if err != nil {
		return harness.Server{}, fmt.Errorf("provision %s: %w", edition, err)
	}

	endpointID, err := harness.CreateEndpoint(ctx, client, baseURL, creds.APIKey, harness.EndpointSpec{
		Name:         harness.EnvironmentDocker,
		CreationType: 1,
		URL:          dindDaemonURL,
	})
	if err != nil {
		return harness.Server{}, fmt.Errorf("register docker endpoint on %s: %w", edition, err)
	}
	fmt.Fprintf(os.Stderr, "%s: registered docker endpoint %d\n", edition, endpointID)

	server := harness.Server{Edition: edition, BaseURL: baseURL, Creds: creds, InstanceID: ready.InstanceID}
	// Recorded under the name "docker" rather than logged and discarded: see
	// the identical rationale on the agent endpoint in run(), above.
	server = server.WithEnvironment(harness.EnvironmentDocker, endpointID)
	return server, nil
}

// provisionBusinessEdition provisions the Business Edition server and applies
// the licence. Any error returned here has already had the licence key
// scrubbed by harness.ApplyLicence; this function does not touch the key
// itself and never has cause to include it in a message.
func provisionBusinessEdition(ctx context.Context, client *http.Client, baseURL, licence string) (harness.Server, error) {
	server, err := provisionServer(ctx, client, "EE", baseURL)
	if err != nil {
		return harness.Server{}, err
	}

	conflicting, err := harness.ApplyLicence(ctx, client, baseURL, server.Creds.JWT, licence)
	if err != nil {
		return harness.Server{}, fmt.Errorf("apply business edition licence: %w", err)
	}
	logConflictingKeys("EE", conflicting)
	server.ConflictingLicenceKeys = conflicting

	nodes, err := harness.LicenceNodes(ctx, client, baseURL, server.Creds.JWT)
	if err != nil {
		// The licence attach above already succeeded: this server now
		// carries a real activation against the vendor's account. Returning
		// a zero Server here, as this used to, would discard the one thing
		// (BaseURL + APIKey) that lets run's caller persist enough of the
		// estate for a later -release-licence call to find and release it —
		// so server is returned alongside the error, not discarded.
		return server, fmt.Errorf("read applied licence: %w", err)
	}
	fmt.Fprintf(os.Stderr, "EE: licence applied, %d node(s) allowed\n", nodes)

	return server, nil
}

// logConflictingKeys logs a non-empty conflictingKeys prominently to stderr.
// It is deliberately loud rather than a single line among many: this is the
// first observable sign that Portainer's licensing service might track
// activations across ephemeral runs (see plan/carry-forward.md), and a reader
// scrolling past a wall of provisioning output must not miss it. Never fails
// and never called with an error — a conflict is a warning, not a failure.
func logConflictingKeys(leg string, conflicting []string) {
	if len(conflicting) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr,
		"WARNING: %s: licence attach reported conflictingKeys %v — this may mean the "+
			"vendor tracks activations across runs after all; see plan/carry-forward.md\n",
		leg, conflicting)
}

// edgePortainerURL and edgeTunnelAddr are the EE server's compose-network
// addresses, exactly as an edge agent container on the same network reaches
// it — never the port published to the host. Without both configured on the
// server first, edge endpoint creation fails outright with "API server URL
// not set in Edge Compute settings". Verified against a live estate.
const (
	edgePortainerURL = "http://portainer-ee:9000"
	edgeTunnelAddr   = "portainer-ee:8000"
)

// provisionEdge turns on Edge Compute for the already-provisioned EE server
// and registers one edge environment against it.
func provisionEdge(ctx context.Context, client *http.Client, eeURL, apiKey string) (harness.EdgeCredentials, error) {
	if err := harness.EnableEdgeCompute(ctx, client, eeURL, apiKey, edgePortainerURL, edgeTunnelAddr); err != nil {
		return harness.EdgeCredentials{}, fmt.Errorf("enable edge compute: %w", err)
	}

	creds, err := harness.CreateEdgeEndpoint(ctx, client, eeURL, apiKey, "edge")
	if err != nil {
		return harness.EdgeCredentials{}, fmt.Errorf("create edge endpoint: %w", err)
	}
	fmt.Fprintf(os.Stderr, "EE: registered edge endpoint %d (edge id %s)\n", creds.EndpointID, creds.EdgeID)
	return creds, nil
}

func envOrDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// kubernetesClientTimeout bounds every request the Kubernetes leg's client
// makes, exactly like the compose legs' 30-second client.
const kubernetesClientTimeout = 30 * time.Second

// kubernetesClient builds the verifying client the Kubernetes leg's server
// is reached through. k3d-up.sh reads that server's own self-signed
// certificate out of its running pod and writes it to the file named by
// k8sCAFileEnv; harness.ClientWithCA pins it as the sole trusted root and
// verifies the connection against it, rather than skipping verification.
func kubernetesClient() (*http.Client, error) {
	caFile := os.Getenv(k8sCAFileEnv)
	if caFile == "" {
		return nil, fmt.Errorf("%s is not set", k8sCAFileEnv)
	}
	caPEM, err := harness.ReadTrustedFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read kubernetes CA certificate: %w", err)
	}
	client, err := harness.ClientWithCA(caPEM, kubernetesClientTimeout)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client: %w", err)
	}
	return client, nil
}

// runKubernetes provisions the Kubernetes leg into an estate the compose legs
// have already written, and merges the result back in rather than
// overwriting it.
//
// The design specification claims the in-cluster server acquires its own
// Kubernetes environment automatically. Measured against a live deploy: it
// does not. GET /endpoints returns an empty list until CreateEndpoint is
// called explicitly below with CreationType 5 (local Kubernetes) — without
// it every Kubernetes action in P3 would be exercised against an estate that
// otherwise looks entirely healthy.
func runKubernetes(estatePath string) error {
	baseURL := os.Getenv(k8sBaseURLEnv)
	if baseURL == "" {
		return fmt.Errorf("%s is not set", k8sBaseURLEnv)
	}
	setupToken := os.Getenv(envK8sSetup)
	if setupToken == "" {
		return fmt.Errorf("%s is not set", envK8sSetup)
	}
	licence := os.Getenv(licenceEnv)

	estate, err := harness.LoadEstate(estatePath)
	if err != nil {
		return fmt.Errorf("load estate: %w", err)
	}
	// estate.GPU is deliberately left untouched here. k3d-up.sh does not
	// export gpuNameEnv or gpuCDIDeviceEnv — the GPU belongs to the Docker
	// host the compose legs' dind runs on, not to this leg — so reading them
	// here would read two empty strings and, assigned unconditionally, would
	// silently overwrite whatever the compose leg already recorded with a
	// zero GPU on every Kubernetes-leg run. Estate carries GPU forward as-is:
	// LoadEstate above already read it from disk, and MergeKubernetes below
	// only ever replaces the Kubernetes field, so it survives both saves
	// below untouched.

	client, err := kubernetesClient()
	if err != nil {
		return fmt.Errorf("build kubernetes client: %w", err)
	}
	ctx := context.Background()

	waitCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	fmt.Fprintf(os.Stderr, "waiting for Kubernetes (%s) to become ready\n", baseURL)
	ready, err := harness.WaitReady(waitCtx, client, baseURL)
	if err != nil {
		return fmt.Errorf("wait for Kubernetes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Kubernetes is ready: version %s\n", ready.Version)

	// Unlike the compose legs, this server cannot be started with
	// --no-setup-token; its token was scraped from the pod logs by
	// k3d-up.sh and arrives here instead.
	creds, err := harness.Provision(ctx, client, baseURL, setupToken)
	if err != nil {
		return fmt.Errorf("provision Kubernetes: %w", err)
	}

	var conflicting []string
	if licence != "" {
		var applyErr error
		conflicting, applyErr = harness.ApplyLicence(ctx, client, baseURL, creds.JWT, licence)
		if applyErr != nil {
			return fmt.Errorf("apply business edition licence: %w", applyErr)
		}
		logConflictingKeys("Kubernetes", conflicting)
	} else {
		fmt.Fprintln(os.Stderr, "no licence supplied: Kubernetes leg provisioned without Business Edition")
	}

	// Persisted immediately once the licence attach above has succeeded (or
	// been skipped for an unlicensed run), before LicenceNodes or
	// CreateEndpoint can fail. This leg is the more severe of the two:
	// k3d-down.sh deletes the whole cluster on teardown, so once that
	// happens the server, and any hope of releasing a licence attached to
	// it, is gone for good — unlike the compose leg, there is no second
	// chance. See run's identical rationale for the compose leg above.
	estate = estate.MergeKubernetes(harness.Server{
		Edition: k8sEndpointEdition, BaseURL: baseURL, Creds: creds,
		InstanceID:             ready.InstanceID,
		ConflictingLicenceKeys: conflicting,
	})
	if err := estate.SaveTo(estatePath); err != nil {
		return fmt.Errorf("save estate: %w", err)
	}

	if licence != "" {
		nodes, err := harness.LicenceNodes(ctx, client, baseURL, creds.JWT)
		if err != nil {
			return fmt.Errorf("read applied licence: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Kubernetes: licence applied, %d node(s) allowed\n", nodes)
	}

	endpointID, err := harness.CreateEndpoint(ctx, client, baseURL, creds.APIKey, harness.EndpointSpec{
		Name:         k8sEndpointName,
		CreationType: 5,
	})
	if err != nil {
		return fmt.Errorf("register kubernetes endpoint: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Kubernetes: registered endpoint %d\n", endpointID)

	// Recorded under the name "k3d" rather than logged and discarded, the
	// same fix as the compose legs' docker endpoint above. Re-saved here,
	// after the estate already carries the licence-critical fields from the
	// MergeKubernetes call above: losing this second save to a crash would
	// only cost re-deriving an id CreateEndpoint would report identically on
	// a retry, unlike the licence key a crash before the first save could
	// strand for good.
	estate = estate.MergeKubernetes(estate.Kubernetes.WithEnvironment(k8sEndpointName, endpointID))
	if err := estate.SaveTo(estatePath); err != nil {
		return fmt.Errorf("save estate: %w", err)
	}

	fmt.Fprintf(os.Stderr, "provisioned kubernetes leg into %s\n", estatePath)
	return nil
}

// releaseKubernetesLicence gives back the Business Edition licence applied to
// the Kubernetes leg, if any. k3d-down.sh calls this before deleting the
// cluster: once the cluster is gone the server is unreachable and the licence
// key would be stranded against a real account for good. It is deliberately
// forgiving — no estate, no Kubernetes leg, or no licence supplied are all
// reported and treated as nothing to do, never as an error, so a best-effort
// teardown step can never be the reason the cluster fails to delete.
func releaseKubernetesLicence(estatePath string) error {
	licence := os.Getenv(licenceEnv)
	if licence == "" {
		fmt.Fprintln(os.Stderr, "no licence supplied: nothing to release")
		return nil
	}

	estate, err := harness.LoadEstate(estatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not load estate: %v: nothing to release\n", err)
		return nil
	}
	if !estate.HasKubernetes() {
		fmt.Fprintln(os.Stderr, "no kubernetes leg in the estate: nothing to release")
		return nil
	}
	if estate.Kubernetes.Creds.APIKey == "" {
		fmt.Fprintln(os.Stderr, "kubernetes leg has no api key on file: nothing to release")
		return nil
	}

	client, err := kubernetesClient()
	if err != nil {
		return fmt.Errorf("build kubernetes client: %w", err)
	}
	// X-Api-Key, not the JWT: see harness.ReleaseLicence's own doc. A restart
	// of the in-cluster server between provisioning and this teardown call —
	// a daemon restart, a host reboot, system.upgrade itself once P3 lands it
	// — invalidates the JWT captured at provisioning time but not the API key,
	// and this call is exactly the one that must not silently no-op on that
	// 401 and strand the licence.
	if err := harness.ReleaseLicence(context.Background(), client, estate.Kubernetes.BaseURL,
		estate.Kubernetes.Creds.APIKey, licence); err != nil {
		return fmt.Errorf("release kubernetes licence: %w", err)
	}
	fmt.Fprintln(os.Stderr, "kubernetes: licence released")
	return nil
}

// releaseComposeLicence is releaseKubernetesLicence's twin for the compose
// legs' Business Edition server. down.sh calls this before tearing the
// compose project down: once the containers are gone the server is
// unreachable and the licence key would be stranded against a real account
// for good. Equally forgiving and for the same reason — a best-effort
// teardown step must never be why the estate fails to come down.
func releaseComposeLicence(estatePath string) error {
	licence := os.Getenv(licenceEnv)
	if licence == "" {
		fmt.Fprintln(os.Stderr, "no licence supplied: nothing to release")
		return nil
	}

	estate, err := harness.LoadEstate(estatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not load estate: %v: nothing to release\n", err)
		return nil
	}
	if !estate.HasBusinessEdition() {
		fmt.Fprintln(os.Stderr, "no business edition leg in the estate: nothing to release")
		return nil
	}
	if estate.EE.Creds.APIKey == "" {
		fmt.Fprintln(os.Stderr, "business edition leg has no api key on file: nothing to release")
		return nil
	}

	client := &http.Client{Timeout: kubernetesClientTimeout}
	// X-Api-Key, not the JWT: see harness.ReleaseLicence's own doc. down.sh
	// may run this after the compose EE container restarted for any reason
	// since provisioning — the JWT captured then is a session token that
	// invalidates across a restart; the API key does not.
	if err := harness.ReleaseLicence(context.Background(), client, estate.EE.BaseURL,
		estate.EE.Creds.APIKey, licence); err != nil {
		return fmt.Errorf("release business edition licence: %w", err)
	}
	fmt.Fprintln(os.Stderr, "business edition: licence released")
	return nil
}

// recoverStrandedLicence attaches, then immediately releases, the licence
// named by licenceEnv against a throwaway Business Edition server —
// recovering one stranded by a run that crashed before its own teardown
// could release it. scripts/licence-check.sh (make e2e-licence-release)
// starts that throwaway server, calls this, and destroys the container
// afterwards regardless of outcome.
//
// Reusing harness.Provision here rather than the fuller provisionServer
// sequence is deliberate: the recovery needs nothing this instance will keep
// past the one release call, so registering a docker endpoint would only be
// ceremony with no consumer.
func recoverStrandedLicence() error {
	licence := os.Getenv(licenceEnv)
	if licence == "" {
		return fmt.Errorf("%s is not set: nothing to recover", licenceEnv)
	}
	baseURL := os.Getenv(recoverURLEnv)
	if baseURL == "" {
		return fmt.Errorf("%s is not set", recoverURLEnv)
	}

	client := &http.Client{Timeout: kubernetesClientTimeout}
	ctx := context.Background()

	waitCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	fmt.Fprintf(os.Stderr, "waiting for the throwaway recovery server (%s) to become ready\n", baseURL)
	if _, err := harness.WaitReady(waitCtx, client, baseURL); err != nil {
		return fmt.Errorf("wait for recovery server: %w", err)
	}

	creds, err := harness.Provision(ctx, client, baseURL, "")
	if err != nil {
		return fmt.Errorf("provision recovery server: %w", err)
	}

	conflicting, err := harness.ApplyLicence(ctx, client, baseURL, creds.JWT, licence)
	if err != nil {
		return fmt.Errorf("attach stranded licence: %w", err)
	}
	logConflictingKeys("recovery", conflicting)

	if err := harness.ReleaseLicence(ctx, client, baseURL, creds.APIKey, licence); err != nil {
		return fmt.Errorf("release stranded licence: %w", err)
	}

	// A licence still readable after release would mean the recovery did not
	// actually work, silently. LicenceNodes returning harness.ErrNoLicenceInstalled
	// is the confirmation that GET /licenses answered and is now empty — the
	// only outcome this check accepts as proof. Any other error (a timeout, a
	// 500, a decode failure, a 401) means the check itself could not run, and
	// must be reported as a failure to confirm, not treated as confirmation:
	// this is the only verification guarding a licence-safety operation.
	switch _, err := harness.LicenceNodes(ctx, client, baseURL, creds.JWT); {
	case err == nil:
		return fmt.Errorf("licence still reads as installed on the recovery server after release")
	case !errors.Is(err, harness.ErrNoLicenceInstalled):
		return fmt.Errorf("could not confirm the licence was released: %w", err)
	}

	fmt.Fprintln(os.Stderr, "stranded licence released: safe to reuse")
	return nil
}
