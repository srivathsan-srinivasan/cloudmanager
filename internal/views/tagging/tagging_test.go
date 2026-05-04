package tagging

import "testing"

func TestSplitInputNormalizesAndDedupes(t *testing.T) {
	got := SplitInput(" VFWEB, prod;vfweb\nops ")
	want := []string{"VFWEB", "prod", "ops"}
	if len(got) != len(want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected tag %d %q, got %q", i, want[i], got[i])
		}
	}
}
