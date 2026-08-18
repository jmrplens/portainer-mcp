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
	claimed := map[string]bool{}
	for _, spec := range AllSpecs() {
		if spec.Destructive && spec.Idempotent {
			claimed[spec.Name] = true
		}
	}

	// One subtest per rostered action, so a roster entry that goes stale
	// names itself rather than hiding inside a set difference. This is the
	// direction that catches a flag being dropped by a regeneration.
	for name, why := range destructiveAndIdempotent {
		t.Run(name, func(t *testing.T) {
			if !claimed[name] {
				t.Errorf("%s is on the Destructive+Idempotent roster (%q) but no longer claims both flags; if that is deliberate, remove it from the roster", name, why)
			}
		})
	}

	// And the other direction, which is the one that caught
	// custom_templates.git_fetch: an action claiming both flags that nobody
	// has written a reason for.
	t.Run("no unrostered action claims both", func(t *testing.T) {
		var extra []string
		for name := range claimed {
			if _, ok := destructiveAndIdempotent[name]; !ok {
				extra = append(extra, name)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			t.Errorf("%v claim Destructive AND Idempotent but are not on the roster.\nAn action that removes or irreversibly alters state, yet promises callers it can be retried freely, is only honest when the request itself determines the end state. If one of these qualifies, add it to destructiveAndIdempotent with the reason; if it does not, drop its Idempotent flag.", extra)
		}
	})
}
