// Package mevd1726 implements the checksum layout for BMW firmware 75P9EJ0B.
// It is intentionally firmware-specific and limited to calibration correction.
package mevd1726

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/freebeamer/core/pkg/checksum"
	"github.com/freebeamer/core/pkg/types"
)

const ProviderID = "bosch.mevd17.2.6.75P9EJ0B"

var (
	ErrLayoutMismatch   = errors.New("firmware does not match validated MEVD17.2.6 75P9EJ0B layout")
	ErrUnknownAlgorithm = errors.New("unknown Bosch checksum algorithm")
	ErrUnsafeCorrection = errors.New("correction outside validated 75P9EJ0B calibration region is refused")
	magic               = []byte{0xfe, 0xca, 0xde, 0xfa, 0xfe, 0xaf, 0xfe, 0xca}
)

const (
	algorithmCRC32 byte   = 0x00
	algorithmADD32 byte   = 0x01
	algorithmADD16 byte   = 0x10
	flashMask      uint32 = 0x0fffffff
)

type Provider struct{}

var _ checksum.Provider = (*Provider)(nil)

func New() *Provider         { return &Provider{} }
func (*Provider) ID() string { return ProviderID }

func (p *Provider) Detect(image []byte) types.ChecksumDetection {
	if len(image) != 4*1024*1024 || !bytes.Contains(image, []byte("75P9EJ0B")) {
		return types.ChecksumDetection{Confidence: types.ChecksumDetectionNone, Reason: "size/software marker mismatch"}
	}
	blocks, err := parseBlocks(image)
	if err != nil || !matchesManifest(blocks) {
		return types.ChecksumDetection{Confidence: types.ChecksumDetectionNone, Reason: "checksum block layout mismatch"}
	}
	return types.ChecksumDetection{Confidence: types.ChecksumDetectionHigh, Reason: "exact 75P9EJ0B marker, size, and six-block manifest"}
}

func (p *Provider) Verify(image []byte) ([]types.ChecksumResult, error) {
	blocks, err := validatedBlocks(image)
	if err != nil {
		return nil, err
	}
	return verify(image, blocks)
}

func (p *Provider) Correct(image []byte) ([]types.ChecksumResult, error) {
	blocks, err := validatedBlocks(image)
	if err != nil {
		return nil, err
	}
	before, err := verify(image, blocks)
	if err != nil {
		return nil, err
	}
	working := append([]byte(nil), image...)
	corrected := map[string]bool{}
	index := 0
	for _, block := range blocks {
		for _, structure := range block.structures {
			result := before[index]
			index++
			if result.Valid {
				continue
			}
			if block.identifier&0xff != 0x60 || structure.algorithm != algorithmADD16 {
				return nil, fmt.Errorf("%w: %s", ErrUnsafeCorrection, result.Name)
			}
			start, end, translateErr := regionOffsets(block, structure, len(working))
			if translateErr != nil {
				return nil, translateErr
			}
			calculated, calcErr := calculate(working[start:end+1], structure.algorithm, structure.seed)
			if calcErr != nil {
				return nil, calcErr
			}
			difference := structure.expected - calculated
			slot := end - 3
			old := binary.LittleEndian.Uint32(working[slot:])
			binary.LittleEndian.PutUint32(working[slot:], old+difference)
			corrected[result.Name] = true
		}
	}
	after, err := verify(working, blocks)
	if err != nil {
		return nil, err
	}
	if !checksum.AllValid(after) {
		return nil, errors.New("MEVD17.2.6 correction did not verify")
	}
	for index := range after {
		after[index].Corrected = corrected[after[index].Name]
	}
	copy(image, working)
	return after, nil
}
