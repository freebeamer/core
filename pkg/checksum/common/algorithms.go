// Package common contains checksum primitives shared by Bosch providers.
package common

import (
	"encoding/binary"
	"errors"
)

var ErrUnalignedRegion = errors.New("checksum region length must be a positive multiple of four bytes")

func CRC32IEEEWordsLE(data []byte, seed uint32) (uint32, error) {
	if len(data) == 0 || len(data)%4 != 0 {
		return 0, ErrUnalignedRegion
	}
	crc := seed
	for offset := 0; offset < len(data); offset += 4 {
		word := binary.LittleEndian.Uint32(data[offset:])
		for bit := 0; bit < 32; bit++ {
			xor := word ^ crc
			word >>= 1
			if xor&1 != 0 {
				crc = (crc >> 1) ^ 0xedb88320
			} else {
				crc >>= 1
			}
		}
	}
	return crc, nil
}

func Add32LE(data []byte, seed uint32) (uint32, error) {
	if len(data) == 0 || len(data)%4 != 0 {
		return 0, ErrUnalignedRegion
	}
	sum := seed
	for offset := 0; offset < len(data); offset += 4 {
		sum += binary.LittleEndian.Uint32(data[offset:])
	}
	return sum, nil
}

// Add16LE follows Bosch SB_ADD16_ALGO_E: all words except the final one are
// added in the low half; the final word contributes in the high half. Thus the
// last dword behaves as a single 32-bit compensation value.
func Add16LE(data []byte, seed uint32) (uint32, error) {
	if len(data) == 0 || len(data)%4 != 0 {
		return 0, ErrUnalignedRegion
	}
	sum := seed
	for offset := 0; offset < len(data)-2; offset += 2 {
		sum += uint32(binary.LittleEndian.Uint16(data[offset:]))
	}
	sum += uint32(binary.LittleEndian.Uint16(data[len(data)-2:])) << 16
	return sum, nil
}
