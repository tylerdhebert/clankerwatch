package clankerwatch

import (
	"fmt"
	"regexp"
	"strings"
)

const maxSessionSlugLen = 100

var sessionSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func NormalizeSessionSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" {
		return "", fmt.Errorf("session slug is required")
	}
	if len(slug) > maxSessionSlugLen {
		return "", fmt.Errorf("session slug must be at most %d characters", maxSessionSlugLen)
	}
	if strings.ContainsAny(slug, " \t\r\n") {
		return "", fmt.Errorf("session slug must not contain whitespace")
	}
	if !sessionSlugPattern.MatchString(slug) {
		return "", fmt.Errorf("session slug must use lowercase letters, numbers, and hyphens")
	}
	return slug, nil
}
