// Package harness brings up and provisions an ephemeral Portainer estate for
// end-to-end tests.
//
// Everything here talks to Portainer over plain net/http rather than through
// internal/portainer or the generated client. That is deliberate: the harness
// is what proves the client works, so a defect in the client must not be able
// to hide inside the thing that sets up its own test.
package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// pollInterval is short because a container reaches ready in about a second.
// The cost of polling quickly is a handful of refused connections; the cost of
// polling slowly is that every suite pays the rounding error.
const pollInterval = 200 * time.Millisecond

// systemStatus is the subset of GET /api/system/status this package reads.
type systemStatus struct {
	Version    string `json:"Version"`
	InstanceID string `json:"InstanceID"`
}

// ReadyStatus is what WaitReady confirms once a server answers its status
// endpoint. InstanceID is Portainer's own Server Instance ID: the caller
// persists it into the estate (see harness.Server.InstanceID) so a later,
// live read of the same field can be compared against what was recorded here
// — see cleanupOrphans' guard in test/e2e/suite/fixtures_test.go, which
// refuses to run its destructive sweep against a server whose live identity
// does not match.
type ReadyStatus struct {
	Version    string
	InstanceID string
}

// WaitReady polls baseURL until Portainer serves its status endpoint, and
// returns what it reports.
//
// Every failure mode before that point is a retry, not an error: a refused
// connection, a 503, a truncated body. The only thing that ends the loop
// unsuccessfully is ctx expiring, which is what makes the caller's deadline
// the single authority on how long startup may take.
func WaitReady(ctx context.Context, client *http.Client, baseURL string) (ReadyStatus, error) {
	var lastErr error
	for {
		status, err := probeStatus(ctx, client, baseURL)
		if err == nil {
			return status, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ReadyStatus{}, fmt.Errorf("wait for %s: %w (last probe: %w)", baseURL, ctx.Err(), lastErr)
		case <-time.After(pollInterval):
		}
	}
}

func probeStatus(ctx context.Context, client *http.Client, baseURL string) (ReadyStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/system/status", nil)
	if err != nil {
		return ReadyStatus{}, fmt.Errorf("build status request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ReadyStatus{}, fmt.Errorf("probe status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ReadyStatus{}, fmt.Errorf("probe status: http %d", resp.StatusCode)
	}
	var status systemStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return ReadyStatus{}, fmt.Errorf("decode status: %w", err)
	}
	if status.Version == "" {
		return ReadyStatus{}, fmt.Errorf("probe status: empty version")
	}
	return ReadyStatus(status), nil
}
