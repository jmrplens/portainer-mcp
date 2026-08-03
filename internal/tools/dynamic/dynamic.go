// Package dynamic projects the catalog as two tools regardless of its size.
//
// A model calls portainer_find_action to discover an action, then
// portainer_execute_action to run it. This keeps the model-facing tool list at
// two entries whether the catalog holds five actions or 442, which is why it is
// the default surface.
package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// maxMatches bounds what find returns. The catalog reaches 442 actions, and a
// broad query would otherwise return most of it in the response body — the same
// token cost this surface exists to avoid, just moved out of the tool list.
// The count of total matches is always reported, so a model can tell it should
// narrow the query rather than assume it has seen everything.
const maxMatches = 20

// Surface registers the find and execute pair.
type Surface struct{}

// FindInput is what portainer_find_action accepts.
type FindInput struct {
	Query string `json:"query" jsonschema:"what you want to do, in plain words — for example 'list stacks' or 'delete a tag'"`
}

// ExecuteInput is what portainer_execute_action accepts.
//
// Input is map[string]any, not json.RawMessage. The SDK infers a tool's input
// schema from this struct, and json.RawMessage is []byte, which infers to an
// array of integers 0-255 — so a client sending a real object would be
// rejected by schema validation before dispatch ever ran. Verified against
// go-sdk v1.7.0.
type ExecuteInput struct {
	Action string         `json:"action" jsonschema:"the canonical action name returned by portainer_find_action"`
	Input  map[string]any `json:"input,omitempty" jsonschema:"that action's parameters"`
}

