package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// probeSentinelValue is deliberately not a valid Portainer credential of any
// shape: no admin password, no API key, no JWT this server ever issued.
// Sending it (instead of the estate's real credential) is what makes every
// probe below safe on every HTTP verb, including a delete, a restore, or the
// Community-to-Business upgrade — see runLeg's doc comment for the measured
// evidence this rests on. It carries a fixed, recognisable value rather than
// an empty header or a random one so that anyone reading Portainer's own
// access log during a run can tell an audit_spec_reality probe apart from a
// real client's mistake.
const probeSentinelValue = "portainer-mcp-audit-spec-reality-never-a-real-value"

// pathParamRE matches one {name} OpenAPI path template segment.
var pathParamRE = regexp.MustCompile(`\{[^{}]+\}`)

// probePlaceholder is substituted for every path parameter, regardless of
// its name or declared type.
//
// Routing matches on path *shape* — segment count and literal segments —
// never on a parameter's runtime value: measured directly against this
// project's own live estate, GET /registries/1 and GET /docker/1/... both
// reached their route's auth check (a 401), not the literal absent-route
// response, regardless of whether "1" named a real registry or environment.
// A single fixed placeholder therefore round-trips identically whether the
// real parameter is an integer id, an environment id, or a string name.
const probePlaceholder = "1"

// resolvePath fills every {param} segment in an OpenAPI path template with
// probePlaceholder, producing the concrete path a real request would use.
func resolvePath(path string) string {
	return pathParamRE.ReplaceAllString(path, probePlaceholder)
}

// bodyFor returns the request body a probe of this method should carry.
//
// GET, HEAD, DELETE and OPTIONS carry none: Portainer's own auth check runs
// before any handler reads a body (measured: DELETE /registries/1 with the
// probe credential returned the same 401 a GET does), so there is nothing a
// body could protect against on those verbs. POST, PUT and PATCH carry an
// empty JSON object — syntactically valid, so a body decoder never reports
// a parse error unrelated to what this probe tests, and exactly what this
// project's own live investigation of registries.configure already sent by
// hand against this same server (see plan/carry-forward.md).
func bodyFor(method string) io.Reader {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return strings.NewReader("{}")
	default:
		return nil
	}
}

// probeResult is one operation's classified outcome against a live server.
type probeResult struct {
	StatusCode  int
	RouteAbsent bool
}

// maxProbeBodyBytes caps how much of a probed response this command reads.
// isRouteAbsent only ever compares against a 19-byte literal ("404 page not
// found"), but a probed route may stream an arbitrarily large body — an
// archive download, a backup, or a log tail — and auditLeg runs
// probeConcurrency probes at once across 441 operations, so several such
// bodies can be resident in memory at the same time. 4KiB is generous
// headroom over the 19 bytes actually needed.
const maxProbeBodyBytes = 4 << 10

// isRouteAbsent reports whether statusCode/body is Go's default mux's
// literal answer to a path with no registered handler ("404 page not
// found", as plain text), as opposed to any response a matched route's own
// handler chain produced — including a 401 from Portainer's auth check, a
// 404 carrying its own {"message","details"} body for a missing resource,
// or an outright success.
//
// This is the whole mechanism the design specification identified and this
// command exists to exploit: see this package's doc comment in main.go for
// the measurements confirming every one of those alternatives is
// distinguishable from the exact literal checked here.
func isRouteAbsent(statusCode int, body []byte) bool {
	return statusCode == http.StatusNotFound && strings.TrimSpace(string(body)) == "404 page not found"
}

// probe issues one HTTP request against baseURL+path using method, with the
// probe credential and (for a mutating verb) an empty JSON body, and
// classifies the response. It never sends the estate's real credential.
func probe(ctx context.Context, client *http.Client, timeout time.Duration, method, baseURL, path string) (probeResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body := bodyFor(method)
	req, err := http.NewRequestWithContext(reqCtx, method, baseURL+path, body)
	if err != nil {
		return probeResult{}, fmt.Errorf("build %s %s: %w", method, path, err)
	}
	req.Header.Set("X-API-Key", probeSentinelValue)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return probeResult{}, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A bounded read is enough: isRouteAbsent compares against a 19-byte
	// literal, and a probed route may stream an arbitrarily large body.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBodyBytes))
	if err != nil {
		return probeResult{}, fmt.Errorf("%s %s: read response: %w", method, path, err)
	}

	return probeResult{
		StatusCode:  resp.StatusCode,
		RouteAbsent: isRouteAbsent(resp.StatusCode, data),
	}, nil
}

// adminCheckPath is the one route this audit calls for a reason other than
// probing: Portainer answers it 204 when an administrator account already
// exists and 404 when none does. There is no "Initialized" field anywhere in
// either vendored specification — GET /system/status returns only Version and
// InstanceID — so this status-only route is the sole documented, credential-
// free way to ask whether an instance has been set up.
const adminCheckPath = "/users/admin/check"

// estateInitialized reports whether the server at baseURL already has an
// administrator account.
//
// It is deliberately three-valued in effect: (true, nil) when the server said
// 204 (or 200, which some Portainer versions answer this route with — both
// mean an administrator already exists), (false, nil) when it said 404, and
// (false, err) when the question could not be answered at all. A caller must
// treat the error case as "not confirmed", never as "initialized" — see
// auditLeg, which skips the public routes on anything but a clear yes.
//
// Unlike every other request this command makes, this one carries no
// credential: the route is itself public, and sending the sentinel would only
// invite Portainer to reject the request before answering the question.
func estateInitialized(ctx context.Context, client *http.Client, timeout time.Duration, baseURL string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+adminCheckPath, nil)
	if err != nil {
		return false, fmt.Errorf("build admin-check request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("admin-check request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("admin-check returned status %d, which answers neither yes nor no", resp.StatusCode)
	}
}
