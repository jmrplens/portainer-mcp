package toolutil

import (
	"fmt"
	"strings"
)

// HintableOutput is embedded in an action's output struct to carry
// model-facing next-step suggestions, such as "call tags.list to see the
// result". It is written once per output formatter and shared by every
// action that returns the same output shape, rather than authored per
// action: the formatter decides the next steps from the result it just
// produced, not from a per-action spec field.
type HintableOutput struct {
	NextSteps []string `json:"next_steps,omitempty"`
}

// hintsStartMarker and hintsEndMarker bound the block WriteHints appends and
// ExtractHints recovers. They are HTML comments so a Markdown renderer that
// only understands prose still displays the surrounding text unchanged.
const (
	hintsStartMarker = "<!-- portainer-mcp:next-steps:start -->"
	hintsEndMarker   = "<!-- portainer-mcp:next-steps:end -->"
)

// WriteHints appends hints to b as a Markdown list bounded by machine
// readable markers. It writes nothing when hints is empty, so a body with no
// next steps carries no trailing marker block. Pair with ExtractHints to
// recover exactly the strings written here from the resulting text.
//
// b is a concrete *strings.Builder, not io.Writer: every output formatter
// that calls this already builds its body in one, and a Builder's Write
// never errors, so there is nothing for a caller to check.
func WriteHints(b *strings.Builder, hints ...string) {
	if len(hints) == 0 {
		return
	}
	fmt.Fprintln(b, hintsStartMarker)
	for _, hint := range hints {
		fmt.Fprintf(b, "- %s\n", hint)
	}
	fmt.Fprintln(b, hintsEndMarker)
}

// ExtractHints recovers the hints WriteHints wrote into body. It returns nil
// — never a slice containing an empty string — when body has no hints
// block, an unterminated one, or one with no list items: any of those would
// otherwise serialise as a blank next step.
func ExtractHints(body string) []string {
	start := strings.Index(body, hintsStartMarker)
	if start == -1 {
		return nil
	}
	rest := body[start+len(hintsStartMarker):]
	end := strings.Index(rest, hintsEndMarker)
	if end == -1 {
		return nil
	}

	var hints []string
	for _, line := range strings.Split(rest[:end], "\n") {
		item, ok := strings.CutPrefix(strings.TrimSpace(line), "- ")
		if !ok {
			continue
		}
		if item = strings.TrimSpace(item); item != "" {
			hints = append(hints, item)
		}
	}
	return hints
}
