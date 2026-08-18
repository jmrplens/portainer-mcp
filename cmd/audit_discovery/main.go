// Command audit_discovery reports which sibling actions cannot yet be told
// apart.
//
// P3 grows the catalog from 19 actions across 3 domains to 441 across 44, and
// every one of them needs narrative discovery metadata (Usage,
// RelatedActions, ParameterGuidance) written by hand. Nobody writes 441 sets
// of that from scratch: the sibling project jmrplens/gitlab-mcp-server closed
// the same backlog to zero across 1062 actions by clustering actions on
// (domain, base name with CRUD and variant suffixes stripped) and working
// only the clusters where two or more siblings turned out indistinguishable —
// a few dozen pairs, not the whole catalog. This command reproduces that
// mechanism.
//
// A cluster is a set of actions in the same domain whose local name reduces
// to the same base once a leading or trailing CRUD verb (list, create,
// delete, ...) or a trailing variant qualifier (_all, _summary, ...) is
// stripped — see baseName. A singleton cluster is never reported: with only
// one member there is nothing to tell it apart from. A cluster of two or more
// is flagged when two or more of its members share the exact same Usage text,
// including two that both leave it empty — text that reads identically for
// every member cannot help a model choose between them, which is exactly the
// property this tool exists to surface.
//
// Report, do not gate. Discovery quality is a judgement call: gating on it
// invites satisfying the gate with filler text instead of writing text that
// actually helps. This command always exits 0 unless the catalog itself
// fails to build; the backlog it prints is informational, and Task 5's
// audit_1to1 is the only audit in this plan that blocks the build.
//
// Built against Business Edition with no version ceiling — Options{Edition:
// edition.EE} and an empty ServerVersion — so every declared action, in
// either edition, is included in its own backlog rather than silently
// filtered out of the one report meant to cover all of them.
//
// It never writes to standard output: that stream carries the MCP JSON-RPC
// transport for the real server binary, and this repository's CI guard bans
// writes to it (and bare fmt.Print* calls) module-wide, this command
// included. The report goes to stderr, following cmd/fetch_spec and
// cmd/audit_e2e_gaps's precedent.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/custom_templates"
	"github.com/jmrplens/portainer-mcp/internal/tools/docker"
	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/tools/tags"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func main() {
	if err := run(os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "audit_discovery: %v\n", err)
		os.Exit(1)
	}
}

