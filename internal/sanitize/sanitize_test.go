package sanitize

import "testing"

func TestSecret(t *testing.T) {
	t.Parallel()

	if got := Secret(""); got != "" {
		t.Fatalf("Secret(empty) = %q, want empty", got)
	}
	if got := Secret("secret"); got != "***" {
		t.Fatalf("Secret(non-empty) = %q, want masked", got)
	}
}
