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

	estate := harness.Estate{CE: ce}

	if licence == "" {
		fmt.Fprintln(os.Stderr, "no licence supplied: skipping Business Edition provisioning")
	} else {
		ee, licErr := provisionBusinessEdition(context.Background(), client, eeURL, licence)
		if licErr != nil {
			return fmt.Errorf("provision Business Edition: %w", licErr)
		}
		estate.EE = ee
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

func envOrDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
