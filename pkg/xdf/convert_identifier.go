package xdf

import (
	"fmt"
	"strings"

	"github.com/freebeamer/core/pkg/types"
)

func convertCategories(source *types.XDFDefinition, document *types.MapDefinition) map[int]string {
	identifiers := make(map[int]string, len(source.Categories))
	for _, category := range source.Categories {
		id := identifier(category.Name, fmt.Sprintf("category-%x", category.Index))
		identifiers[category.Index] = id
		document.Categories = append(document.Categories, types.MapCategory{
			ID:   id,
			Name: valueOrFallback(category.Name, id),
		})
	}
	return identifiers
}

func resolveCategories(values []int, lookup map[int]string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if id := lookup[value]; id != "" {
			result = append(result, id)
			continue
		}
		result = append(result, fmt.Sprintf("category-%x", value))
	}
	return result
}

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
