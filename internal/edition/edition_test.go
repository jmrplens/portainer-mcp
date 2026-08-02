package edition

import "testing"

func TestParse_KnownValues_AreAccepted(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]Edition{"CE": CE, "ce": CE, "EE": EE, "ee": EE, "": ""} {
		got, err := Parse(input)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParse_UnknownValue_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := Parse("business"); err == nil {
		t.Error("Parse(\"business\") error = nil, want an error")
	}
}

// EE is a superset of CE: an EE server serves every CE operation. The reverse
// is false, and getting this backwards would expose 179 EE-only operations on
// a CE server.
func TestIncludes_Supersetting_IsAsymmetric(t *testing.T) {
	t.Parallel()
	cases := []struct {
		have, required Edition
		want           bool
	}{
		{EE, CE, true},
		{EE, EE, true},
		{CE, CE, true},
		{CE, EE, false},
	}
	for _, tc := range cases {
		if got := tc.have.Includes(tc.required); got != tc.want {
			t.Errorf("%q.Includes(%q) = %v, want %v", tc.have, tc.required, got, tc.want)
		}
	}
}
