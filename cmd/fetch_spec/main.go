// Command fetch_spec downloads Portainer's published OpenAPI specification,
// bundles its split files into one document, applies normalisation rules, and
// writes the result under api/specs/ for committing.
//
// The specification is committed rather than fetched at build time so that
// builds are reproducible and work offline, and so that every upgrade to a new
// Portainer release arrives as a reviewable diff.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const specBaseURL = "https://api-docs.portainer.io/"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "fetch_spec: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("fetch_spec", flag.ContinueOnError)
	edition := fs.String("edition", "ee", "Portainer edition: ce or ee")
	version := fs.String("version", "", "Portainer version, e.g. 2.44.0 (required)")
	outDir := fs.String("out", "api/specs", "directory to write the bundled spec into")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if *version == "" {
		return fmt.Errorf("-version is required")
	}
	if *edition != "ce" && *edition != "ee" {
		return fmt.Errorf("invalid -edition %q: want ce or ee", *edition)
	}

	spec, provenance, conflicts, err := bundle(httpFetcher(), *edition, *version)
	if err != nil {
		return err
	}
	for _, conflict := range conflicts {
		fmt.Fprintf(os.Stderr, "  conflict: %s\n", conflict)
	}
	for _, change := range normalise(spec) {
		fmt.Fprintf(os.Stderr, "  normalised: %s\n", change)
	}
	if broken := danglingRefs(spec); len(broken) > 0 {
		for _, ref := range broken {
			fmt.Fprintf(os.Stderr, "  dangling: %s\n", ref)
		}
		return fmt.Errorf("%d unresolved reference(s); refusing to write a spec a generator cannot consume", len(broken))
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%d component name conflict(s); resolve before writing", len(conflicts))
	}

	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", *outDir, err)
	}
	if err := writeJSON(filepath.Join(*outDir, fmt.Sprintf("%s-%s.json", *edition, *version)), spec); err != nil {
		return err
	}
	if len(provenance) > 0 {
		if err := writeJSON(filepath.Join(*outDir, fmt.Sprintf("%s-%s-provenance.json", *edition, *version)), provenance); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "wrote %s %s\n", *edition, *version)
	return nil
}

// httpFetcher retrieves documents from Portainer's documentation host. The
// default User-Agent is rejected by its CDN, so one is set explicitly.
func httpFetcher() fetcher {
	client := &http.Client{Timeout: 60 * time.Second}
	return func(rel string) ([]byte, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, specBaseURL+rel, nil)
		if err != nil {
			return nil, fmt.Errorf("build request for %s: %w", rel, err)
		}
		req.Header.Set("User-Agent", "portainer-mcp-fetch-spec")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", rel, err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("get %s: status %d", rel, resp.StatusCode)
		}
		const maxSpecBytes = 64 << 20 // 64 MiB; the largest published spec is ~1.2 MB
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxSpecBytes))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", rel, err)
		}
		if len(body) == int(maxSpecBytes) {
			return nil, fmt.Errorf("read %s: response exceeds %d bytes", rel, maxSpecBytes)
		}
		return body, nil
	}
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", " ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	// Write to a sibling temporary file and rename: os.Rename is atomic within
	// a filesystem, so a crash or a later failure can never leave a truncated
	// or orphaned file at the final path.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpName, path, err)
	}
	return nil
}
