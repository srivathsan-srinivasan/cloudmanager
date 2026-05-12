package publicip

import "testing"

func TestCIDRFromIP(t *testing.T) {
	tests := map[string]string{
		"203.0.113.10": "203.0.113.10/32",
		"2001:db8::1":  "2001:db8::1/128",
	}
	for input, want := range tests {
		got, err := CIDRFromIP(input)
		if err != nil {
			t.Fatalf("CIDRFromIP(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("CIDRFromIP(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCIDRFromIPRejectsInvalidIP(t *testing.T) {
	if _, err := CIDRFromIP("not-an-ip"); err == nil {
		t.Fatal("expected invalid IP to fail")
	}
}
