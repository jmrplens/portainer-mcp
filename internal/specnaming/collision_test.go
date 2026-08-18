package specnaming_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specnaming"
)

// TestUnit_ResolveParameters_NoCollision_EveryNameUnchanged pins the rule's
// only common case: an operation whose parameters and body properties
// already contribute distinct wire names must come out of ResolveParameters
// byte-for-byte unchanged. A rule that qualified unconditionally would
// rename 400-odd operations' parameters and make every one of them drift
// against its own catalog shape, which is a far larger failure than the
// collision it was written for.
func TestUnit_ResolveParameters_NoCollision_EveryNameUnchanged(t *testing.T) {
	t.Parallel()
	params := []specnaming.Parameter{
		{Name: "id", Origin: specnaming.OriginPath},
		{Name: "endpointId", Origin: specnaming.OriginQuery},
	}
	got, err := specnaming.ResolveParameters(params, []string{"name", "swarmId"})
	if err != nil {
		t.Fatalf("ResolveParameters() error = %v, want nil", err)
	}
	want := []string{"id", "endpointId"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResolveParameters() = %v, want %v", got, want)
	}
}

// TestUnit_ResolveParameters_CrossOriginCollision_BodyKeepsThePlainName is
// the defect this package exists for, in the exact shape StackMigrate
// declares it: a query parameter and a body property that both render to
// "endpointId". The body keeps the plain name; the parameter carries its
// origin. Both directions are asserted — a rule that qualified the body
// instead would satisfy "the two names differ" just as well, and is the
// mutation this test has to be able to see.
func TestUnit_ResolveParameters_CrossOriginCollision_BodyKeepsThePlainName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		params    []specnaming.Parameter
		bodyNames []string
		want      []string
	}{
		{
			name:      "StackMigrate: query parameter versus a required body property",
			params:    []specnaming.Parameter{{Name: "id", Origin: specnaming.OriginPath}, {Name: "endpointId", Origin: specnaming.OriginQuery}},
			bodyNames: []string{"endpointId", "isHelm", "name", "namespace", "swarmId"},
			want:      []string{"id", "endpointIdQuery"},
		},
		{
			name:      "CreateKubernetesIngress: path parameter versus a body property",
			params:    []specnaming.Parameter{{Name: "id", Origin: specnaming.OriginPath}, {Name: "namespace", Origin: specnaming.OriginPath}},
			bodyNames: []string{"name", "namespace"},
			want:      []string{"id", "namespacePath"},
		},
		{
			name:      "the same name from two locations is qualified by each location, not merged",
			params:    []specnaming.Parameter{{Name: "namespace", Origin: specnaming.OriginPath}, {Name: "namespace", Origin: specnaming.OriginQuery}},
			bodyNames: []string{"namespace"},
			want:      []string{"namespacePath", "namespaceQuery"},
		},
		{
			name:      "a header parameter is qualified by its own origin",
			params:    []specnaming.Parameter{{Name: "token", Origin: specnaming.OriginHeader}},
			bodyNames: []string{"token"},
			want:      []string{"tokenHeader"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := specnaming.ResolveParameters(tc.params, tc.bodyNames)
			if err != nil {
				t.Fatalf("ResolveParameters() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveParameters() = %v, want %v", got, tc.want)
			}
			// The body's own names are never touched by this rule: it
			// returns nothing for them precisely so no caller can be
			// tempted to rename one. Asserted here as a property of the
			// contract, not merely of this call: every returned name that
			// changed must be a parameter's.
			for i, p := range tc.params {
				if got[i] != p.Name && !strings.HasPrefix(got[i], p.Name) {
					t.Errorf("parameter %q was renamed to %q, which does not extend its own name; the rule qualifies, it does not rewrite", p.Name, got[i])
				}
			}
		})
	}
}

// TestUnit_ResolveParameters_QualifiedNameCollidesWithAThirdContributor_Refuses
// covers the one case the rule cannot resolve: the qualified name is itself
// already taken. Renaming into an occupied name is exactly the silent
// shadowing the refusal existed to prevent, so this must refuse rather than
// qualify twice or invent a suffix.
func TestUnit_ResolveParameters_QualifiedNameCollidesWithAThirdContributor_Refuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		params    []specnaming.Parameter
		bodyNames []string
		wantIn    string
	}{
		{
			name:      "the qualified name is already a body property",
			params:    []specnaming.Parameter{{Name: "namespace", Origin: specnaming.OriginPath}},
			bodyNames: []string{"namespace", "namespacePath"},
			wantIn:    "namespacePath",
		},
		{
			name:      "the qualified name is already another parameter",
			params:    []specnaming.Parameter{{Name: "namespace", Origin: specnaming.OriginPath}, {Name: "namespacePath", Origin: specnaming.OriginQuery}},
			bodyNames: []string{"namespace"},
			wantIn:    "namespacePath",
		},
		{
			name:      "a parameter with no origin cannot be qualified at all",
			params:    []specnaming.Parameter{{Name: "namespace", Origin: ""}},
			bodyNames: []string{"namespace"},
			wantIn:    "namespace",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := specnaming.ResolveParameters(tc.params, tc.bodyNames)
			if err == nil {
				t.Fatalf("ResolveParameters() = %v, error = nil; want a refusal", got)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("ResolveParameters() error = %q, want it to name %q", err, tc.wantIn)
			}
		})
	}
}

