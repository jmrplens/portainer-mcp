package main

import (
	"encoding/json"
	"fmt"
)

// normalise applies every repair rule to spec in place and returns one line per
// change made.
//
// Portainer's published specification is not directly consumable by a code
// generator. Each rule here exists because a specific, verified defect in the
// upstream document breaks generation; each is deliberately narrow, so that a
// future upstream fix simply makes the rule report zero changes rather than
// silently altering something else.
func normalise(spec Spec) []string {
	var changes []string
	changes = append(changes, deduplicateEnums(spec, "")...)
	changes = append(changes, narrowWildcardContent(spec, "")...)
	return changes
}

// deduplicateEnums removes repeated values from every enum, preserving order.
//
// Two EE schemas repeat values: docker images.Status (12 values, 6 unique) and
// policies.PolicyType (24 values, 13 unique). oapi-codegen turns them into Go
// switch statements with duplicate cases, which does not compile; ogen rejects
// the document outright.
func deduplicateEnums(node any, path string) []string {
	var changes []string
	switch typed := node.(type) {
	case map[string]any:
		if values, ok := typed["enum"].([]any); ok {
			seen := map[string]bool{}
			unique := make([]any, 0, len(values))
			for _, value := range values {
				key, err := json.Marshal(value)
				if err != nil {
					continue
				}
				if !seen[string(key)] {
					seen[string(key)] = true
					unique = append(unique, value)
				}
			}
			if len(unique) != len(values) {
				typed["enum"] = unique
				changes = append(changes, fmt.Sprintf("%s/enum: %d -> %d values", path, len(values), len(unique)))
			}
		}
		for _, key := range sortedKeys(typed) {
			changes = append(changes, deduplicateEnums(typed[key], path+"/"+key)...)
		}
	case []any:
		for i, value := range typed {
			changes = append(changes, deduplicateEnums(value, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return changes
}

// narrowWildcardContent rewrites `*/*` content types to `application/json`.
//
// 17 EE operations declare `*/*`, including GET /stacks. ogen cannot generate
// for them and would have to skip the operation entirely, which would break 1:1
// coverage. Verified against a live Portainer 2.43.0 EE instance: GET /stacks,
// GET /endpoints/{id}/edge/status and GET /edge_configurations all answer with
// Content-Type: application/json. The wildcard is a lazy annotation, not real
// content negotiation.
//
// An explicit application/json entry alongside the wildcard is left untouched:
// overwriting it would change a contract the spec states deliberately.
func narrowWildcardContent(node any, path string) []string {
	var changes []string
	switch typed := node.(type) {
	case map[string]any:
		if content, ok := typed["content"].(map[string]any); ok {
			_, hasWildcard := content["*/*"]
			_, hasJSON := content["application/json"]
			if hasWildcard && !hasJSON {
				content["application/json"] = content["*/*"]
				delete(content, "*/*")
				changes = append(changes, fmt.Sprintf("%s/content: */* -> application/json", path))
			}
		}
		for _, key := range sortedKeys(typed) {
			changes = append(changes, narrowWildcardContent(typed[key], path+"/"+key)...)
		}
	case []any:
		for i, value := range typed {
			changes = append(changes, narrowWildcardContent(value, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return changes
}
