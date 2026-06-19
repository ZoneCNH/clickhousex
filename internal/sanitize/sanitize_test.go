package sanitize

import "testing"

func TestSecret(t *testing.T) {
	if got := Secret(""); got != "" {
		t.Fatalf("empty secret should remain empty, got %q", got)
	}
	if got := Secret("password"); got != "***" {
		t.Fatalf("non-empty secret should be masked, got %q", got)
	}
}
