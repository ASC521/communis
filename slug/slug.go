package slug

import (
	"regexp"
	"strings"
)

func Slugify(s string) string {
	s = strings.ToLower(s)

	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	reg := regexp.MustCompile("[^a-z0-9-]+")
	s = reg.ReplaceAllString(s, "")

	reg = regexp.MustCompile("-+")
	s = reg.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")

	maxLen := 100
	if len(s) > maxLen {
		s = s[:maxLen]
		s = strings.TrimRight(s, "-")
	}

	if s == "" {
		return "untitled"
	}

	return s
}
