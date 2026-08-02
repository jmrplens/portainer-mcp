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
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "provision: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	estatePath := os.Getenv(harness.EstateFileEnv)
	if estatePath == "" {
		return fmt.Errorf("%s is not set", harness.EstateFileEnv)
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
