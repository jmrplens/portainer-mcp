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
	"crypto/tls"
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

	// k8sBaseURLEnv and k8sSetupTokenEnv carry the Kubernetes leg's address and
	// scraped setup token from k3d-up.sh; there is no default for either,
	// unlike the compose legs, because the NodePort k3d assigns is not known
	// ahead of time.
	k8sBaseURLEnv      = "PORTAINER_E2E_K8S_URL"
	k8sSetupTokenEnv   = "PORTAINER_E2E_K8S_SETUP_TOKEN"
	k8sEndpointName    = "k3d"
	k8sEndpointEdition = "Kubernetes"
)

func main() {
	kubernetes := flag.Bool("kubernetes", false, "provision the Kubernetes leg into the existing estate")
	releaseLicence := flag.Bool("release-licence", false,
		"release the Kubernetes leg's Business Edition licence instead of provisioning")
	flag.Parse()

	if err := run(*kubernetes, *releaseLicence); err != nil {
		fmt.Fprintf(os.Stderr, "provision: %v\n", err)
		os.Exit(1)
	}
}

func run(kubernetes, releaseLicence bool) error {
	estatePath := os.Getenv(harness.EstateFileEnv)
	if estatePath == "" {
		return fmt.Errorf("%s is not set", harness.EstateFileEnv)
	}

	if releaseLicence {
		return releaseKubernetesLicence(estatePath)
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
	// Task 8's fixtures target this single endpoint.
	agentID, err := harness.CreateEndpoint(context.Background(), client, ceURL, ce.Creds.APIKey,
		harness.AgentEndpoint("agent", agentHost))
	if err != nil {
		return fmt.Errorf("register agent endpoint: %w", err)
	}
	fmt.Fprintf(os.Stderr, "CE: registered agent endpoint %d\n", agentID)

	estate := harness.Estate{CE: ce, AgentID: agentID}

	if licence == "" {
		fmt.Fprintln(os.Stderr, "no licence supplied: skipping Business Edition provisioning")
	} else {
		ee, licErr := provisionBusinessEdition(context.Background(), client, eeURL, licence)
		if licErr != nil {
			return fmt.Errorf("provision Business Edition: %w", licErr)
		}
		estate.EE = ee

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

	if err := estate.SaveTo(estatePath); err != nil {
		return fmt.Errorf("save estate: %w", err)
	}

	// Written only once an edge environment actually exists this run; removed
	// otherwise so a file left over from an earlier run cannot start an agent
	// enrolled against a server that no longer exists.
	if estate.EdgeAgentID != "" && estate.EdgeKey != "" {
		if err := harness.WriteEdgeEnv(edgeEnvPath, estate.EdgeAgentID, estate.EdgeKey, estate.EdgeEndpointID); err != nil {
			return fmt.Errorf("write edge environment file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "wrote edge environment file at %s\n", edgeEnvPath)
	} else if err := harness.RemoveEdgeEnv(edgeEnvPath); err != nil {
		return fmt.Errorf("remove stale edge environment file: %w", err)
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
	version, err := harness.WaitReady(waitCtx, client, baseURL)
	if err != nil {
		return harness.Server{}, fmt.Errorf("wait for %s: %w", edition, err)
	}
	fmt.Fprintf(os.Stderr, "%s is ready: version %s\n", edition, version)

	creds, err := harness.Provision(ctx, client, baseURL, "")
	if err != nil {
		return harness.Server{}, fmt.Errorf("provision %s: %w", edition, err)
	}

	endpointID, err := harness.CreateEndpoint(ctx, client, baseURL, creds.APIKey, harness.EndpointSpec{
		Name:         "docker",
		CreationType: 1,
		URL:          dindDaemonURL,
	})
	if err != nil {
		return harness.Server{}, fmt.Errorf("register docker endpoint on %s: %w", edition, err)
	}
	fmt.Fprintf(os.Stderr, "%s: registered docker endpoint %d\n", edition, endpointID)

	return harness.Server{Edition: edition, BaseURL: baseURL, Creds: creds}, nil
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

	if err := harness.ApplyLicence(ctx, client, baseURL, server.Creds.JWT, licence); err != nil {
		return harness.Server{}, fmt.Errorf("apply business edition licence: %w", err)
	}

	nodes, err := harness.LicenceNodes(ctx, client, baseURL, server.Creds.JWT)
	if err != nil {
		return harness.Server{}, fmt.Errorf("read applied licence: %w", err)
	}
	fmt.Fprintf(os.Stderr, "EE: licence applied, %d node(s) allowed\n", nodes)

	return server, nil
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

// insecureClient is a client for the Kubernetes leg alone: the Helm chart's
// certificate is self-signed, and verifying it would just mean failing every
// request. The bypass is scoped to a freshly built Transport on a client used
// only here, never a mutation of http.DefaultTransport, which would silently
// stop verifying certificates for every other request this process makes.
func insecureClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Deliberate: the Helm chart's certificate is self-signed. gosec's G402 is
	// excluded for this file in .golangci.yml rather than suppressed with an
	// inline directive, alongside the one authorised such directive in
	// internal/portainer/client.go.
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
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
	setupToken := os.Getenv(k8sSetupTokenEnv)
	if setupToken == "" {
		return fmt.Errorf("%s is not set", k8sSetupTokenEnv)
	}
	licence := os.Getenv(licenceEnv)

	estate, err := harness.LoadEstate(estatePath)
	if err != nil {
		return fmt.Errorf("load estate: %w", err)
	}

	client := insecureClient()
	ctx := context.Background()

	waitCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	fmt.Fprintf(os.Stderr, "waiting for Kubernetes (%s) to become ready\n", baseURL)
	version, err := harness.WaitReady(waitCtx, client, baseURL)
	if err != nil {
		return fmt.Errorf("wait for Kubernetes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Kubernetes is ready: version %s\n", version)

	// Unlike the compose legs, this server cannot be started with
	// --no-setup-token; its token was scraped from the pod logs by
	// k3d-up.sh and arrives here instead.
	creds, err := harness.Provision(ctx, client, baseURL, setupToken)
	if err != nil {
		return fmt.Errorf("provision Kubernetes: %w", err)
	}

	if licence == "" {
		fmt.Fprintln(os.Stderr, "no licence supplied: Kubernetes leg provisioned without Business Edition")
	} else {
		if err := harness.ApplyLicence(ctx, client, baseURL, creds.JWT, licence); err != nil {
			return fmt.Errorf("apply business edition licence: %w", err)
		}
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

	estate.Kubernetes = harness.Server{Edition: k8sEndpointEdition, BaseURL: baseURL, Creds: creds}
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
	if estate.Kubernetes.Creds.JWT == "" {
		fmt.Fprintln(os.Stderr, "kubernetes leg has no jwt on file: nothing to release")
		return nil
	}

	client := insecureClient()
	if err := harness.ReleaseLicence(context.Background(), client, estate.Kubernetes.BaseURL,
		estate.Kubernetes.Creds.JWT, licence); err != nil {
		return fmt.Errorf("release kubernetes licence: %w", err)
	}
	fmt.Fprintln(os.Stderr, "kubernetes: licence released")
	return nil
}
