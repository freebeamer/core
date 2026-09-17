package a2l

import (
	"fmt"
	"strings"
)

// identifier, uniqueIdentifier, and valueOrFallback intentionally duplicate
// pkg/xdf's small identical helpers rather than sharing them: ADR 0001
// treats each external-format converter as independent of the others, owned
// entirely alongside its own format.

func identifier(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	previousWasDash := false
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			result.WriteRune(character)
			previousWasDash = false
			continue
		}
		if result.Len() > 0 && !previousWasDash {
			result.WriteByte('-')
			previousWasDash = true
		}
	}
	id := strings.Trim(result.String(), "-")
	if id == "" {
		return fallback
	}
	return id
}

func uniqueIdentifier(id string, used map[string]int) string {
	used[id]++
	if used[id] == 1 {
		return id
	}
	return fmt.Sprintf("%s-%d", id, used[id])
}

func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
