package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var refPattern = regexp.MustCompile(`"\$ref":\s*"([^"]+)"`)

// danglingRefs reports every $ref in spec that does not resolve within it.
//
// An external $ref is reported too: it means bundling did not finish, and a
// generator would have to fetch at build time, defeating the point of
// committing the specification.
func danglingRefs(spec Spec) []string {
	encoded, err := json.Marshal(spec)
	if err != nil {
		return []string{fmt.Sprintf("encode spec: %v", err)}
	}

	seen := map[string]bool{}
	var broken []string
	for _, match := range refPattern.FindAllStringSubmatch(string(encoded), -1) {
		ref := match[1]
		if seen[ref] {
			continue
		}
		seen[ref] = true
		if !strings.HasPrefix(ref, "#/") {
			broken = append(broken, "external: "+ref)
			continue
		}
		if _, err := resolvePointer(spec, ref[1:]); err != nil {
			broken = append(broken, ref)
		}
	}
	return broken
}
