package specdiff

import "unicode"

// commonInitialisms, splitWords, consumeInitialisms and bodyJSONTag are
// reimplemented verbatim from cmd/gen_action_inputs/naming.go (that package
// is `package main`, so nothing outside it can import these at all,
// regardless of preference — see LoadSpecOperation's doc comment for the
// same reasoning applied to spec loading).
//
// This is not a stylistic choice: a Portainer body property is declared in
// the vendored specification in PascalCase-ish style ("BaseURL",
// "TLSSkipVerify"), but the generated Go struct field — and therefore the
// JSON property name every tool surface actually publishes, which is what
// ShapeFromCatalog reads via ActionSpec.InputSchema — carries the
// lower-camel-case rendering bodyJSONTag produces ("baseUrl",
// "tlsSkipVerify"). If ShapeFromSpec used the raw spec property name as
// FieldShape.JSONName instead, every single body field would compare as one
// field removed (the raw name) and one field added (the JSON name) against
// the catalog's shape — not a cosmetic mismatch but a complete inability to
// compare a body field at all, for every operation that has one. Rendering
// the identical transform here is what keeps ShapeFromSpec's JSONName equal
// to ShapeFromCatalog's for a field that has not actually changed.
var commonInitialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true, "CSS": true, "DNS": true,
	"EOF": true, "GUID": true, "HTML": true, "HTTP": true, "HTTPS": true, "ID": true,
	"IP": true, "JSON": true, "LHS": true, "QPS": true, "RAM": true, "RHS": true,
	"RPC": true, "SLA": true, "SMTP": true, "SQL": true, "SSH": true, "TCP": true,
	"TLS": true, "TTL": true, "UDP": true, "UI": true, "UID": true, "UUID": true,
	"URI": true, "URL": true, "UTF8": true, "VM": true, "XML": true, "XMPP": true,
	"XSRF": true, "XSS": true,
}

// splitWords breaks a wire identifier into its component words. See
// cmd/gen_action_inputs/naming.go's identical function for the full
// rationale; reproduced here unchanged.
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

// consumeInitialisms splits an uppercase-only rune run into words by greedily
// matching the longest recognised initialism at each position. See
// cmd/gen_action_inputs/naming.go's identical function.
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

// bodyJSONTag renders wire identifier words as the lower-camel-case JSON tag
// this project publishes to a model. See cmd/gen_action_inputs/naming.go's
// identical function for the full rationale; reproduced here unchanged.
func bodyJSONTag(words []string) string {
	if len(words) == 0 {
		return ""
	}
	out := []rune(lowerFirst(words[0]))
	for _, w := range words[1:] {
		out = append(out, []rune(titleCase(w))...)
	}
	return string(out)
}

func lowerFirst(s string) string {
	r := []rune(s)
	for i := range r {
		r[i] = unicode.ToLower(r[i])
	}
	return string(r)
}

// titleCase renders s with its first rune upper-cased and every other rune
// lower-cased, regardless of how s was originally cased.
func titleCase(s string) string {
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
