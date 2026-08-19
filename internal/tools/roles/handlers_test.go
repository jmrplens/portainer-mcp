package roles

import "testing"

// TestUnit_EveryDeclaredAction_HasANarrative is the only guard that catches a
// lost narrative in this domain, and it exists because nothing else does.
//
// narrative()'s default arm returns a zero ActionNarrative, which
// toolutil.WithNarrative treats as "no override" — so deleting this domain's
// only case leaves the action publishing the vendored specification's own
// wording ("List roles" / "List all roles available for use"), which says
// nothing about the edition asymmetry that is the entire point of the
// override. cmd/audit_spec_drift cannot catch that: with the override gone,
// catalog and document agree again and the audit reports "No drift". That was
// measured during wave 2 stage A, on a different domain, which is why every
// domain since carries this test.
func TestUnit_EveryDeclaredAction_HasANarrative(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if !s.TitleOverridden || !s.DescriptionOverridden {
			t.Errorf("%s (%s) has no narrative() entry", s.Name, s.OperationID)
		}
	}
}