// match is one search result.
//
// InputSchema, RequiredParams and Example are what make this surface's token
// cost acceptable at 441 actions: find only ever returns them for the
// handful of matches a query produced, never for the whole catalog, so
// publishing the full shape here costs nothing the surface wasn't already
// paying to describe the match at all.
type match struct {
	Action         string         `json:"action"`
	Domain         string         `json:"domain"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Mutating       bool           `json:"mutating"`
	Destructive    bool           `json:"destructive"`
	InputSchema    map[string]any `json:"inputSchema"`
	RequiredParams []string       `json:"requiredParams,omitempty"`
	Example        map[string]any `json:"example,omitempty"`
	score          int
	// spec is carried unexported (never marshalled) so fillSchemaDetails can
	// compute InputSchema/RequiredParams/Example after truncation, for only
	// the matches actually returned — see search's doc comment for why that
	// order matters at 441 actions.
	spec toolutil.ActionSpec
}

// findResult wraps the matches so a truncated result can say so. A model that
// receives a bare list has no way to tell whether it saw everything.
type findResult struct {
	Matched int     `json:"matched"`
	Matches []match `json:"matches"`
	Note    string  `json:"note,omitempty"`
}

// Register adds portainer_find_action and portainer_execute_action.
func (Surface) Register(server *mcp.Server, catalog *actioncatalog.Catalog, deps tools.Deps) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "portainer_find_action",
		Title:       "Find a Portainer action",
		Description: "Searches the available Portainer actions and returns those matching your query. Call this first, then pass the chosen action's canonical name to portainer_execute_action. Results say whether an action mutates or destroys state, and each match carries its full parameter JSON Schema, which of those parameters are required, and an example value for each — use them instead of guessing.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in FindInput) (*mcp.CallToolResult, any, error) {
		matches := search(catalog.Actions(), in.Query)
		if len(matches) == 0 {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
				"No action matched %q. Try broader words, or a domain name: %s",
				in.Query, strings.Join(catalog.Domains(), ", "))}}}, nil, nil
		}

		result := findResult{Matched: len(matches), Matches: matches}
		if len(matches) > maxMatches {
			result.Matches = matches[:maxMatches]
			result.Note = fmt.Sprintf(
				"Showing the %d best matches of %d. Narrow the query to see the rest.",
				maxMatches, len(matches))
		}
		fillSchemaDetails(result.Matches)

		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("encode matches: %w", err)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "portainer_execute_action",
		Title:       "Execute a Portainer action",
		Description: "Runs an action discovered with portainer_find_action. Pass its canonical name and that action's parameters.",
		Annotations: executeAnnotations(catalog),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ExecuteInput) (*mcp.CallToolResult, any, error) {
		spec, ok := catalog.Lookup(in.Action)
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
					"Unknown action %q. Call portainer_find_action to discover the available actions.", in.Action)}},
			}, nil, nil
		}
		input, err := json.Marshal(in.Input)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: encode input: %w", in.Action, err)
		}
		res, err := tools.Execute(ctx, spec, deps, input)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", spec.Name, err)
		}
		return res, nil, nil
	})

	return nil
}

// Scoring weights. A name match must always outrank description matches: the
// haystack contribution is capped below a single name hit, because a long
// natural-language query would otherwise let an action whose description
// happens to share many common words outscore the action actually named.
const (
	scoreExactName     = 100
	scoreNameSubstring = 10
	scoreHaystackTerm  = 1
	maxHaystackScore   = 5
)

// search ranks actions against a query. Matching is deliberately simple: an
// exact name match beats a name substring, which beats a title or description
// hit. P3 can add synonyms and fuzzy matching once the catalog is large enough
// for the difference to be measurable.
//
// It does not compute InputSchema, RequiredParams or Example — that is left
// to fillSchemaDetails, called by Register only after truncating to
// maxMatches. Register discards everything past maxMatches, and at 441
// actions a broad query can score most of the catalog; computing the schema
// details (a deep copy plus a fresh example map, per match) for results that
// are about to be thrown away is wasted work on the hot path every call to
// portainer_find_action takes.
func search(specs []toolutil.ActionSpec, query string) []match {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return nil
	}

	var out []match
	for _, spec := range specs {
		name := strings.ToLower(spec.Name)
		haystack := strings.ToLower(spec.Name + " " + spec.Domain + " " + spec.Title + " " + spec.Description)

		score, haystackScore := 0, 0
		for _, term := range terms {
			switch {
			case name == term:
				score += scoreExactName
			case strings.Contains(name, term):
				score += scoreNameSubstring
			case strings.Contains(haystack, term):
				haystackScore += scoreHaystackTerm
			}
		}
		if haystackScore > maxHaystackScore {
			haystackScore = maxHaystackScore
		}
		score += haystackScore
		if score == 0 {
			continue
		}
		out = append(out, match{
			Action: spec.Name, Domain: spec.Domain, Title: spec.Title,
			Description: spec.Description, Mutating: spec.Mutating,
			Destructive: spec.Destructive, score: score, spec: spec,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].Action < out[j].Action
	})
	return out
}

// fillSchemaDetails populates InputSchema, RequiredParams and Example for
// exactly the matches passed in, mutating each in place. Called by Register
// after truncating search's results to maxMatches, so the per-match schema
// deep-copy and example-map allocation are paid only for what the response
// actually returns.
func fillSchemaDetails(matches []match) {
	for i := range matches {
		matches[i].InputSchema, matches[i].RequiredParams, matches[i].Example = schemaDetails(matches[i].spec)
	}
}

// schemaDetails computes the three parameter-shape fields a match carries:
// its input schema, the required parameter names within it, and an
// illustrative example. actioncatalog.Build already runs spec.Validate before
// any action reaches a catalog, which guarantees Input is nil or a struct —
// the only cases InputSchema can fail on — so an error here would mean a
// defect elsewhere in the catalog, not bad input a caller of search supplied.
// Rather than fail the whole find call over one such spec, this degrades to
// no schema for that match alone.
func schemaDetails(spec toolutil.ActionSpec) (schema map[string]any, required []string, example map[string]any) {
	schema, err := spec.InputSchema()
	if err != nil {
		return nil, nil, nil
	}
	return schema, toolutil.RequiredParams(schema), exampleFor(spec, schema)
}

// exampleFor builds a single illustrative value per property in schema, so a
// model calling execute has something concrete to shape its call around
// instead of inferring types from the schema alone. A property named in
// spec.ParameterGuidance uses that guidance's ExampleBinding; every other
// property falls back to a generic placeholder for its JSON Schema type.
// Returns nil when the schema declares no properties, so an action that
// takes nothing carries no empty example object.
func exampleFor(spec toolutil.ActionSpec, schema map[string]any) map[string]any {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return nil
	}
	example := make(map[string]any, len(props))
	for name, raw := range props {
		propSchema, _ := raw.(map[string]any)
		if guidance, ok := spec.ParameterGuidance[name]; ok && guidance.ExampleBinding != "" {
			if value, ok := bindingAs(propSchema, guidance.ExampleBinding); ok {
				example[name] = value
				continue
			}
		}
		example[name] = placeholderFor(propSchema)
	}
	return example
}

// bindingAs converts a guidance ExampleBinding, which is always text, to the
// property's declared JSON Schema type. FillScopeParameterGuidance fills
// guidance for Portainer's identifier parameters, which commonly declare
// "integer" (id, endpointId): publishing ExampleBinding as a bare string for
// one of those would show a quoted value where the schema requires a number,
// so a model copying the example verbatim would fail schema validation. It
// reports false when the text does not parse as the declared type, so the
// caller falls back to a generic placeholder rather than publishing an
// example the schema itself would reject.
func bindingAs(propSchema map[string]any, binding string) (any, bool) {
	switch propType(propSchema) {
	case "string":
		return binding, true
	case "integer":
		n, err := strconv.Atoi(binding)
		return n, err == nil
	case "number":
		f, err := strconv.ParseFloat(binding, 64)
		return f, err == nil
	case "boolean":
		b, err := strconv.ParseBool(binding)
		return b, err == nil
	default:
		return nil, false
	}
}

// placeholderFor returns a generic value matching a JSON Schema property's
// declared type, for a property exampleFor has no ParameterGuidance for. It
// illustrates shape only — never a value claimed to be valid on any real
// server, which is exactly the caveat ParameterGuidance.ExampleBinding
// documents for its own, guidance-backed examples.
func placeholderFor(propSchema map[string]any) any {
	switch propType(propSchema) {
	case "string":
		return "string"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	default:
		return nil
	}
}

// propType reads a JSON Schema property's effective type for placeholderFor
// and bindingAs. "type" is not always the single plain string those two
// switch on directly: a nullable property can declare it as an array (e.g.
// ["string", "null"]), and a $ref'd or inline-object property can omit "type"
// altogether while still declaring "properties". Reading only raw["type"] as
// a string turned every one of those into the default branch, so
// placeholderFor published a bare null — no shape at all — for exactly the
// properties a model most needs an example of. A type array resolves to its
// first non-"null" entry; a schema with no type but a $ref or its own
// properties is treated as an object.
func propType(propSchema map[string]any) string {
	switch t := propSchema["type"].(type) {
	case string:
		return t
	case []any:
		for _, entry := range t {
			if s, ok := entry.(string); ok && s != "null" {
				return s
			}
		}
	}
	if _, hasRef := propSchema["$ref"]; hasRef {
		return "object"
	}
	if _, hasProps := propSchema["properties"]; hasProps {
		return "object"
	}
	return ""
}

// executeAnnotations describe the execute tool, which can reach anything in the
// catalog, so it must advertise the most dangerous action available.
func executeAnnotations(catalog *actioncatalog.Catalog) *mcp.ToolAnnotations {
	a := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := false
	for _, spec := range catalog.Actions() {
		if spec.Mutating {
			a.ReadOnlyHint = false
		}
		if spec.Destructive {
			destructive = true
		}
	}
	if !a.ReadOnlyHint {
		a.DestructiveHint = &destructive
	}
	return a
}
