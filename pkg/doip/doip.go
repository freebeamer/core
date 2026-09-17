// Package doip implements a minimal ISO 13400 (DoIP — Diagnostics over
// Internet Protocol) client: the TCP transport BMW's ENET diagnostic
// interface (and MHD's WiFi-ENET adapter, which bridges to it) uses to
// carry UDS (ISO 14229) diagnostic requests/responses. Only what's needed
// to perform routing activation and exchange diagnostic messages over one
// TCP connection is implemented — no UDP vehicle discovery, no TLS, and
// nothing on DoIP's ECU reprogramming/emulation side. See
// docs/mhd-live-monitor-v0-plan.md for the evidence this is built from and
// what's confirmed versus still uncertain.
package doip

import "fmt"

// ProtocolVersion2012 is DoIP protocol version 0x02 (ISO 13400:2012),
// the version byte value the real-world reference implementation this
// package is evidenced against documents as correct for most ECUs.
const ProtocolVersion2012 byte = 0x02

// DefaultTCPPort is the standard DoIP TCP port (ISO 13400 Table 39),
// confirmed independently to be what BMW's ENET adapter (including
// MHD's) uses — see docs/mhd-live-monitor-v0-plan.md.
const DefaultTCPPort = 13400

// PayloadType is a DoIP generic header payload type (ISO 13400 Table 12).
// Only the values this package's Client actually sends or expects to
// receive are named; anything else is preserved as its raw numeric value.
type PayloadType uint16

const (
	PayloadGenericNegativeAck           PayloadType = 0x0000
	PayloadRoutingActivationRequest     PayloadType = 0x0005
	PayloadRoutingActivationResponse    PayloadType = 0x0006
	PayloadAliveCheckRequest            PayloadType = 0x0007
	PayloadAliveCheckResponse           PayloadType = 0x0008
	PayloadDiagnosticMessage            PayloadType = 0x8001
	PayloadDiagnosticMessagePositiveAck PayloadType = 0x8002
	PayloadDiagnosticMessageNegativeAck PayloadType = 0x8003
)

// ActivationType is a DoIP routing activation request's activation type
// (ISO 13400 Table 47). ActivationDefault is the specification's own
// default and the value this package uses unless told otherwise; no
// BMW-specific source confirms it's what MHD's adapter expects, only
// that it's the standard fallback — see
// docs/mhd-live-monitor-v0-plan.md.
type ActivationType byte

const (
	ActivationDefault                        ActivationType = 0x00
	ActivationDiagnosticRequiredByRegulation ActivationType = 0x01
	ActivationCentralSecurity                ActivationType = 0xE1
)

// RoutingActivationResponseCode is a DoIP routing activation response
// code (ISO 13400 Table 49).
type RoutingActivationResponseCode byte

const (
	RoutingDeniedUnknownSourceAddress       RoutingActivationResponseCode = 0x00
	RoutingDeniedAllSocketsRegisteredActive RoutingActivationResponseCode = 0x01
	RoutingDeniedSourceAddressMismatch      RoutingActivationResponseCode = 0x02
	RoutingDeniedSourceAddressRegistered    RoutingActivationResponseCode = 0x03
	RoutingDeniedMissingAuthentication      RoutingActivationResponseCode = 0x04
	RoutingDeniedRejectedConfirmation       RoutingActivationResponseCode = 0x05
	RoutingDeniedUnsupportedActivationType  RoutingActivationResponseCode = 0x06
	RoutingDeniedRequiresTLS                RoutingActivationResponseCode = 0x07
	RoutingSuccess                          RoutingActivationResponseCode = 0x10
	RoutingSuccessConfirmationRequired      RoutingActivationResponseCode = 0x11
)

func (code RoutingActivationResponseCode) String() string {
	switch code {
	case RoutingDeniedUnknownSourceAddress:
		return "denied: unknown source address"
	case RoutingDeniedAllSocketsRegisteredActive:
		return "denied: all sockets registered are active"
	case RoutingDeniedSourceAddressMismatch:
		return "denied: source address does not match"
	case RoutingDeniedSourceAddressRegistered:
		return "denied: source address already registered"
	case RoutingDeniedMissingAuthentication:
		return "denied: missing authentication"
	case RoutingDeniedRejectedConfirmation:
		return "denied: rejected confirmation"
	case RoutingDeniedUnsupportedActivationType:
		return "denied: unsupported activation type"
	case RoutingDeniedRequiresTLS:
		return "denied: requires TLS"
	case RoutingSuccess:
		return "success"
	case RoutingSuccessConfirmationRequired:
		return "success (confirmation required)"
	default:
		return fmt.Sprintf("unknown response code 0x%02X", byte(code))
	}
}

// header is DoIP's fixed 8-byte generic header: protocol version, its
// bitwise complement, payload type, and payload length (ISO 13400 Table
// 15) — evidenced against jacobschaer/python-doipclient's
// DoIPClient._pack_doip, read directly from source (messages.py/client.py,
// not a paraphrase), see docs/mhd-live-monitor-v0-plan.md.
type header struct {
	ProtocolVersion byte
	PayloadType     PayloadType
	PayloadLength   uint32
}

const headerSize = 8

// maxPayloadLength caps how large a single DoIP message body this package
// will allocate for, purely as a sanity guard against a malformed or
// corrupted length field — far larger than any message this package's
// use case (routing activation, a small UDS diagnostic exchange) needs.
const maxPayloadLength = 64 * 1024
