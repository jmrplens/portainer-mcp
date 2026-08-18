package main

import (
	"fmt"
	"sort"
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
// (case-insensitively), which is rendered fully upper-case — and, since a
// plural initialism ("TagIDs", "ClusterIPs") is common throughout the
// vendored spec, a trailing lower-case "s" pluraliser attached to a
// recognised initialism is rendered lower-case after it ("IDs", "IPs"),
// never upper-cased into "IDS"/"IPS" the way an ordinary word's title-casing
// would. See trailingPluralInitialism and isPluralSuffixWord's doc comments
// for the two distinct shapes splitWords hands this function for the same
// wire convention, and
// TestUnit_GoFieldName_PluralInitialismSuffix_RendersCorrectlyWithUnchangedWireTag
// for both proved against the real spec's 13 affected property names.
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
	for i, w := range words {
		if stem, ok := trailingPluralInitialism(w); ok {
			out = append(out, []rune(upper(stem))...)
			out = append(out, 's')
			continue
		}
		if isPluralSuffixWord(w) && i > 0 && commonInitialisms[upper(words[i-1])] {
			out = append(out, 's')
			continue
		}
		if commonInitialisms[upper(w)] {
			out = append(out, []rune(upper(w))...)
			continue
		}
		out = append(out, []rune(title(w))...)
	}
	return string(out)
}

// trailingPluralInitialism reports whether w is one of splitWords' merged
// words that pairs a recognised initialism with a trailing lower-case "s"
// pluraliser as a single word — the shape produced when the wire name spells
// the initialism with a single, otherwise-unmatched capital letter
// (oapi-codegen's plain style: "TagIds" splits as ["Tag", "Ids"], with "Ids"
// one merged word — see splitWords' doc comment on why a lone unmatched
// capital swallows the whole lower-case run that follows it, rather than
// standing alone). "Ids" itself is not a commonInitialisms entry (only "ID"
// is), so goFieldName's ordinary upper(w) lookup already misses it and would
// otherwise title-case the whole word into "Ids"; this lets the stem ("Id")
// be recognised and upper-cased on its own, with the pluraliser appended
// lower-case, rendering "IDs" instead.
//
// A false match is only possible if some other, unrelated word happens to
// both end in a lower-case "s" and have an initialism for its stem — checked
// against every entry in commonInitialisms when this was added, and true of
// none of them (nothing in the table ends in a letter whose removal leaves
// another entry in the same table).
//
// This is trailingPluralInitialismIn pinned to commonInitialisms, the only
// table goFieldName may ever consult (see goFieldName's own doc comment);
// actionGoFieldName below calls trailingPluralInitialismIn directly, pinned
// to actionInitialisms instead.
func trailingPluralInitialism(w string) (stem string, ok bool) {
	return trailingPluralInitialismIn(w, commonInitialisms)
}

// trailingPluralInitialismIn is trailingPluralInitialism's check,
// parameterized over which initialism table the stem is looked up against.
func trailingPluralInitialismIn(w string, initialisms map[string]bool) (stem string, ok bool) {
	r := []rune(w)
	if len(r) < 2 || r[len(r)-1] != 's' {
		return "", false
	}
	stem = string(r[:len(r)-1])
	if !initialisms[upper(stem)] {
		return "", false
	}
	return stem, true
}

