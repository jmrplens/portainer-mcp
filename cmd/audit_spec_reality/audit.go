package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// selftestPath is a path no vendored specification, past or present, has
// ever documented. Every call to auditLeg probes it first and requires it
// to classify as an absent route before probing anything real.
//
// This is the standing defence against this project's own repeated trap
// (thirteen non-discriminating checks across three phases, all because an
// assertion matched something the fixture supplied): a detector that has
// never been watched catching a known-absent route is indistinguishable
// from one where every probe silently fails and "no divergence" simply
// means "nothing answered". Baking the negative case into every run, rather
// than trusting a one-off manual check, makes that failure mode loud instead
// of quiet — see auditLeg's returned error for what a failed self-test does
// to the rest of that run.
const selftestPath = "/__audit_spec_reality_selftest_canary_route_that_must_never_exist__"

// probeConcurrency bounds how many probes run at once against one server.
// 441 operations at a fraction of a second each is fast even run serially;
// this is a modest speed-up, not a requirement — every probe is
// independent (a fresh request, no shared mutable state but the result
// slice below, which probeMu protects) so there is nothing a bound of 8
// could corrupt by running two probes at once.
const probeConcurrency = 8

// legDivergence is one documented operation a server does not actually
// serve: the specification names a route this server answers with Go's
// literal default-mux 404 instead of routing anywhere.
type legDivergence struct {
	OperationID string
	Method      string
	Path        string
	Domain      string
	StatusCode  int
}

// legResult is what probing every operation in one vendored spec against one
// live server found.
type legResult struct {
	Leg   string
	Total int
	// Divergent lists every operation classified as an absent route, sorted
	// by OperationID for a deterministic report.
	Divergent []legDivergence
	// ProbeErrors lists operations whose probe could not complete at all
	// (a transport failure — a closed connection, a timeout). These are
	// reported distinctly from Divergent: a probe that never got an answer
	// says nothing about whether the route exists, and folding it into
	// "divergent" would overstate what was actually observed.
	ProbeErrors []string
}

// auditLeg probes every operation in ops against baseURL and returns what it
// found.
//
// It runs the self-test (selftestPath) first and refuses to report anything
// about the real operations if the self-test does not come back classified
// as an absent route: see selftestPath's own doc comment for why. This is
// the same order cmd/audit_1to1 uses for its allow-list validation — verify
// the mechanism itself is trustworthy before trusting anything it reports.
func auditLeg(ctx context.Context, legName, baseURL string, ops map[string]specOperation, timeout time.Duration) (legResult, error) {
	client := &http.Client{}

	selftest, err := probe(ctx, client, timeout, http.MethodGet, baseURL, selftestPath)
	if err != nil {
		return legResult{}, fmt.Errorf("%s: self-test probe could not run at all: %w", legName, err)
	}
	if !selftest.RouteAbsent {
		return legResult{}, fmt.Errorf(
			"%s: self-test failed — a request to %s, a path no specification has ever documented, returned status %d instead of Go's literal absent-route response; this audit's detector cannot be trusted this run, so it refuses to report anything about the %d real operations",
			legName, selftestPath, selftest.StatusCode, len(ops))
	}

	names := make([]string, 0, len(ops))
	for name := range ops {
		names = append(names, name)
	}
	sort.Strings(names)

	result := legResult{Leg: legName, Total: len(ops)}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, probeConcurrency)

	for _, name := range names {
		op := ops[name]
		wg.Add(1)
		sem <- struct{}{}
		go func(op specOperation) {
			defer wg.Done()
			defer func() { <-sem }()

			res, err := probe(ctx, client, timeout, op.Method, baseURL, resolvePath(op.Path))

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.ProbeErrors = append(result.ProbeErrors,
					fmt.Sprintf("%s (%s %s): %v", op.OperationID, op.Method, op.Path, err))
				return
			}
			if res.RouteAbsent {
				result.Divergent = append(result.Divergent, legDivergence{
					OperationID: op.OperationID,
					Method:      op.Method,
					Path:        op.Path,
					Domain:      op.Domain,
					StatusCode:  res.StatusCode,
				})
			}
		}(op)
	}
	wg.Wait()

	sort.Slice(result.Divergent, func(i, j int) bool {
		return result.Divergent[i].OperationID < result.Divergent[j].OperationID
	})
	sort.Strings(result.ProbeErrors)

	return result, nil
}
