package toolutil

import (
	"reflect"
	"strings"
)

// ParameterGuidance is model-facing help for supplying one parameter
// correctly: what it means, where a valid value comes from, and what it is
// commonly confused with. It is discovery metadata, read by the tool
// surfaces when they publish a schema, never by Validate.
type ParameterGuidance struct {
	// SemanticRole says what the parameter identifies or controls, in terms
	// a model can act on rather than restating its type.
	SemanticRole string `json:"semantic_role,omitempty"`
	// ValueSource says where a valid value comes from — typically another
	// action's output — so a model does not have to guess or invent one.
	ValueSource string `json:"value_source,omitempty"`
	// CommonConfusions lists other parameters or concepts this one is
	// mistaken for, and why they are not interchangeable.
	CommonConfusions []string `json:"common_confusions,omitempty"`
	// ExampleBinding is a single concrete value, illustrating shape rather
	// than asserting that the value is valid for any particular server.
	ExampleBinding string `json:"example_binding,omitempty"`
}

// scopeParameterDefaults is the central table this whole mechanism exists
// for. Portainer reuses a small set of identifier names — endpointId,
// stackId, userId and the like — across most of its 441 actions, so writing
// their guidance once here and letting FillScopeParameterGuidance apply it
// everywhere is what makes populating ParameterGuidance for every action
// affordable. A domain only ever hand-writes guidance for a parameter that
// is genuinely specific to it, or for a shared name whose meaning it
// overrides.
//
// endpointId, environmentId and edgeId exist because Portainer overloads
// "environment": an endpoint id, an environment-group id and an edge id are
// three different numbers that all show up as "id" in different routes. A
// model that confuses them acts on the wrong host, which is why each of the
// three entries cross-references the other two by name.
var scopeParameterDefaults = map[string]ParameterGuidance{
	"id": {
		SemanticRole: "Numeric identifier of the resource this action targets. Its exact meaning is route-specific.",
		ValueSource:  "Depends on the domain: it may be an environment (endpoint) id, a stack id, a user id or another resource's id. Check a *.list action in the same domain for valid values.",
		CommonConfusions: []string{
			"Not necessarily an environment id: many domains reuse \"id\" for their own resource, even though Portainer also names an environment's identifier \"id\" in endpoint routes.",
		},
		ExampleBinding: "1",
	},
	"endpointId": {
		SemanticRole: "Identifies the Portainer environment (endpoint) an action targets — the Docker, Kubernetes or Nomad host or cluster registered with this Portainer instance.",
		ValueSource:  "The numeric id of an environment, as listed by environments.list.",
		CommonConfusions: []string{
			"Not an edge id: edge-specific routes address the same edge-agent environment through a separate edgeId namespace.",
			"Not an environment group id: environment groups have their own numeric id, distinct from any member environment's endpointId.",
		},
		ExampleBinding: "3",
	},
	"environmentId": {
		SemanticRole: "Alternate field name Portainer uses for the same environment (endpoint) identifier as endpointId; some routes name it environmentId instead.",
		ValueSource:  "The numeric id of an environment, as listed by environments.list — the same value space as endpointId.",
		CommonConfusions: []string{
			"Not an edge id: edge-specific routes use a separate edgeId namespace even when addressing an edge-agent environment.",
			"Not an environment group id: groups are addressed by their own id, never by environmentId.",
		},
		ExampleBinding: "3",
	},
	"edgeId": {
		SemanticRole: "Identifies an Edge Compute environment or device in edge-specific routes such as edge stacks, edge jobs and edge updates.",
		ValueSource:  "The numeric id of an edge-enabled environment, as listed among edge environments.",
		CommonConfusions: []string{
			"Not interchangeable with endpointId or environmentId: an edge environment has both an endpointId for general environment routes and an edgeId for edge-specific routes, and the two numbers are not the same.",
			"Not an environment group id.",
		},
		ExampleBinding: "7",
	},
	"stackId": {
		SemanticRole: "Identifies a Portainer stack — a deployed compose or manifest unit — independent of which environment it runs on.",
		ValueSource:  "The numeric id of a stack, as listed by stacks.list.",
		CommonConfusions: []string{
			"Not an endpointId: a stack has its own id even though it also targets a specific environment via endpointId.",
		},
		ExampleBinding: "12",
	},
	"registryId": {
		SemanticRole:   "Identifies a container registry configured in Portainer.",
		ValueSource:    "The numeric id of a registry, as listed by registries.list.",
		ExampleBinding: "2",
	},
	"userId": {
		SemanticRole: "Identifies a Portainer user account.",
		ValueSource:  "The numeric id of a user, as listed by users.list.",
		CommonConfusions: []string{
			"Not a teamId: users and teams are separate identifier spaces.",
		},
		ExampleBinding: "5",
	},
	"teamId": {
		SemanticRole: "Identifies a Portainer team — a named group of users.",
		ValueSource:  "The numeric id of a team, as listed by teams.list.",
		CommonConfusions: []string{
			"Not a userId: teams and users are separate identifier spaces.",
		},
		ExampleBinding: "4",
	},
	"namespace": {
		SemanticRole:   "Names the Kubernetes namespace an action targets within an environment.",
		ValueSource:    "A Kubernetes namespace name — a string, not a numeric id — as listed by the Kubernetes namespace-listing action for the target endpointId.",
		ExampleBinding: "default",
	},
	"tagId": {
		SemanticRole:   "Identifies a Portainer tag used to group and filter environments.",
		ValueSource:    "The numeric id of a tag, as listed by tags.list.",
		ExampleBinding: "1",
	},
}

// FillScopeParameterGuidance returns a copy of specs in which every
// parameter named in the central default table above has guidance, without
// disturbing any guidance already set explicitly. It never mutates specs
// itself: callers pass package-level slices shared across editions and
// surfaces, and filling in place would leak one catalog's guidance into
// another's.
func FillScopeParameterGuidance(specs []ActionSpec) []ActionSpec {
	out := make([]ActionSpec, len(specs))
	for i, spec := range specs {
		merged := make(map[string]ParameterGuidance, len(spec.ParameterGuidance))
		for name, guidance := range spec.ParameterGuidance {
			merged[name] = guidance
		}
		for _, name := range jsonFieldNames(spec.Input) {
			if _, explicit := merged[name]; explicit {
				continue
			}
			if def, ok := scopeParameterDefaults[name]; ok {
				merged[name] = def
			}
		}
		if len(merged) == 0 {
			merged = nil
		}
		spec.ParameterGuidance = merged
		out[i] = spec
	}
	return out
}

// jsonFieldNames returns the JSON field names of input's struct type, in the
// order they appear, using the same tag rules encoding/json applies: an
// explicit name before the first comma, "-" excludes the field, and no tag
// falls back to the Go field name. input may be nil, a struct or a pointer
// to one; anything else yields no names.
func jsonFieldNames(input any) []string {
	if input == nil {
		return nil
	}
	t := reflect.TypeOf(input)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		tag, hasTag := field.Tag.Lookup("json")
		name, _, _ := strings.Cut(tag, ",")
		if hasTag && name == "-" && tag == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}
	return names
}
