package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func Integer(values map[string]string, key string, bits int) (uint64, error) {
	text := strings.TrimSpace(values[key])
	if text == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(text, 0, bits)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, text, err)
	}
	return value, nil
}
