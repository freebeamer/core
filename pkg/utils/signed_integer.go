package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func SignedInteger(values map[string]string, key string) (int64, error) {
	text := strings.TrimSpace(values[key])
	if text == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, text, err)
	}
	return value, nil
}
