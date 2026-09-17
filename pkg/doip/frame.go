package doip

import (
	"encoding/binary"
	"fmt"
)

// ErrInvalidHeader is returned when a received DoIP generic header fails
// its own self-check (the protocol version's bitwise complement doesn't
// match) or is otherwise malformed.
var ErrInvalidHeader = fmt.Errorf("doip: invalid generic header")

// ErrMessageTooLarge is returned when a received DoIP message's declared
// payload length exceeds maxPayloadLength.
var ErrMessageTooLarge = fmt.Errorf("doip: message too large")

// packMessage renders one complete DoIP message: the 8-byte generic
// header followed by payload. See ISO 13400 Table 15.
func packMessage(protocolVersion byte, payloadType PayloadType, payload []byte) []byte {
	message := make([]byte, headerSize+len(payload))
	message[0] = protocolVersion
	message[1] = 0xFF ^ protocolVersion
	binary.BigEndian.PutUint16(message[2:4], uint16(payloadType))
	binary.BigEndian.PutUint32(message[4:8], uint32(len(payload)))
	copy(message[headerSize:], payload)
	return message
}

// parseHeader decodes a DoIP generic header from its fixed 8 bytes,
// validating the protocol version's self-check byte.
func parseHeader(raw [headerSize]byte) (header, error) {
	protocolVersion := raw[0]
	inverse := raw[1]
	if inverse != 0xFF^protocolVersion {
		return header{}, fmt.Errorf("%w: protocol version 0x%02X inverse byte mismatch (got 0x%02X)", ErrInvalidHeader, protocolVersion, inverse)
	}
	return header{
		ProtocolVersion: protocolVersion,
		PayloadType:     PayloadType(binary.BigEndian.Uint16(raw[2:4])),
		PayloadLength:   binary.BigEndian.Uint32(raw[4:8]),
	}, nil
}
