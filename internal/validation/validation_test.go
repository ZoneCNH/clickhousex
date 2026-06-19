package validation

import "testing"

func TestRequireNonEmpty(t *testing.T) {
	if err := RequireNonEmpty("name", "primary"); err != nil {
		t.Fatalf("expected non-empty value to pass, got %v", err)
	}

	err := RequireNonEmpty("name", "")
	if err == nil {
		t.Fatal("expected empty value to fail")
	}
	if got := err.Error(); got != "name is required" {
		t.Fatalf("unexpected error: %q", got)
	}
}
