package checksum

import "github.com/freebeamer/core/pkg/types"

func AllValid(results []types.ChecksumResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if !result.Valid {
			return false
		}
	}
	return true
}
