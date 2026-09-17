package mevd1726

import (
	"fmt"

	"github.com/freebeamer/core/pkg/checksum/common"
	"github.com/freebeamer/core/pkg/types"
)

func regionOffsets(block block, structure structure, imageLength int) (uint64, uint64, error) {
	blockBase := canonicalAddress(block.memoryStart)
	startAddress := canonicalAddress(structure.start)
	endAddress := canonicalAddress(structure.end)
	if startAddress < blockBase || endAddress < startAddress {
		return 0, 0, ErrLayoutMismatch
	}
	start := block.fileStart + uint64(startAddress-blockBase)
	end := block.fileStart + uint64(endAddress-blockBase)
	if end >= uint64(imageLength) || start >= end {
		return 0, 0, ErrLayoutMismatch
	}
	return start, end, nil
}

func canonicalAddress(address uint32) uint32 {
	return address&flashMask | 0x80000000
}

func verify(image []byte, blocks []block) ([]types.ChecksumResult, error) {
	var results []types.ChecksumResult
	for _, block := range blocks {
		for index, structure := range block.structures {
			result, err := verifyStructure(image, block, structure, index)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}
	return results, nil
}

func verifyStructure(image []byte, block block, structure structure, index int) (types.ChecksumResult, error) {
	start, end, err := regionOffsets(block, structure, len(image))
	if err != nil {
		return types.ChecksumResult{}, err
	}
	calculated, err := calculate(image[start:end+1], structure.algorithm, structure.seed)
	if err != nil {
		return types.ChecksumResult{}, err
	}
	expected := structure.expected
	if structure.algorithm == algorithmCRC32 {
		expected = ^expected
	}
	return types.ChecksumResult{
		Name:          fmt.Sprintf("block-0x%02X-%d-%s", byte(block.identifier), index+1, algorithmName(structure.algorithm)),
		RegionStart:   start,
		RegionEnd:     end,
		StorageOffset: structure.offset + 16,
		Stored:        uint64(expected),
		Calculated:    uint64(calculated),
		Valid:         calculated == expected,
	}, nil
}

func calculate(data []byte, algorithm byte, seed uint32) (uint32, error) {
	switch algorithm {
	case algorithmCRC32:
		return common.CRC32IEEEWordsLE(data, seed)
	case algorithmADD32:
		return common.Add32LE(data, seed)
	case algorithmADD16:
		return common.Add16LE(data, seed)
	default:
		return 0, fmt.Errorf("%w: %#x", ErrUnknownAlgorithm, algorithm)
	}
}

func algorithmName(algorithm byte) string {
	switch algorithm {
	case algorithmCRC32:
		return "CRC32"
	case algorithmADD32:
		return "ADD32"
	case algorithmADD16:
		return "ADD16"
	default:
		return fmt.Sprintf("UNKNOWN-%02X", algorithm)
	}
}
