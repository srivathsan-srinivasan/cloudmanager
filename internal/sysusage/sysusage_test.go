package sysusage

import (
	"regexp"
	"testing"
)

func TestFooterTextFormat(t *testing.T) {
	text := FooterText()
	if !regexp.MustCompile(`^CPU:\d+\.\d% Mem:\d+MB$`).MatchString(text) {
		t.Fatalf("unexpected footer text: %q", text)
	}
}
