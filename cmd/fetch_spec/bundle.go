package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// fetcher retrieves a document relative to the API documentation root. It is
// injected so tests never reach the network.
type fetcher func(relativePath string) ([]byte, error)

// Spec is a decoded OpenAPI document.
type Spec = map[string]any

// versionEntry is one row of Portainer's per-edition version manifest.
type versionEntry struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Name string `json:"name"`
}

// bundle assembles one edition and version into a single OpenAPI document.
//
// Portainer splits both paths and components across sibling files. Merging only
// the paths — as an earlier implementation did — leaves every internal $ref
// dangling, which is invisible until a code generator tries to resolve them.
// Both are merged here, and any component name defined twice with differing
// content is reported rather than silently overwritten.
func bundle(fetch fetcher, edition, version string) (Spec, map[string]string, []string, error) {
	manifestRaw, err := fetch(edition + "-versions.json")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch %s version manifest: %w", edition, err)
	}
	var manifest []versionEntry
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, nil, nil, fmt.Errorf("decode %s version manifest: %w", edition, err)
	}

	var entry versionEntry
	for _, candidate := range manifest {
		if candidate.ID == version {
			entry = candidate
			break
		}
	}
	if entry.File == "" {
		return nil, nil, nil, fmt.Errorf("version %q not published for edition %q", version, edition)
	}

	rootRaw, err := fetch(entry.File)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch %s: %w", entry.File, err)
	}
	root, err := decodeYAML(rootRaw)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode %s: %w", entry.File, err)
	}

	// Older versions ship as a single self-contained file.
	if !strings.HasSuffix(entry.File, "/openapi.yaml") {
		return root, map[string]string{}, nil, nil
	}

	dir := entry.File[:strings.LastIndex(entry.File, "/")+1]
	loaded := map[string]Spec{}
	load := func(name string) (Spec, error) {
		if doc, ok := loaded[name]; ok {
			return doc, nil
		}
		raw, err := fetch(dir + name)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", name, err)
		}
		doc, err := decodeYAML(raw)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		loaded[name] = doc
		return doc, nil
	}

	paths, _ := root["paths"].(map[string]any)
	merged := make(map[string]any, len(paths))
	provenance := map[string]string{}
	for path, node := range paths {
		item, _ := node.(map[string]any)
		ref, _ := item["$ref"].(string)
		if ref == "" {
			merged[path] = node
			continue
		}
		name, fragment, found := strings.Cut(ref, "#")
		if !found {
			return nil, nil, nil, fmt.Errorf("path %s: $ref %q has no fragment", path, ref)
		}
		doc, err := load(name)
		if err != nil {
			return nil, nil, nil, err
		}
		resolved, err := resolvePointer(doc, fragment)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("path %s: %w", path, err)
		}
		merged[path] = resolved
		provenance[path] = name
	}
	root["paths"] = merged

	components, _ := root["components"].(map[string]any)
	if components == nil {
		components = map[string]any{}
		root["components"] = components
	}
	var conflicts []string
	for _, name := range sortedKeys(loaded) {
		docComponents, _ := loaded[name]["components"].(map[string]any)
		for _, section := range sortedKeys(docComponents) {
			entries, _ := docComponents[section].(map[string]any)
			target, _ := components[section].(map[string]any)
			if target == nil {
				target = map[string]any{}
				components[section] = target
			}
			for _, key := range sortedKeys(entries) {
				if existing, ok := target[key]; ok && !reflect.DeepEqual(existing, entries[key]) {
					conflicts = append(conflicts, fmt.Sprintf("%s/%s redefined by %s", section, key, name))
				}
				target[key] = entries[key]
			}
		}
	}

	return root, provenance, conflicts, nil
}

// decodeYAML parses YAML into map[string]any. yaml.v3 already decodes mappings
// with string keys into map[string]any, so no key coercion is needed.
func decodeYAML(raw []byte) (Spec, error) {
	var doc Spec
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return doc, nil
}

// resolvePointer walks a JSON Pointer fragment such as /paths/~1stacks.
func resolvePointer(doc Spec, fragment string) (any, error) {
	var cur any = doc
	for _, part := range strings.Split(strings.Trim(fragment, "/"), "/") {
		key := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		node, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("pointer %q: %q is not an object", fragment, key)
		}
		cur, ok = node[key]
		if !ok {
			return nil, fmt.Errorf("pointer %q: %q not found", fragment, key)
		}
	}
	return cur, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
