package main

import (
	"fmt"
	"strings"
	"unicode"
)

// commonInitialisms are the Go-idiomatic initialisms this generator
// upper-cases in full when they appear as one word of an identifier (e.g.
// "endpointId" -> "EndpointID", "URL" -> "URL"), matching the convention the
// pilot domains' hand-written Input structs already followed (registryInspectInput.EndpointID,
// registryCreateInput.URL, registryConfigureInput.TLSSkipVerify) rather than
// oapi-codegen's own plainer "Id" style.
//
// This is the standard list golint used (golang/lint's commonInitialisms),
// plus none added: adding a Portainer-specific abbreviation (say "ECR") would
// force-uppercase spec property names that are not actually written as
// initialisms in the vendored spec (registries.registryCreatePayload declares
// "Ecr", not "ECR").
var commonInitialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true, "CSS": true, "DNS": true,
	"EOF": true, "GUID": true, "HTML": true, "HTTP": true, "HTTPS": true, "ID": true,
	"IP": true, "JSON": true, "LHS": true, "QPS": true, "RAM": true, "RHS": true,
	"RPC": true, "SLA": true, "SMTP": true, "SQL": true, "SSH": true, "TCP": true,
	"TLS": true, "TTL": true, "UDP": true, "UI": true, "UID": true, "UUID": true,
	"URI": true, "URL": true, "UTF8": true, "VM": true, "XML": true, "XMPP": true,
	"XSRF": true, "XSS": true,
}

// splitWords breaks a wire identifier — whether the lower-camel style OpenAPI
// uses for path/query parameter names ("endpointId", "tlsSkipVerify") or the
// PascalCase style this Portainer API's own payload schemas use for body
// properties ("BaseURL", "Authentication") — into its component words.
//
// Each maximal run of uppercase letters is handed to consumeInitialisms,
// which greedily matches known initialisms within it rather than treating
// the whole run as one unrecognised word — the difference between reading
// "TLSCACertFile" as "TLS"+"CA"+"Cert"+"File" and misreading it as a single
// "Tlsca" word. If that run is immediately followed by a lowercase letter and
// consumeInitialisms' last word is a single unmatched capital, that capital
// is merged onto the following lowercase run instead of standing alone: the
// last capital of an acronym run belongs to the word after it, the same
// property that makes "BaseURL" split as "Base"+"URL" (not "Base"+"U"+"RL")
// and "endpointId" split as "endpoint"+"Id" (not "endpoint"+"I"+"d").
func splitWords(name string) []string {
	if name == "" {
		return nil
	}
	runes := []rune(name)
	n := len(runes)
	var words []string
	i := 0
	for i < n {
		if !unicode.IsUpper(runes[i]) {
			j := i + 1
			for j < n && unicode.IsLower(runes[j]) {
				j++
			}
			words = append(words, string(runes[i:j]))
			i = j
			continue
		}

		j := i
		for j < n && unicode.IsUpper(runes[j]) {
			j++
		}
		run := consumeInitialisms(runes[i:j])
		if j < n && unicode.IsLower(runes[j]) && len(run) > 0 && len([]rune(run[len(run)-1])) == 1 {
			k := j
			for k < n && unicode.IsLower(runes[k]) {
				k++
			}
			run[len(run)-1] += string(runes[j:k])
			j = k
		}
		words = append(words, run...)
		i = j
	}
	return words
}

// consumeInitialisms splits an uppercase-only rune run into words by
// greedily matching the longest recognised initialism at each position. A
// position where no initialism matches falls back to a single-letter word,
// so an unrecognised acronym still splits into letters that render the same
// whether upper-cased or title-cased (each contributes exactly one letter to
// a JSON tag either way) rather than being swallowed into one mis-cased word.
func consumeInitialisms(run []rune) []string {
	var words []string
	for len(run) > 0 {
		matched := false
		for length := len(run); length >= 2; length-- {
			candidate := string(run[:length])
			if commonInitialisms[candidate] {
				words = append(words, candidate)
				run = run[length:]
				matched = true
				break
			}
		}
		if !matched {
			words = append(words, string(run[0]))
			run = run[1:]
		}
	}
	return words
}

