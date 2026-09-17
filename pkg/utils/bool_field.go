package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func BoolField(values map[string]string, key string) (bool, error) {
	text := strings.TrimSpace(values[key])
	if text == "" || text == "0" {
		return false, nil
	}
	if text == "1" {
		return true, nil
	}
	value, err := strconv.ParseBool(text)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: %w", key, text, err)
	}
	return value, nil
}
