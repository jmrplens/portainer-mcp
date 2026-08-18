// Package specnaming holds the one rule that decides what an operation's
// top-level fields are called on the wire when two different parts of the
// same operation want the same name.
//
// Why a package of its own, rather than a function in either caller: two
// independent derivations of an operation's field names already exist and
// must agree exactly — cmd/gen_action_inputs's assembleOperationFields
// (which names the fields a generated Input struct publishes to a model) and
// internal/specdiff's ShapeFromSpec (which names the fields the drift audit
// expects to find in that struct). If those two ever name the same field
// differently, cmd/audit_spec_drift does not report "these two disagree": it
// reports one field added and another removed, on an operation nobody
// touched, and the allow-list gains an entry excusing a divergence that does
// not exist. cmd/gen_action_inputs is `package main` and cannot be imported
// from anywhere, which has previously been read as "reimplement the rule on
// both sides" (see internal/specdiff/naming.go, which does exactly that for
// bodyJSONTag). It only forbids importing the *main* package: both sides
// already import internal/toolutil, so both can import this, and a rule that
// lives here has no mirror to keep in step.
//
// The rule, in one sentence: when a path, query or header parameter
// contributes the same wire name as a request-body property, the body keeps
// the plain name and the parameter carries its own origin as a suffix.
//
// Why the body wins, written down so a future reader can disagree
// deliberately rather than by accident. Across both vendored
// specifications there are exactly five operations where this fires, and in
// every one of them the body property is the substantive input and the
// parameter is the addressing or repair one:
//
//   - StackMigrate (POST /stacks/{id}/migrate) declares a required body
//     property "EndpointID" naming the environment the stack is migrating
//     *to*, and an optional query parameter "endpointId" that exists only to
//     supply the environment a pre-1.18 stack was created without. The
//     required field that names the destination is the one a model should
//     find under the obvious name.
//   - CreateKubernetesIngress, UpdateKubernetesIngress,
//     CreateKubernetesService and UpdateKubernetesService each declare a path
//     parameter "namespace" and a body property "Namespace". Here the
//     argument is weaker — the path parameter is what actually addresses the
//     resource — and a future reader may reasonably decide these four should
//     go the other way. What must not happen is a per-operation table:
//     "qualify the body for these four, the parameter for that one" is one
//     more thing to maintain, to forget, and to let the two sides drift
//     apart on. If the ruling changes, change it here, for all five, and let
//     the agreement test in cmd/gen_action_inputs prove both sides moved
//     together.
//
// This rule deliberately does not resolve a collision between two *body*
// properties. bodyJSONTag is not injective, so two distinct spec property
// names can render to one JSON tag; there is no principled winner between
// them, and both callers still refuse that outright.
package specnaming

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// The origins a top-level field can be contributed by. These are the same
// strings cmd/gen_action_inputs's fieldSpec.Origin and
// internal/specdiff's FieldShape.Origin already carry, so neither side has
// to translate between its own vocabulary and this package's.
const (
	OriginPath   = "path"
	OriginQuery  = "query"
	OriginHeader = "header"
	OriginBody   = "body"
)

// Parameter is one path, query or header parameter as the specification
// declares it: the name it contributes on the wire, and where it came from.
type Parameter struct {
	Name   string
	Origin string
}

// ResolveParameters returns, by index, the wire name each of params
// contributes once collisions with the request body are disambiguated.
// bodyNames is every wire name the request body contributes — already
// rendered the way the caller publishes it (bodyJSONTag on the generator
// side, the identical transform in internal/specdiff), and including a
// synthesised name for a map-shaped body, since that too is a name the body
// occupies.
//
// A parameter whose name no body property takes is returned unchanged; this
// is the case for every operation in both vendored specifications but five.
// A parameter that collides is returned qualified by Qualify.
//
// It refuses when a qualified name is itself already taken by a third
// contributor — another parameter, or another body property. Renaming a
// field into a name something else already occupies is the same silent
// shadowing this rule exists to prevent, one step removed, and there is no
// second suffix to reach for that would not be inventing names to escape a
// specification nobody has actually written yet.
//
// It deliberately does not refuse the collisions its callers already refuse
// for themselves: two parameters sharing one name, or two body properties
// rendering to one tag. Those keep their existing, more specific refusals at
// the call sites, which name the raw specification identifiers this function
// never sees.
//
// The result depends only on the set of names and origins passed in, never
// on their order: whether a parameter is qualified is decided against
// bodyNames alone (a set), and the refusal is decided against the complete
// set of final names, which is the same set for every permutation. The
// refusal message sorts its own contents for the same reason.
func ResolveParameters(params []Parameter, bodyNames []string) ([]string, error) {
	body := make(map[string]bool, len(bodyNames))
	for _, n := range bodyNames {
		body[n] = true
	}

	resolved := make([]string, len(params))
	qualified := make([]bool, len(params))
	for i, p := range params {
		resolved[i] = p.Name
		if p.Name == "" || !body[p.Name] {
			continue
		}
		q, err := Qualify(p.Name, p.Origin)
		if err != nil {
			return nil, fmt.Errorf("%s parameter %q collides with a request body property of the same name: %w", p.Origin, p.Name, err)
		}
		resolved[i] = q
		qualified[i] = true
	}

	// Every final name, counted once per contributor. A qualified name that
	// occurs more than once here landed on a name something else already
	// holds — the one case this rule cannot resolve.
	occurrences := make(map[string]int, len(body)+len(params))
	for n := range body {
		occurrences[n]++
	}
	for _, n := range resolved {
		occurrences[n]++
	}

	var refusals []string
	for i, p := range params {
		if !qualified[i] || occurrences[resolved[i]] < 2 {
			continue
		}
		refusals = append(refusals, fmt.Sprintf("%s parameter %q would be renamed to %q, which is already contributed by another field", p.Origin, p.Name, resolved[i]))
	}
	if len(refusals) > 0 {
		sort.Strings(refusals) // the message must not depend on parameter order either
		return nil, fmt.Errorf("cannot disambiguate a parameter that shares a wire name with a request body property: %s", strings.Join(refusals, "; "))
	}

	return resolved, nil
}

// Qualify renders the disambiguated form of a parameter's wire name:
// the name with its origin appended in the lower-camel-case every wire name
// in this project is published in ("endpointId" from the query becomes
// "endpointIdQuery").
//
// A suffix rather than a prefix so the qualified name sorts next to the
// plain one — a model reading an alphabetically ordered schema sees
// "endpointId" and "endpointIdQuery" adjacent, which is the only hint it
// gets that the two are related — and so the semantic head of the name
// stays where a reader looks for it.
//
// It refuses an empty name, an empty origin and OriginBody. The first two
// have no qualified form to render; the third is the rule itself: the body
// keeps the plain name, so asking for its qualified one means a caller has
// inverted the rule (see this package's doc comment).
func Qualify(name, origin string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("cannot qualify an empty wire name")
	}
	if origin == "" {
		return "", fmt.Errorf("parameter %q states no origin to qualify it by", name)
	}
	if origin == OriginBody {
		return "", fmt.Errorf("a request body property keeps its plain name and is never qualified; %q was asked for its %s-qualified form", name, origin)
	}
	return name + titleFirst(origin), nil
}

// titleFirst upper-cases s's first rune and leaves the rest alone: the
// origins this package qualifies by are single lower-case words, so this is
// the whole of the casing rule.
func titleFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
