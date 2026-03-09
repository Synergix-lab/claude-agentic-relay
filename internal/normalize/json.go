package normalize

import (
	"encoding/json"
	"strings"
	"unicode"
)

// JSONKeys normalizes all keys in a JSON string from camelCase to snake_case.
// If the input is not valid JSON, it is returned unchanged.
func JSONKeys(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Only process JSON objects and arrays
	if s[0] != '{' && s[0] != '[' {
		return s
	}

	var raw any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return s
	}

	converted := convertKeys(raw)
	out, err := json.Marshal(converted)
	if err != nil {
		return s
	}
	return string(out)
}

func convertKeys(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[toSnakeCase(k)] = convertKeys(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = convertKeys(v)
		}
		return out
	default:
		return v
	}
}

// toSnakeCase converts camelCase/PascalCase to snake_case.
// Already snake_case strings pass through unchanged.
func toSnakeCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)

	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				// Insert underscore before uppercase unless already preceded by underscore or uppercase
				if prev != '_' && !unicode.IsUpper(prev) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
