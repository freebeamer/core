package common

import (
	"errors"
	"testing"
)

func TestChecksumPrimitives(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	if got, err := Add32LE(data, 0xfadecafe); err != nil || got != 0x06e8d304 {
		t.Fatalf("ADD32=%08x,%v", got, err)
	}
	if got, err := Add16LE(data, 0xfadecafe); err != nil || got != 0x02e5d707 {
		t.Fatalf("ADD16=%08x,%v", got, err)
	}
	if got, err := CRC32IEEEWordsLE(data, 0xfadecafe); err != nil || got != 0xf2d4a4b2 {
		t.Fatalf("CRC32=%08x,%v", got, err)
	}
}
func TestChecksumPrimitivesRejectUnaligned(t *testing.T) {
	for _, fn := range []func([]byte, uint32) (uint32, error){CRC32IEEEWordsLE, Add32LE, Add16LE} {
		if _, err := fn([]byte{1, 2}, 0); !errors.Is(err, ErrUnalignedRegion) {
			t.Fatalf("error=%v", err)
		}
	}
}
