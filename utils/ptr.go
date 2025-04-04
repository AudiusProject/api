package utils

func String(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
