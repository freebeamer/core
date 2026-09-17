package binfile

import "github.com/freebeamer/core/pkg/types"

func DiffBytes(original, current []byte) ([]types.BinaryChange, error) {
	if len(original) != len(current) {
		return nil, ErrSizeMismatch
	}
	var changes []types.BinaryChange
	for offset, old := range original {
		if now := current[offset]; old != now {
			changes = append(changes, types.BinaryChange{Offset: uint64(offset), Original: old, Current: now})
		}
	}
	return changes, nil
}
