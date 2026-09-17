package mevd1726

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/freebeamer/core/pkg/types"
)

func validatedBlocks(image []byte) ([]block, error) {
	if New().Detect(image).Confidence != types.ChecksumDetectionHigh {
		return nil, ErrLayoutMismatch
	}
	return parseBlocks(image)
}

func parseBlocks(image []byte) ([]block, error) {
	var blocks []block
	search := 0
	for {
		relative := bytes.Index(image[search:], magic)
		if relative < 0 {
			break
		}
		position := search + relative
		search = position + 1
		if position < 64 {
			continue
		}
		header := position - 64
		if header+72 > len(image) {
			continue
		}
		candidate, valid, err := parseBlock(image, header)
		if err != nil {
			return nil, err
		}
		if valid {
			blocks = append(blocks, candidate)
		}
	}
	sort.Slice(blocks, func(left, right int) bool {
		return blocks[left].fileStart < blocks[right].fileStart
	})
	return blocks, nil
}

func parseBlock(image []byte, header int) (block, bool, error) {
	identifier := binary.LittleEndian.Uint32(image[header:])
	size := uint64(binary.LittleEndian.Uint32(image[header+4:]))
	memoryEnd := binary.LittleEndian.Uint32(image[header+12:])
	count := int(binary.LittleEndian.Uint32(image[header+0x2c:]))
	if !validBlockHeader(image, header, identifier, size, count) {
		return block{}, false, nil
	}
	memoryStart := memoryEnd + 4 - uint32(size)
	if !flashAddress(memoryStart) || !flashAddress(memoryEnd) {
		return block{}, false, nil
	}
	trailer := binary.LittleEndian.Uint32(image[uint64(header)+size-4:])
	if trailer != 0 && trailer != 0xdeadbeef {
		return block{}, false, nil
	}

	candidate := block{
		fileStart:   uint64(header),
		size:        size,
		identifier:  identifier,
		memoryStart: memoryStart,
		memoryEnd:   memoryEnd + 3,
	}
	for index := 0; index < count; index++ {
		offset := uint64(header + 0x34 + index*32)
		if offset+32 > uint64(len(image)) {
			return block{}, false, ErrLayoutMismatch
		}
		candidate.structures = append(candidate.structures, parseStructure(image, offset))
	}
	return candidate, true, nil
}

func validBlockHeader(image []byte, header int, identifier uint32, size uint64, count int) bool {
	validAddressPrefix := image[header+15] == 0x80 || image[header+15] == 0xa0
	return knownBlock(byte(identifier)) &&
		image[header+1] == 0 &&
		image[header+3] == 0 &&
		validAddressPrefix &&
		size >= 0x40 &&
		uint64(header)+size <= uint64(len(image)) &&
		count >= 1 && count <= 8
}

func parseStructure(image []byte, offset uint64) structure {
	return structure{
		offset:    offset,
		algorithm: byte(binary.LittleEndian.Uint16(image[offset+28:])),
		start:     binary.LittleEndian.Uint32(image[offset+4:]),
		end:       binary.LittleEndian.Uint32(image[offset+8:]),
		seed:      binary.LittleEndian.Uint32(image[offset+12:]),
		expected:  binary.LittleEndian.Uint32(image[offset+16:]),
	}
}

func matchesManifest(blocks []block) bool {
	if len(blocks) != len(validatedManifest) {
		return false
	}
	for index, expected := range validatedManifest {
		actual := blocks[index]
		if actual.fileStart != expected.start ||
			actual.size != expected.size ||
			actual.identifier != expected.identifier ||
			len(actual.structures) != expected.structures {
			return false
		}
	}
	return true
}

func knownBlock(identifier byte) bool {
	switch identifier {
	case 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
		0x90, 0xa0, 0xb0, 0xc0, 0xd0, 0xe0, 0xf0, 0xf1:
		return true
	default:
		return false
	}
}

func flashAddress(address uint32) bool {
	return address>>28 == 0x8 || address>>28 == 0xa
}
