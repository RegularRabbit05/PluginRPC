package utils

func Or(a string, b string) string {
	if a != "" {
		return a
	}
	return b
}