// goFieldName renders wire identifier words as an exported Go field name:
// each word is title-cased, except a word recognised as a common initialism
// (case-insensitively), which is rendered fully upper-case.
//
// goFieldName is not injective, and neither is bodyJSONTag below: two
// distinct wire property names can render to the same output. "TLSSkipVerify"
// and "TlsSkipVerify" both produce "tlsSkipVerify" via bodyJSONTag (and the
// identical "TLSSkipVerify" via this function); "baseUrl" and "BaseURL" both
// produce "baseUrl" via bodyJSONTag too. Neither function can detect this on
// its own — each only ever sees one name at a time — so a caller assembling
// more than one field into the same struct must track the rendered names
// across the whole set and refuse a repeat itself; see assembleFields, which
// does exactly that for every struct this generator emits.
func goFieldName(words []string) string {
	var out []rune
	for _, w := range words {
		if commonInitialisms[upper(w)] {
			out = append(out, []rune(upper(w))...)
			continue
		}
		out = append(out, []rune(title(w))...)
	}
	return string(out)
}

// bodyJSONTag renders wire identifier words as the lower-camel-case JSON tag
// this project publishes to a model, regardless of how the underlying
// Portainer wire schema happened to capitalise the property.
//
// The first word is rendered fully lower-case; every following word is
// title-cased with only its first letter capitalised — never fully upper,
// even for a recognised initialism. This reproduces the exact convention the
// pilot domains' hand-written Input structs already used: "BaseURL" (body
// property) -> "baseUrl", "TLSSkipVerify" -> "tlsSkipVerify". Path and query
// parameter names are not run through this function at all: OpenAPI already
// declares those in lower-camel-case, and the wire identifier is used as the
// tag verbatim.
func bodyJSONTag(words []string) string {
	if len(words) == 0 {
		return ""
	}
	out := []rune(lower(words[0]))
	for _, w := range words[1:] {
		out = append(out, []rune(title(w))...)
	}
	return string(out)
}

func upper(s string) string {
	return string(mapRunes(s, unicode.ToUpper))
}

func lower(s string) string {
	return string(mapRunes(s, unicode.ToLower))
}

// title renders s with its first rune upper-cased and every other rune
// lower-cased, regardless of how s was originally cased.
func title(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	out := make([]rune, len(r))
	out[0] = unicode.ToUpper(r[0])
	for i := 1; i < len(r); i++ {
		out[i] = unicode.ToLower(r[i])
	}
	return string(out)
}

func mapRunes(s string, f func(rune) rune) []rune {
	r := []rune(s)
	out := make([]rune, len(r))
	for i, c := range r {
		out[i] = f(c)
	}
	return out
}

// exportedName upper-cases the first rune of a raw OpenAPI operationId,
// matching both oapi-codegen's own naming pass and cmd/gen_applicability's
// identical transform: every operationId in the vendored specs is plain ASCII
// camelCase or PascalCase with no separators, so this single-rune change is
// the whole transformation.
func exportedName(id string) string {
	if id == "" {
		return id
	}
	r := []rune(id)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// inputStructName derives the Input struct's name from an operation's
// exported OperationID, matching the pilot domains' existing hand-written
// names exactly: "TagCreate" -> "tagCreateInput", "EcrDeleteRepository" ->
// "ecrDeleteRepositoryInput". This is what lets a regenerated inputs.gen.go
// slot in as a drop-in replacement: the domain's Specs() function already
// writes `Input: tagCreateInput{}` and every handler already declares `var
// params tagCreateInput`.
func inputStructName(operationID string) string {
	r := []rune(operationID)
	if len(r) == 0 {
		return "input"
	}
	r[0] = unicode.ToLower(r[0])
	return string(r) + "Input"
}

// ActionName derives the canonical, domain-qualified action name (the
// ActionSpec.Name a model searches for through portainer_find_action and
// calls portainer_execute_action with) from a domain package name and an
// operation's exported OperationID: "TagCreate" in domain "tags" becomes
// "tags.create".
//
// The local part is the OperationID with the domain's own prefix removed,
// lower-cased and snake_cased. That prefix is not simply the domain string:
// three cases actually occur across the vendored spec's 46 tags, tried in
// this order against every operation independently (a domain is not
// committed to one shape for every one of its operations) —
//
//  1. the domain name itself, underscores removed ("settings" matches
//     SettingsInspect's "Settings", "AllowList" for domain "allowlist");
//  2. the domain's last word singularised, underscores removed ("tags" ->
//     "tag" matches TagCreate; "registries" -> "registry" matches
//     RegistryList; "team_memberships" -> "teammembership" matches
//     TeamMembershipCreate) — the shape the pilot domains' own hand-written
//     names already assume;
//  3. neither, in which case the whole OperationID is kept and no prefix is
//     removed at all (RepositoryTagsDelete and Ecr* keep their full name in
//     domain "registries": neither "registries" nor "registry" prefixes
//     them).
//
// A plain strings.TrimPrefix(operationID, domain) is not this rule: it would
// also strip "Auth" from "AuthenticateUser" in domain "auth" (wrong — the
// word is "Authenticate", not "Auth" followed by a new word), so a match
// only counts when what follows is either the end of the identifier or the
// start of a new word (an upper-case rune) — never mid-word. Comparison
// itself is case-insensitive throughout, since operationID is PascalCase and
// domain is snake_case lower-case.
//
// Stripping a prefix that consumes the OperationID whole (remainder empty)
// is treated as a real match, not "no match, keep the whole thing" — domain
// "motd"'s only operation is itself "MOTD", and domain "team_memberships"
// has a bare "TeamMemberships" alongside its TeamMembership* siblings. Both
// leave nothing after their own domain name is removed, and this is refused
// below rather than silently emitting "motd.motd": a domain hitting this
// must declare that one action's name by hand instead of generating it.
func ActionName(domain, operationID string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("gen_action_inputs: ActionName: domain must not be empty")
	}
	if operationID == "" {
		return "", fmt.Errorf("gen_action_inputs: ActionName: operationID must not be empty")
	}

	remainder := stripDomainPrefix(operationID, domainPrefixCandidates(domain))

	words := splitWords(remainder)
	parts := make([]string, 0, len(words))
	for _, w := range words {
		parts = append(parts, lower(w))
	}
	local := strings.Join(parts, "_")
	if local == "" {
		return "", fmt.Errorf(
			"gen_action_inputs: ActionName: operationID %q leaves nothing after removing domain %q's own prefix; declare this action's Name by hand instead of generating it",
			operationID, domain)
	}
	return domain + "." + local, nil
}