// run builds the catalog, clusters its actions, and writes the discovery
// backlog report to w.
func run(w io.Writer) error {
	catalog, err := actioncatalog.Build(allSpecs(), actioncatalog.Options{Edition: edition.EE})
	if err != nil {
		return fmt.Errorf("build action catalog: %w", err)
	}

	clusters := clusterActions(catalog.Actions())

	_, err = fmt.Fprint(w, buildReport(clusters))
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// allSpecs collects every domain's declared actions. Each pilot domain
// package is added by hand today; P3 grows this list alongside the domain
// packages themselves.
func allSpecs() []toolutil.ActionSpec {
	var specs []toolutil.ActionSpec
	specs = append(specs, system.Specs()...)
	specs = append(specs, tags.Specs()...)
	specs = append(specs, registries.Specs()...)
	specs = append(specs, docker.Specs()...)
	specs = append(specs, custom_templates.Specs()...)
	return specs
}

// crudVerbs are the operation words this tool recognises as forming a family
// of actions on the same underlying resource when they lead or trail an
// action's local name — "list" and "list_all" both belong to the "list"
// family, "delete" and "repository_tags_delete" both belong to the "delete"
// family. The set intentionally stays narrow: it names verbs, not resource
// nouns, so that two actions on genuinely different resources (such as
// ecr_delete_repository and ecr_delete_tags, which share no recognised token
// at either end) are never forced into the same cluster.
var crudVerbs = map[string]bool{
	"list": true, "get": true, "create": true, "update": true,
	"delete": true, "remove": true, "add": true, "set": true,
	"inspect": true, "configure": true, "ping": true, "upgrade": true,
	"install": true, "uninstall": true, "enable": true, "disable": true,
	"start": true, "stop": true, "restart": true, "pause": true, "resume": true,
	"join": true, "leave": true, "invite": true, "revoke": true,
	"assign": true, "unassign": true, "attach": true, "detach": true,
	"import": true, "export": true, "execute": true, "exec": true, "run": true,
	"cancel": true, "retry": true, "connect": true, "disconnect": true,
	"lock": true, "unlock": true, "activate": true, "deactivate": true,
	"deploy": true, "redeploy": true, "rollback": true, "backup": true,
	"restore": true, "duplicate": true, "clone": true, "move": true,
	"rename": true, "search": true, "validate": true, "download": true,
	"upload": true, "generate": true, "refresh": true, "reload": true,
}

// variantSuffixes are trailing qualifiers this tool recognises as narrowing
// an operation rather than naming a different one — "list_all" and
// "list_summary" both belong to the "list" family. Kept as narrow as
// crudVerbs and for the same reason: a resource noun (like "repository" or
// "tags") must never be stripped here, or unrelated resources would collapse
// into one cluster.
var variantSuffixes = map[string]bool{
	"all": true, "details": true, "summary": true, "short": true,
	"mine": true, "count": true, "associated": true, "extended": true,
	"basic": true, "full": true, "brief": true, "minimal": true,
}

// clusterKey identifies one sibling cluster: a domain and the base name
// shared by every member.
type clusterKey struct {
	Domain string
	Base   string
}

// cluster is one group of actions sharing a domain and base name. Actions is
// sorted by name for deterministic reporting.
type cluster struct {
	Key     clusterKey
	Actions []toolutil.ActionSpec
}

// clusterActions groups actions by (domain, base name with CRUD and variant
// suffixes stripped). The result is sorted by domain then base name so the
// report is deterministic across runs.
func clusterActions(actions []toolutil.ActionSpec) []cluster {
	byKey := map[clusterKey][]toolutil.ActionSpec{}
	for _, a := range actions {
		key := clusterKey{Domain: a.Domain, Base: baseName(a.Name, a.Domain)}
		byKey[key] = append(byKey[key], a)
	}

	clusters := make([]cluster, 0, len(byKey))
	for key, members := range byKey {
		sorted := append([]toolutil.ActionSpec(nil), members...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
		clusters = append(clusters, cluster{Key: key, Actions: sorted})
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Key.Domain != clusters[j].Key.Domain {
			return clusters[i].Key.Domain < clusters[j].Key.Domain
		}
		return clusters[i].Key.Base < clusters[j].Key.Base
	})
	return clusters
}

// baseName reduces name's local part (name with domain's "domain." prefix
// removed) to the token that identifies its operation family:
//
//   - If the local name's leading underscore-separated token is a recognised
//     CRUD verb, that verb is the base: "list" and "list_all" both reduce to
//     "list".
//   - Otherwise, if the trailing token is a recognised CRUD verb or variant
//     suffix, that trailing token is the base: "repository_tags_delete"
//     reduces to "delete", the same base as the bare "delete" action.
//   - Otherwise the local name is its own base, singleton by construction:
//     "ecr_delete_repository" and "ecr_delete_tags" share no recognised token
//     at either end, so each stays distinct and neither clusters with plain
//     "delete".
//
// This is a name-level heuristic, not a semantic one: two actions that mean
// nearly the same thing in English but use different verbs (system.update and
// system.upgrade, for instance) do not cluster, because they share no literal
// token. That mirrors the sibling project's own mechanism and keeps the
// clustering cheap and predictable rather than a judgement call in itself.
func baseName(name, domain string) string {
	local := strings.TrimPrefix(name, domain+".")
	tokens := strings.Split(local, "_")
	if len(tokens) == 0 {
		return local
	}
	if crudVerbs[tokens[0]] {
		return tokens[0]
	}
	if last := tokens[len(tokens)-1]; len(tokens) > 1 && (crudVerbs[last] || variantSuffixes[last]) {
		return last
	}
	return local
}

// duplicateGroup is two or more actions in the same cluster whose Usage text
// is character-for-character identical, and so cannot help a model tell them
// apart.
type duplicateGroup struct {
	Usage   string
	Actions []string
}

// needsAttention reports the disambiguation gaps within one cluster.
//
// A singleton cluster always returns nil: with one member there is nothing
// to disambiguate it from, regardless of what its Usage says — flagging it
// would blame an action for a comparison that does not exist. A cluster of
// two or more is flagged group by group: every subset of members sharing the
// exact same Usage text is one group, because that shared text reads
// identically for each of them.
func needsAttention(c cluster) []duplicateGroup {
	if len(c.Actions) < 2 {
		return nil
	}

	byUsage := map[string][]string{}
	var order []string
	for _, a := range c.Actions {
		if _, seen := byUsage[a.Usage]; !seen {
			order = append(order, a.Usage)
		}
		byUsage[a.Usage] = append(byUsage[a.Usage], a.Name)
	}

	var groups []duplicateGroup
	for _, usage := range order {
		names := byUsage[usage]
		if len(names) < 2 {
			continue
		}
		groups = append(groups, duplicateGroup{Usage: usage, Actions: names})
	}
	return groups
}

// buildReport renders the discovery backlog: how many sibling clusters exist
// (clusters with two or more members — a singleton is never a "sibling" of
// anything), how many of those need attention, and the specific groups that
// do, each showing the shared Usage text so a reader knows exactly what to
// rewrite.
func buildReport(clusters []cluster) string {
	var siblingClusters int
	type flaggedCluster struct {
		Key    clusterKey
		Groups []duplicateGroup
	}
	var flagged []flaggedCluster

	for _, c := range clusters {
		if len(c.Actions) < 2 {
			continue
		}
		siblingClusters++
		if groups := needsAttention(c); len(groups) > 0 {
			flagged = append(flagged, flaggedCluster{Key: c.Key, Groups: groups})
		}
	}

	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP discovery audit")
	fmt.Fprintln(&b, "=============================")
	fmt.Fprintf(&b, "Sibling clusters:      %d\n", siblingClusters)
	fmt.Fprintf(&b, "Needing attention:     %d of %d\n", len(flagged), siblingClusters)

	if len(flagged) == 0 {
		fmt.Fprintln(&b, "\nEvery sibling cluster has distinguishing Usage text.")
		return b.String()
	}

	fmt.Fprintln(&b, "\nClusters where siblings cannot be told apart:")
	for _, fc := range flagged {
		fmt.Fprintf(&b, "\n  %s (base %q):\n", fc.Key.Domain, fc.Key.Base)
		for _, g := range fc.Groups {
			usage := g.Usage
			if usage == "" {
				usage = "(no Usage text)"
			} else {
				usage = fmt.Sprintf("%q", usage)
			}
			fmt.Fprintf(&b, "    - %s share identical Usage: %s\n", strings.Join(g.Actions, ", "), usage)
		}
	}
	return b.String()
}