// isPluralSuffixWord reports whether w is the standalone one-letter "s"
// word splitWords emits on its own — the shape produced when the wire name
// spells its initialism out in full, two-or-more capitals, before the
// pluraliser ("TagIDs" splits as ["Tag", "ID", "s"]: the merge that produces
// trailingPluralInitialism's single-word shape only fires when the
// initialism run left exactly one, unmatched capital; a fully-matched
// initialism run of two or more letters never triggers it, so the trailing
// lower-case "s" is left as its own word instead). Left to goFieldName's
// general title(w) case, this lone "s" upper-cases to "S", rendering
// "TagIDS" — worse than never special-casing plurals at all, because a
// recognised initialism is never otherwise followed by a lower-case letter
// in this project's output. goFieldName only takes this branch once it has
// also confirmed the immediately preceding word is itself a recognised
// initialism, so an unrelated word that happens to be split with a trailing
// single "s" word for some other reason is untouched.
func isPluralSuffixWord(w string) bool {
	return w == "s"
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
// "ecrDeleteRepositoryInput". This is what lets a regenerated inputs.go
// slot in as a drop-in replacement: the domain's Specs() function already
// writes `Input: tagCreateInput{}` and every handler already declares `var
// params tagCreateInput`.
//
// operationID is run through splitActionWords/actionGoFieldName — not
// splitWords/goFieldName, the machinery assembleFields uses for every wire
// property name — rather than merely lower-casing its first rune, which is
// what this function did before: a plain rune swap left every one of an
// operationId's own words exactly as the vendored specification happened to
// spell them, so "StackCreateKubernetesUrl" rendered
// "stackCreateKubernetesUrlInput" (golangci-lint's revive var-naming flags
// it: a recognised initialism, "URL", spelled as an ordinary word) and
// "GitOpsSourcesTestById" rendered "gitOpsSourcesTestByIdInput" (same
// defect, "ID"). Routing through actionGoFieldName first renders every
// recognised initialism upper-case regardless of how the source operationId
// spelled it, matching what handlerFuncName below now does for the identical
// reason.
//
// This is deliberately not goFieldName/splitWords, the pair assembleFields
// uses for every generated Go field name from a wire property: an early
// version of this fix routed operationIDs through that pair instead, and
// regenerating "GetKubernetesGPUInfo" through it isolated a genuine
// consumeInitialisms defect (it greedily matches the initialism "UI" inside
// the uppercase run "GPUI", splitting "Info" apart from its own leading "I"
// before goFieldName ever sees it, and renders "GPUINfo") — worse than the
// naive rune swap this replaces, which happened to leave "GPUInfo" alone
// because it never re-split anything. splitActionWords/actionGoFieldName do
// not have that defect for this identifier: actionInitialisms recognises
// "GPU" as a whole initialism (see its own doc comment), so the greedy
// matcher never gets to the wrong reading in the first place. Reusing that
// pair here, rather than teaching splitWords/goFieldName the same fix,
// leaves every wire tag this generator has ever emitted untouched — see
// TestUnit_ActionSplitter_WireTagsStayByteIdentical, which is exactly the
// guard that would fail if this function reached back into that shared
// machinery instead.
//
// Checked against every operationId in both vendored specs (445 total):
// TestUnit_InputStructNameAndHandlerFuncName_OnlyTheseOperationsChangeAcrossBothSpecs
// in naming_test.go is the corpus proof, covering handlerFuncName
// identically since both route through the same pair.
func inputStructName(operationID string) string {
	if operationID == "" {
		return "input"
	}
	name := actionGoFieldName(splitActionWords(operationID))
	if name == "" {
		return "input"
	}
	r := []rune(name)
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
// ResolveActionName (below) is where that hand-declared name is actually
// supplied for the operations this function itself cannot name — Task 2 and
// 3's domain packages call that, not this function, for every operation.
//
// The remainder is split into words by splitActionWords, not splitWords:
// the two have different jobs and different correctness requirements — a
// wire JSON tag must match the vendored API byte for byte, an action name
// only has to read well to a model — and sharing one splitter between them
// means neither job can be served without risking the other. See
// splitActionWords's own doc comment for what that bought and
// TestUnit_ActionSplitter_WireTagsStayByteIdentical for the proof it holds.
func ActionName(domain, operationID string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("gen_action_inputs: ActionName: domain must not be empty")
	}
	if operationID == "" {
		return "", fmt.Errorf("gen_action_inputs: ActionName: operationID must not be empty")
	}

	remainder := stripDomainPrefix(operationID, domainPrefixCandidates(domain))

	words := splitActionWords(remainder)
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

// actionInitialisms is commonInitialisms' counterpart for action names, not
// wire tags: everything commonInitialisms already recognises, plus the
// abbreviations that only ever show up in an operationId, never in a wire
// property name.
//
// It is a deliberately separate table, not commonInitialisms with entries
// added, because commonInitialisms has a second reader: bodyJSONTag renders
// a recognised initialism's later letters lower-case regardless ("TLS" ->
// "Tls" mid-tag), so adding, say, "CA" there to fix
// "SettingsEdgeMTLSCACertificates" would silently turn some future body
// property's wire tag from "tlsCACertFile" into "tlsCaCertFile" the moment
// that property happened to contain "CA" — a wire-format change nobody
// asked for, in a file nobody would think to re-check, discovered only if
// something downstream still parses the old tag.
// TestUnit_ActionSplitter_WireTagsStayByteIdentical is what proves adding an
// entry here can never do that: it regenerates every domain's Input structs
// with actionInitialisms carrying extra entries commonInitialisms does not
// have, and asserts the output does not change by one byte.
//
//   - GPU, RBAC and CSV are real acronyms the vendored spec's operationIds
//     use (GetKubernetesGPUInfo, GetKubernetesRBACStatus, AuthLogsCSV) that
//     commonInitialisms simply never needed for a Go field name.
//   - MTLS and CA are Portainer's own abbreviations for mutual TLS and
//     certificate authority (EndpointMTLSCertificate,
//     SettingsEdgeMTLSCACertificates) — not standard Go-lint initialisms,
//     but exactly as legitimate as TLS itself for a name a model has to
//     read.
var actionInitialisms = func() map[string]bool {
	out := make(map[string]bool, len(commonInitialisms)+8)
	for k, v := range commonInitialisms {
		out[k] = v
	}
	for _, k := range []string{"GPU", "RBAC", "CSV", "MTLS", "CA"} {
		out[k] = true
	}
	return out
}()

// actionSpecialActionWords are whole words splitActionWords recognises by
// exact, case-sensitive literal match before it ever looks at runs of
// upper-case letters. They exist for a shape actionInitialisms cannot
// express at all: a token that is only partly upper-case ("OAuth" is
// upper-case "OA" then lower-case "uth", not an all-caps acronym), so the
// upper-case-run scan below only ever sees "OA" of it and — matched or
// not — has no way to pull "uth" in with it. Checked longest-first so one
// entry can never shadow a longer one that starts the same way.
var actionSpecialActionWords = []string{"OAuth"}

// splitActionWords is splitWords' counterpart for action names: same
// overall shape — scan runs of upper-case letters, hand each to a
// greedy-longest initialism matcher, merge a trailing single unmatched
// capital onto the word that follows it — but answering to what an action
// name needs instead of what a wire tag needs, which turns out to differ in
// two concrete ways this task's own 441-name read-through found:
//
//   - it consults actionInitialisms, not commonInitialisms, so
//     "GetKubernetesGPUInfo" splits as "Get"+"Kubernetes"+"GPU"+"Info", not
//     "Get"+"Kubernetes"+"G"+"P"+"UI"+"nfo" (splitWords finds the real
//     initialism "UI" greedily inside "GPUI" and never gets to reconsider
//     "GPU" as the better read — actionInitialisms recognising "GPU" first
//     is what fixes this, not a smarter search);
//   - a run continues through a digit, not just a lower-case letter, so
//     "BackupToS3" splits as "Backup"+"To"+"S3" and "UpdateK8sPodSecurityRule"
//     as "Update"+"K8s"+"Pod"+"Security"+"Rule" — splitWords treats a digit
//     as ending the current word and starting a new one-character word,
//     which is exactly right for a Go field name (there is no such thing as
//     a digit-continued Go identifier segment) and exactly wrong for an
//     action name a model has to read as one unit.
//
// splitWords itself, commonInitialisms and every one of splitWords' other
// callers (goFieldName, bodyJSONTag, and therefore every wire tag this
// generator has ever emitted) are untouched by any of this: this is a new,
// independent function, not a parameterised version of the old one, so nothing
// about wire-tag rendering can move by adding a word here.
func splitActionWords(name string) []string {
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
			for j < n && isActionWordContinuation(runes[j]) {
				j++
			}
			words = append(words, string(runes[i:j]))
			i = j
			continue
		}

		if word, ok := matchSpecialActionWord(runes[i:]); ok {
			words = append(words, word)
			i += len([]rune(word))
			continue
		}

		j := i
		for j < n && unicode.IsUpper(runes[j]) {
			j++
		}
		run := consumeActionInitialisms(runes[i:j])
		if j < n && isActionWordContinuation(runes[j]) && len(run) > 0 && len([]rune(run[len(run)-1])) == 1 {
			k := j
			for k < n && isActionWordContinuation(runes[k]) {
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

// isActionWordContinuation reports whether r continues a word
// splitActionWords has already started: a lower-case letter, the same as
// splitWords, or — the one difference — a digit, so "S3" and "K8s" merge
// into one token instead of the digit splitting off into its own
// one-character word.
func isActionWordContinuation(r rune) bool {
	return unicode.IsLower(r) || unicode.IsDigit(r)
}

// matchSpecialActionWord reports whether remaining begins with one of
// actionSpecialActionWords, returning the literal word matched.
func matchSpecialActionWord(remaining []rune) (string, bool) {
	for _, word := range actionSpecialActionWords {
		wr := []rune(word)
		if len(remaining) >= len(wr) && string(remaining[:len(wr)]) == word {
			return word, true
		}
	}
	return "", false
}

// consumeActionInitialisms is consumeInitialisms' counterpart for
// actionInitialisms: identical greedy-longest-match algorithm, run against
// the action-name initialism table instead of commonInitialisms. Kept as a
// separate function, rather than a shared one both take a table parameter
// to, so a future edit to either can never accidentally change the other's
// behaviour by changing a shared helper's signature or default.
func consumeActionInitialisms(run []rune) []string {
	var words []string
	for len(run) > 0 {
		matched := false
		for length := len(run); length >= 2; length-- {
			candidate := string(run[:length])
			if actionInitialisms[candidate] {
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

// capitalizeFirstRune upper-cases w's first rune and leaves every other rune
// exactly as it already was — unlike title (used by goFieldName/bodyJSONTag),
// which also forces every following rune lower-case regardless of its
// original casing.
//
// actionGoFieldName's fallback branch below uses this instead of title for
// the same reason exportedName leaves a whole operationId's trailing runes
// alone (see its own doc comment): every word splitActionWords produces
// already carries its own correct casing verbatim from the source
// operationId — a splitter only ever decides word *boundaries*, it never
// touches a rune's case — with one exception, matchSpecialActionWord's
// literal "OAuth": the internal capital "A" is meaningful and title's blanket
// lower-casing would silently destroy it ("OAuth" -> "Oauth", the exact kind
// of surprise regression this function exists to not introduce). A word
// actionInitialisms recognises never reaches this branch at all (the
// initialism check above always matches first), so this only ever fires for
// a word with no recognised initialism reading, where "leave it alone" is
// always the right answer for an operationId's own spelling.
func capitalizeFirstRune(w string) string {
	if w == "" {
		return w
	}
	r := []rune(w)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// actionGoFieldName renders splitActionWords' output as an exported Go
// identifier, the same overall shape as goFieldName (recognised initialisms
// rendered fully upper-case, a trailing plural initialism suffix handled the
// same way) but consulting actionInitialisms instead of commonInitialisms,
// and capitalizeFirstRune instead of title for the word that is neither.
//
// This is a deliberately independent function, not goFieldName parameterized
// over a table: inputStructName and handlerFuncName are its only callers,
// and neither may ever reach bodyJSONTag or a wire property's Go field name
// (see TestUnit_ActionSplitter_WireTagsStayByteIdentical, and
// TestUnit_ActionSplitter_ImprovesNamesWithoutChangingOldSplitterOutput's own
// doc comment on why the two tables are kept apart at all). Keeping this
// function's control flow wholly separate from goFieldName's means neither
// can ever change the other's behaviour by way of a shared helper.
func actionGoFieldName(words []string) string {
	var out []rune
	for i, w := range words {
		if stem, ok := trailingPluralInitialismIn(w, actionInitialisms); ok {
			out = append(out, []rune(upper(stem))...)
			out = append(out, 's')
			continue
		}
		if isPluralSuffixWord(w) && i > 0 && actionInitialisms[upper(words[i-1])] {
			out = append(out, 's')
			continue
		}
		if actionInitialisms[upper(w)] {
			out = append(out, []rune(upper(w))...)
			continue
		}
		out = append(out, []rune(capitalizeFirstRune(w))...)
	}
	return string(out)
}

// namedOperation is one operation the real vendored specification declares,
// paired with the domain directory that claims it. Declared here rather
// than in naming_test.go because validateActionNameOverrides below —
// production code, not a test helper — takes a slice of it; naming_test.go's
// allOperations, which actually builds that slice from the real spec, only
// consumes the type.
type namedOperation struct {
	Domain      string
	OperationID string
}

// actionNameOverride is one operationId ActionName's mechanical rule cannot
// turn into a usable name at all, paired with the name to use instead and,
// in Reason, why the rule falls short — so the next person to read this
// table does not have to re-derive the argument this task already had.
type actionNameOverride struct {
	// Domain is the package this operationId belongs to. ResolveActionName
	// checks it against its own domain argument so a copy-paste mistake
	// wiring the wrong override to the wrong domain fails loudly instead of
	// silently handing back a name for an unrelated operation.
	Domain string
	Name   string
	Reason string
}

// actionNameOverrides holds every operationId this task found ActionName
// cannot name on its own: three because stripping the operation's own
// domain prefix leaves nothing at all (ActionName refuses these outright,
// so there is no mechanical output to prefer over), and one, "Generate",
// because the mechanical rule succeeds but produces a name with no
// information in it at all.
//
// This table is not exhaustive over the 423 actions still to come — most
// operationIds will never need an entry — but Task 2 and Task 3 will find
// more of the first kind (an operationId identical to its own domain's
// prefix) as they write the remaining domains, and ResolveActionName is the
// mechanism both should extend rather than reinvent.
//
// TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation is what keeps
// this table honest against the vendored spec the same way audit_1to1's
// allow-list is kept honest against it: an entry naming an operationId the
// real spec does not have, or naming the wrong domain for one it does have,
// is a name nobody is getting generated for them, and it would silently
// outlive whatever future respec made it stale.
var actionNameOverrides = map[string]actionNameOverride{
	"MOTD": {
		Domain: "motd",
		Name:   "motd.get",
		Reason: "operationId equals domain \"motd\"'s own prefix exactly; ActionName refuses rather than emit \"motd.motd\"",
	},
	"Backup": {
		Domain: "backup",
		Name:   "backup.create",
		Reason: "operationId equals domain \"backup\"'s own prefix exactly (POST /backup, downloads a backup archive); ActionName refuses rather than emit \"backup.backup\"",
	},
	"TeamMemberships": {
		Domain: "team_memberships",
		Name:   "team_memberships.list_for_team",
		Reason: "operationId equals domain \"team_memberships\"'s own prefix exactly (GET /teams/{id}/memberships, the memberships of one team); ActionName refuses rather than emit \"team_memberships.team_memberships\". " +
			"The name is \"list_for_team\", not the shorter \"list\", because \"team_memberships.list\" is already the name ActionName mechanically mints for the *different* operationId TeamMembershipList (GET /team_memberships, every membership on the server). " +
			"Those two are a real collision — TestUnit_ResolveActionName_NoCollisionAcrossTheEntireSpecification is what now proves no override shadows a mechanical name, a check the older ActionName-only collision test structurally could not make",
	},
	"Generate": {
		Domain: "cloud",
		Name:   "cloud.generate_ssh_key",
		Reason: "operationId is the bare verb \"Generate\" with no object (POST /sshkeygen); ActionName succeeds and returns \"cloud.generate\", which is grammatically valid but names nothing a model could act on without reading the description first",
	},
	"StackCreateKubernetesFile": {
		Domain: "stacks",
		Name:   "stacks.create_kubernetes_string",
		Reason: "the operationId is wrong about its own route: POST /stacks/create/kubernetes/string takes a Kubernetes manifest as the JSON property stackFileContent and declares no multipart form at all, so ActionName's mechanical \"stacks.create_kubernetes_file\" names a file upload that does not exist. " +
			"This is the third kind of entry this table holds, after \"operationId equals its own domain prefix\" and \"operationId names nothing actionable\": a mechanical name that is not merely uninformative but false. " +
			"It is also a collision in meaning rather than in text — the stacks domain has two genuine multipart uploads, StackCreateDockerStandaloneFile and StackCreateDockerSwarmFile, whose hand-written actions are named create_docker_standalone_file and create_docker_swarm_file, so \"create_kubernetes_file\" would sit beside them claiming a shape it does not have. " +
			"Renamed to the \"_string\" form its two Docker siblings already use for the same inline-content shape. " +
			"Its neighbour StackCreateKubernetesGit is deliberately left alone: \"stacks.create_kubernetes_git\" is inconsistent with create_docker_standalone_repository for the same /repository route shape, but it is not false, and this table is for names that mislead",
	},
}

// ResolveActionName is what Task 2 and Task 3's domain packages call instead
// of ActionName directly: actionNameOverrides checked first, ActionName's
// mechanical rule as the fallback for every operationId the table does not
// mention. Centralising the check here means no domain author has to
// remember, while writing the 423rd action, that this one table needs
// consulting before ActionName does.
func ResolveActionName(domain, operationID string) (string, error) {
	return resolveActionName(actionNameOverrides, domain, operationID)
}

// resolveActionName is ResolveActionName with the override table as a
// parameter rather than read from the package variable. Production code always
// calls ResolveActionName; this exists so the collision check over the whole
// specification can be driven with a *synthetic* table as well as the real
// one. Without that, a test asserting "the real table collides with nothing"
// would pass just as happily against a check that never compares overrides at
// all — which is precisely how the defect this split was introduced for
// survived: the previous collision test called ActionName directly, so an
// override's produced name was never compared against anything.
func resolveActionName(overrides map[string]actionNameOverride, domain, operationID string) (string, error) {
	if override, ok := overrides[operationID]; ok {
		if override.Domain != domain {
			return "", fmt.Errorf(
				"gen_action_inputs: ResolveActionName: operationID %q is overridden for domain %q, called with domain %q",
				operationID, override.Domain, domain)
		}
		return override.Name, nil
	}
	return ActionName(domain, operationID)
}

// validateActionNameOverrides checks actionNameOverrides against ops (every
// operation the real vendored spec declares, domain-tagged): every
// override's operationId must actually exist, under the domain the override
// itself names. The reverse direction — an operation this rule refuses that
// has no override — is deliberately not checked here: Task 2 and 3 add
// overrides only for the operations they are actively naming, not for every
// refusal that exists anywhere in the spec today, so a refusal with no
// override yet is expected, not a defect this function should report.
func validateActionNameOverrides(overrides map[string]actionNameOverride, ops []namedOperation) error {
	exists := make(map[string]string, len(ops)) // operationID -> domain
	for _, op := range ops {
		exists[op.OperationID] = op.Domain
	}

	operationIDs := make([]string, 0, len(overrides))
	for id := range overrides {
		operationIDs = append(operationIDs, id)
	}
	sort.Strings(operationIDs)

	for _, operationID := range operationIDs {
		override := overrides[operationID]
		domain, ok := exists[operationID]
		if !ok {
			return fmt.Errorf("gen_action_inputs: actionNameOverrides: operationId %q has no matching operation in the vendored spec; remove this stale entry", operationID)
		}
		if domain != override.Domain {
			return fmt.Errorf("gen_action_inputs: actionNameOverrides: operationId %q belongs to domain %q in the vendored spec, not %q as this entry declares", operationID, domain, override.Domain)
		}
	}
	return nil
}
