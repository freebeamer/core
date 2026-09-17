package utils

import (
	"encoding/xml"
	"strings"
)

func Attrs(values []xml.Attr) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		result[strings.ToLower(value.Name.Local)] = value.Value
	}
	return result
}