// TestUnit_ResolveParameters_IsIndependentOfInputOrder is the property the
// original refusal was written to protect. A silent shadow made an
// operation's shape depend on which contributor a map happened to yield
// first; a disambiguation rule that depended on input order would
// reintroduce exactly that, one level up. Every permutation of the
// parameters and of the body names must produce the same name for the same
// parameter.
func TestUnit_ResolveParameters_IsIndependentOfInputOrder(t *testing.T) {
	t.Parallel()
	params := []specnaming.Parameter{
		{Name: "id", Origin: specnaming.OriginPath},
		{Name: "namespace", Origin: specnaming.OriginPath},
		{Name: "endpointId", Origin: specnaming.OriginQuery},
	}
	bodyNames := []string{"endpointId", "name", "namespace"}

	want := map[string]string{}
	base, err := specnaming.ResolveParameters(params, bodyNames)
	if err != nil {
		t.Fatalf("ResolveParameters() error = %v", err)
	}
	for i, p := range params {
		want[p.Origin+" "+p.Name] = base[i]
	}

	for _, permutedParams := range permutations(params) {
		for _, permutedBody := range permutationsOfStrings(bodyNames) {
			got, err := specnaming.ResolveParameters(permutedParams, permutedBody)
			if err != nil {
				t.Fatalf("ResolveParameters(%v, %v) error = %v", permutedParams, permutedBody, err)
			}
			for i, p := range permutedParams {
				key := p.Origin + " " + p.Name
				if got[i] != want[key] {
					t.Errorf("parameter %s resolved to %q with parameters %v and body %v, but to %q in declaration order: the rule depends on input order",
						key, got[i], permutedParams, permutedBody, want[key])
				}
			}
		}
	}
}

// TestUnit_ResolveParameters_RefusalIsIndependentOfInputOrder pins the same
// property for the failing path: a refusal that only fired for some orders
// would be a shape that silently depends on iteration order for the rest.
func TestUnit_ResolveParameters_RefusalIsIndependentOfInputOrder(t *testing.T) {
	t.Parallel()
	params := []specnaming.Parameter{
		{Name: "namespace", Origin: specnaming.OriginPath},
		{Name: "id", Origin: specnaming.OriginPath},
	}
	bodyNames := []string{"namespace", "namespacePath", "name"}
	var messages []string
	for _, permutedParams := range permutations(params) {
		for _, permutedBody := range permutationsOfStrings(bodyNames) {
			_, err := specnaming.ResolveParameters(permutedParams, permutedBody)
			if err == nil {
				t.Fatalf("ResolveParameters(%v, %v) error = nil, want the same refusal every order produces", permutedParams, permutedBody)
			}
			messages = append(messages, err.Error())
		}
	}
	sort.Strings(messages)
	if messages[0] != messages[len(messages)-1] {
		t.Errorf("refusal message depends on input order: %q vs %q", messages[0], messages[len(messages)-1])
	}
}

// TestUnit_Qualify_AppendsTheOriginInLowerCamelCase pins the rendered form
// itself, so a future edit that changes the suffix into a prefix, or a
// separator, is a deliberate change to a named expectation rather than a
// silent rename of every disambiguated field in the catalog.
func TestUnit_Qualify_AppendsTheOriginInLowerCamelCase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, origin, want string }{
		{"endpointId", specnaming.OriginQuery, "endpointIdQuery"},
		{"namespace", specnaming.OriginPath, "namespacePath"},
		{"token", specnaming.OriginHeader, "tokenHeader"},
	} {
		t.Run(tc.name+"/"+tc.origin, func(t *testing.T) {
			got, err := specnaming.Qualify(tc.name, tc.origin)
			if err != nil {
				t.Fatalf("Qualify(%q, %q) error = %v", tc.name, tc.origin, err)
			}
			if got != tc.want {
				t.Errorf("Qualify(%q, %q) = %q, want %q", tc.name, tc.origin, got, tc.want)
			}
		})
	}
}

// TestUnit_Qualify_RefusesWhatItCannotQualify covers the two inputs that
// have no qualified form at all: no origin to append, and no name to append
// it to.
func TestUnit_Qualify_RefusesWhatItCannotQualify(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, origin string }{
		{"namespace", ""},
		{"", specnaming.OriginQuery},
		{"namespace", "body"},
	} {
		t.Run(tc.name+"/"+tc.origin, func(t *testing.T) {
			if got, err := specnaming.Qualify(tc.name, tc.origin); err == nil {
				t.Errorf("Qualify(%q, %q) = %q, error = nil; want a refusal", tc.name, tc.origin, got)
			}
		})
	}
}

// permutations returns every ordering of params.
func permutations(params []specnaming.Parameter) [][]specnaming.Parameter {
	if len(params) <= 1 {
		return [][]specnaming.Parameter{append([]specnaming.Parameter(nil), params...)}
	}
	var out [][]specnaming.Parameter
	for i := range params {
		rest := make([]specnaming.Parameter, 0, len(params)-1)
		rest = append(rest, params[:i]...)
		rest = append(rest, params[i+1:]...)
		for _, p := range permutations(rest) {
			out = append(out, append([]specnaming.Parameter{params[i]}, p...))
		}
	}
	return out
}

// permutationsOfStrings returns every ordering of names.
func permutationsOfStrings(names []string) [][]string {
	if len(names) <= 1 {
		return [][]string{append([]string(nil), names...)}
	}
	var out [][]string
	for i := range names {
		rest := make([]string, 0, len(names)-1)
		rest = append(rest, names[:i]...)
		rest = append(rest, names[i+1:]...)
		for _, p := range permutationsOfStrings(rest) {
			out = append(out, append([]string{names[i]}, p...))
		}
	}
	return out
}
