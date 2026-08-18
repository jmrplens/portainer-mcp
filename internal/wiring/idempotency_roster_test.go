package wiring

import (
	"sort"
	"testing"
)

// destructiveAndIdempotent is the roster of catalog actions that claim BOTH
// flags. Adding a name here must be a deliberate edit, because the pair is
// only coherent for a narrow class of action and was already got wrong once.
//
// toolutil.ActionSpec.Idempotent means "can be repeated without additional
// effect", and internal/tools/register.go passes it to callers as
// IdempotentHint — a retry-safety promise. Destructive means the action
// removes or irreversibly alters state. Both together are honest for a
// delete: deleting an already-deleted resource leaves the same end state, so
// a retry costs nothing.
//
// They are a contradiction for an action whose end state is decided by
// something other than the request. custom_templates.git_fetch carried both
// until 2026-08-18: it replaces stored content with whatever the remote holds
// at the moment of the call, so two identical calls can produce different
// results, and the hint invited a caller to retry the one write in that
// domain with no undo. It had been generated Idempotent by the verb rule
// (PUT), then hand-marked Destructive four lines away, and nothing read the
// two together.
//
// The test the wave-1 stacks domain grew for the same defect pins flags
// per-action within one domain. This one is the catalog-wide counterpart:
// it does not judge whether any individual pairing is right, only that the
// set of actions claiming both cannot grow without someone saying so here.
var destructiveAndIdempotent = map[string]string{
	"custom_templates.delete":          "deleting an absent template leaves the same end state",
	"registries.delete":                "deleting an absent registry leaves the same end state",
	"registries.ecr_delete_repository": "deleting an absent ECR repository leaves the same end state",
	"tags.delete":                      "deleting an absent tag leaves the same end state",
}

func TestUnit_DestructiveAndIdempotentActions_MatchTheirRoster(t *testing.T) {
	var got []string
	for _, spec := range AllSpecs() {
		if spec.Destructive && spec.Idempotent {
			got = append(got, spec.Name)
		}
	}
	sort.Strings(got)

	want := make([]string, 0, len(destructiveAndIdempotent))
	for name := range destructiveAndIdempotent {
		want = append(want, name)
	}
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("actions claiming Destructive AND Idempotent = %v, roster = %v.\nAn action that removes or irreversibly alters state, yet promises callers it can be retried freely, is only honest when the request itself determines the end state. If this one qualifies, add it to destructiveAndIdempotent with the reason; if it does not, drop its Idempotent flag.", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("actions claiming Destructive AND Idempotent = %v, roster = %v", got, want)
		}
	}
}
