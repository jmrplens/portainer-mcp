package version

import "testing"

func TestString_DefaultValues_ReportsDev(t *testing.T) {
	t.Parallel()
	if got := String(); got != "dev (unknown, unknown)" {
		t.Errorf("String() = %q, want %q", got, "dev (unknown, unknown)")
	}
}

func TestString_InjectedValues_ReportsThem(t *testing.T) {
	origV, origC, origD := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = origV, origC, origD })

	Version, Commit, BuildDate = "1.0.0", "abc1234", "2026-08-02"
	if got := String(); got != "1.0.0 (abc1234, 2026-08-02)" {
		t.Errorf("String() = %q, want %q", got, "1.0.0 (abc1234, 2026-08-02)")
	}
}
