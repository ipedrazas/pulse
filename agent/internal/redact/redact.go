package redact

import "strings"

const placeholder = "[REDACTED]"

// Envs returns a copy of envs with values replaced by [REDACTED]
// for any key that contains one of the given patterns (case-insensitive).
func Envs(envs map[string]string, patterns []string) map[string]string {
	if len(patterns) == 0 {
		return envs
	}

	result := make(map[string]string, len(envs))
	for k, v := range envs {
		if matchesAny(k, patterns) {
			result[k] = placeholder
		} else {
			result[k] = v
		}
	}
	return result
}

func matchesAny(key string, patterns []string) bool {
	upper := strings.ToUpper(key)
	for _, p := range patterns {
		if strings.Contains(upper, p) {
			return true
		}
	}
	return false
}
