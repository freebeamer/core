// Package identify contains conservative, firmware-content-based ECU identification.
package identify

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/freebeamer/core/pkg/types"
)

const targetSize = 4 * 1024 * 1024

var softwarePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b[0-9]{2}P[0-9][A-Z0-9]{4}\b`),
	regexp.MustCompile(`\b[A-Z0-9]*FEF81A001[A-Z0-9]*\b`),
	regexp.MustCompile(`\b1037[0-9]{6}\b`),
}

func Identify(image []byte) types.ECUIdentity {
	stringsFound := printableStrings(image)
	joined := strings.ToUpper(strings.Join(stringsFound, "\n"))
	family := strings.Contains(joined, "MEVD17.2.6") || strings.Contains(joined, "MEVD1726")
	bmw := strings.Contains(joined, "BMW")
	n55 := strings.Contains(joined, "N55")
	software := extractSoftware(joined)
	identity := types.ECUIdentity{Software: software}
	if family {
		identity.ECUVendor = "Bosch"
		identity.ECUFamily = "MEVD17.2.6"
	}
	if bmw || n55 {
		identity.Manufacturer = "BMW"
	}
	if n55 {
		identity.Engine = "BMW N55"
	}
	sizeMatch := len(image) == targetSize
	switch {
	case family && n55 && sizeMatch && (bmw || len(software) > 0):
		identity.Confidence = types.IdentificationHigh
	case family && (sizeMatch || (bmw && n55) || len(software) > 0):
		identity.Confidence = types.IdentificationMedium
	case family || (bmw && n55):
		identity.Confidence = types.IdentificationLow
	}
	return identity
}

func printableStrings(image []byte) []string {
	result := asciiRuns(image)
	for _, byteOrder := range []int{0, 1} {
		decoded := make([]byte, 0, len(image)/2)
		for index := byteOrder; index+1 < len(image); index += 2 {
			other := index + 1
			if byteOrder == 1 {
				other = index - 1
			}
			if image[other] != 0 {
				decoded = append(decoded, 0)
				continue
			}
			decoded = append(decoded, image[index])
		}
		result = append(result, asciiRuns(decoded)...)
	}
	return result
}

func asciiRuns(data []byte) []string {
	var result []string
	start := -1
	flush := func(end int) {
		if start >= 0 && end-start >= 4 {
			result = append(result, string(data[start:end]))
		}
		start = -1
	}
	for index, value := range data {
		if value >= 0x20 && value <= 0x7e {
			if start < 0 {
				start = index
			}
		} else {
			flush(index)
		}
	}
	flush(len(data))
	return result
}

func extractSoftware(text string) []string {
	unique := map[string]struct{}{}
	for _, pattern := range softwarePatterns {
		for _, match := range pattern.FindAllString(text, -1) {
			unique[match] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// ContainsMarker is exposed for tests and future identification providers.
func ContainsMarker(image []byte, marker string) bool {
	return bytes.Contains(bytes.ToUpper(image), bytes.ToUpper([]byte(marker)))
}
