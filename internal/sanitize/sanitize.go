package sanitize

// Secret masks a sensitive value. Returns "***" if non-empty, empty string otherwise.
func Secret(value string) string {
	if value == "" {
		return ""
	}
	return "***"
}
