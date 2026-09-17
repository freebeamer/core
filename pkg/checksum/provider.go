// Package checksum provides provider selection and safe checksum workflows.
package checksum

import "github.com/freebeamer/core/pkg/types"

type Provider interface {
	ID() string
	Detect(image []byte) types.ChecksumDetection
	Verify(image []byte) ([]types.ChecksumResult, error)
	Correct(image []byte) ([]types.ChecksumResult, error)
}