// domainPrefixCandidates returns the prefix(es) ActionName tries against an
// operationID for domain, longest-shape-first: the domain name itself
// (underscores removed), then the same with its last underscore-separated
// word singularised, when that differs. Both are returned lower-case;
// stripDomainPrefix folds the operationID's case to compare against them, so
// neither candidate needs any case massaging of its own.
func domainPrefixCandidates(domain string) []string {
	segments := strings.Split(domain, "_")
	asIs := strings.ToLower(strings.Join(segments, ""))
	candidates := []string{asIs}

	last := segments[len(segments)-1]
	if sing := singularize(last); sing != last {
		singSegments := make([]string, len(segments))
		copy(singSegments, segments)
		singSegments[len(singSegments)-1] = sing
		if singular := strings.ToLower(strings.Join(singSegments, "")); singular != asIs {
			candidates = append(candidates, singular)
		}
	}
	return candidates
}

// singularize applies the minimal English singularisation the vendored
// spec's ~46 domain names actually need: "ies" -> "y" (registries ->
// registry, policies -> policy) and a plain trailing "s" dropped otherwise
// (tags -> tag, addons -> addon, memberships -> membership). It is not a
// general solution — "kubernetes" becomes the nonsense "kubernete" — but
// nothing in the vendored spec ever begins with that string, so the
// nonsense candidate simply never matches rather than matching wrongly.
func singularize(word string) string {
	lowerWord := strings.ToLower(word)
	switch {
	case strings.HasSuffix(lowerWord, "ies") && len(lowerWord) > 3:
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(lowerWord, "s") && !strings.HasSuffix(lowerWord, "ss") && len(lowerWord) > 1:
		return word[:len(word)-1]
	default:
		return word
	}
}

// stripDomainPrefix returns the part of operationID left after removing the
// first candidate that prefixes it at a word boundary (case-insensitively),
// or operationID unchanged if none do. A candidate that consumes operationID
// whole returns "" — a deliberate, real match ActionName above turns into a
// refusal, not a signal to fall through to the next candidate or keep the
// original.
//
// operationID is always plain ASCII (see exportedName's doc comment), so
// indexing by byte offset is exact rather than an approximation of rune
// boundaries.
func stripDomainPrefix(operationID string, candidates []string) string {
	lowerID := strings.ToLower(operationID)
	for _, prefix := range candidates {
		if prefix == "" || len(lowerID) < len(prefix) || lowerID[:len(prefix)] != prefix {
			continue
		}
		if len(operationID) == len(prefix) {
			return ""
		}
		if next := rune(operationID[len(prefix)]); unicode.IsUpper(next) {
			return operationID[len(prefix):]
		}
	}
	return operationID
}
