package toolutil

import "fmt"

// DomainTags is the curated, one-source-of-truth mapping from a domain
// package name — the identifier shared by an internal/tools/<domain>
// directory and every action declared inside it (ActionSpec.Domain) — to the
// OpenAPI tag(s) whose operations that domain covers.
//
// It exists because "directory name", "OpenAPI tag" and "first URL path
// segment" are three different strings for several of the vendored spec's 46
// tags, and assuming any two of them coincide is how a domain silently loses
// its operations. cmd/gen_action_inputs groups operations by tag (the only
// grouping the spec itself declares) and looks up a domain directory's
// operations through this table, not by matching the directory's own name
// against a tag or a path segment — a domain directory absent from this
// table, or a tag with operations that no domain here claims, is a hard
// error there rather than a silently empty generated file. See
// cmd/gen_action_inputs's package doc for the defect this closes.
//
// Every domain here covers exactly one tag: the vendored spec already groups
// /backup with /restore, and /cloud/credentials with /sshkeygen, under the
// single tags "backup" and "cloud_credentials" respectively, so there is
// nothing left for this table to merge there. What this table does do is
// choose, for every one of the 46 tags, a directory/domain name a model and a
// domain author can both read without translating: "licenses", not the
// spec's singular "license" (whose operations live under /licenses);
// "edge_configurations", not the spec's abbreviated "edge_configs" (whose
// operations live under /edge_configurations); and "cloud", not the spec's
// "cloud_credentials", for the tag that also covers /sshkeygen. "kaas" keeps
// its own name and stays a separate domain from "cloud" even though one of
// its operations also touches /cloud/endpoints: it is a different tag
// ("kaas") naming a different concept (provisioning a Kubernetes-as-a-Service
// cluster), not a second spelling of cloud-credentials management.
//
// Populated for every tag the vendored specification declares today, not
// only the domains that already have a directory — see
// TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec in
// cmd/gen_action_inputs, which checks this table against the real spec file,
// and ValidateDomainTags below, which checks the table's own consistency.
var DomainTags = map[string][]string{
	"addons":                {"addons"},
	"allowlist":             {"allowlist"},
	"auth":                  {"auth"},
	"auto_updates":          {"auto_updates"},
	"backup":                {"backup"},
	"cloud":                 {"cloud_credentials"},
	"custom_templates":      {"custom_templates"},
	"docker":                {"docker"},
	"edge":                  {"edge"},
	"edge_agent":            {"edge_agent"},
	"edge_configurations":   {"edge_configs"},
	"edge_groups":           {"edge_groups"},
	"edge_jobs":             {"edge_jobs"},
	"edge_stacks":           {"edge_stacks"},
	"edge_update_schedules": {"edge_update_schedules"},
	"endpoint_groups":       {"endpoint_groups"},
	"endpoints":             {"endpoints"},
	"gitops":                {"gitops"},
	"helm":                  {"helm"},
	"kaas":                  {"kaas"},
	"kubernetes":            {"kubernetes"},
	"ldap":                  {"ldap"},
	"licenses":              {"license"},
	"motd":                  {"motd"},
	"observability":         {"observability"},
	"omni":                  {"omni"},
	"policies":              {"policies"},
	"recommendations":       {"recommendations"},
	"registries":            {"registries"},
	"resource_controls":     {"resource_controls"},
	"roles":                 {"roles"},
	"settings":              {"settings"},
	"ssl":                   {"ssl"},
	"stacks":                {"stacks"},
	"status":                {"status"},
	"support":               {"support"},
	"system":                {"system"},
	"tags":                  {"tags"},
	"team_memberships":      {"team_memberships"},
	"teams":                 {"teams"},
	"templates":             {"templates"},
	"upload":                {"upload"},
	"useractivity":          {"useractivity"},
	"users":                 {"users"},
	"webhooks":              {"webhooks"},
	"websocket":             {"websocket"},
}

// ValidateDomainTags checks domainTags's own internal consistency: every
// domain names at least one tag, and no tag is claimed by more than one
// domain. A tag claimed twice would make "which domain does this operation
// belong to" a matter of map iteration order rather than a fact every reader
// of the table can rely on.
//
// Takes the table as a parameter, rather than always checking the package
// var, so a test can exercise both a valid and a deliberately broken table
// without mutating shared state.
func ValidateDomainTags(domainTags map[string][]string) error {
	seen := make(map[string]string, len(domainTags))
	for domain, tags := range domainTags {
		if len(tags) == 0 {
			return fmt.Errorf("toolutil: domain %q lists no tags in DomainTags", domain)
		}
		for _, tag := range tags {
			if other, dup := seen[tag]; dup {
				return fmt.Errorf("toolutil: tag %q is claimed by both domain %q and domain %q in DomainTags", tag, other, domain)
			}
			seen[tag] = domain
		}
	}
	return nil
}
