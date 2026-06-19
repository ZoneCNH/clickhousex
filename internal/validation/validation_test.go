package validation

import "testing"

func TestRequireNonEmpty(t *testing.T) {
	t.Parallel()

	if err := RequireNonEmpty("name", "value"); err != nil {
		t.Fatalf("RequireNonEmpty(non-empty) returned error: %v", err)
	}
	if err := RequireNonEmpty("name", ""); err == nil || err.Error() != "name is required" {
		t.Fatalf("RequireNonEmpty(empty) = %v, want name is required", err)
	}
}
