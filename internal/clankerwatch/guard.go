package clankerwatch

import (
	"regexp"
	"strings"
)

var leadingBlockComment = regexp.MustCompile(`(?s)^\s*/\*.*?\*/`)
var leadingLineComment = regexp.MustCompile(`(?m)^\s*--.*(\r?\n|$)`)

func IsReadOnlySQL(sql string) bool {
	cleaned := stripLeadingSQLComments(sql)
	if cleaned == "" {
		return false
	}
	first := firstSQLWord(cleaned)
	switch first {
	case "select", "with", "explain", "show", "describe", "desc":
		return true
	default:
		return false
	}
}

func stripLeadingSQLComments(sql string) string {
	s := strings.TrimSpace(sql)
	for {
		before := s
		s = strings.TrimSpace(leadingBlockComment.ReplaceAllString(s, ""))
		s = strings.TrimSpace(leadingLineComment.ReplaceAllString(s, ""))
		if s == before {
			return s
		}
	}
}

func firstSQLWord(sql string) string {
	sql = strings.TrimSpace(sql)
	for i, r := range sql {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') {
			return strings.ToLower(sql[:i])
		}
	}
	return strings.ToLower(sql)
}
