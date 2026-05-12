package client

import (
	"strings"
)

// ToTableName converts Model name to plural snake_case (e.g. User -> users).
func ToTableName(s string) string {
	return pluralize(ToSnakeCase(s))
}

// ToSnakeCase converts Go/Schema names to snake_case for DB.
func ToSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// pluralize adds basic "s" or "es" to a name.
func pluralize(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "y") && !strings.HasSuffix(s, "ay") && !strings.HasSuffix(s, "ey") && !strings.HasSuffix(s, "iy") && !strings.HasSuffix(s, "oy") && !strings.HasSuffix(s, "uy") {
		return s[:len(s)-1] + "ies"
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") || strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}
