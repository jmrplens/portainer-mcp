package resource_controls

import "testing"

// TestUnit_EveryDeclaredAction_HasANarrative is the only guard that catches a
// lost narrative in this domain, and it exists because nothing else does.
//
// narrative()'s default arm returns a zero ActionNarrative, which
// toolutil.WithNarrative treats as "no override" — so deleting one of the
// three cases leaves that action publishing the vendored specification's own
// wording, which for all three is a single sentence that never mentions the
// missing read route, the replace semantics of update, or the three refusals
// this catalog raises before Portainer is called. cmd/audit_spec_drift cannot
// catch that: with the override gone, catalog and document agree again and
// the audit reports "No drift". That was measured during wave 2 stage A, on a
// different domain, which is why every domain since carries this test.
func TestUnit_EveryDeclaredAction_HasANarrative(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if !s.TitleOverridden || !s.DescriptionOverridden {
			t.Errorf("%s (%s) has no narrative() entry", s.Name, s.OperationID)
		}
	}
}
