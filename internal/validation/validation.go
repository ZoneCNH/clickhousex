package validation

import "fmt"

// RequireNonEmpty returns an error if value is empty.
func RequireNonEmpty(field string, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}
