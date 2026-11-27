// Package hsfz implements a minimal client for HSFZ ("High Speed Fahrzeug
// Zugang"), the TCP diagnostic protocol BMW F-series vehicles speak over
// ENET on port 6801. It carries the same UDS (ISO 14229) requests as
// ISO 13400 DoIP (see pkg/doip), with a different framing and no routing
// activation step.
//
// Evidence: on 2026-11-27 the owner's F10 N55 with the MHD WiFi-ENET
// adapter refused every TCP connection to 192.168.4.1:13400 (DoIP) while
// MHD's own app, on the same phone and network, held connections to
// 192.168.4.1:6801. Only the destination port was observed (the phone's
// own socket table); nothing about MHD's traffic was inspected. The frame
// layout below is HSFZ as documented by open-source BMW ENET tooling. What
// is still unconfirmed against the real car is listed on each item, and in
// docs/mhd-live-monitor-v0-plan.md.
//
// Frame layout (all big-endian):
//
//	uint32 length   number of payload bytes following the 6-byte header
//	uint16 control  ControlWord
//	payload         for ControlDiagnostic: source, target, UDS bytes
package hsfz

import "fmt"

// DefaultTCPPort is the HSFZ diagnostic port on BMW's ENET gateway, and
// the port MHD's app was observed using on MHD's WiFi-ENET adapter.
const DefaultTCPPort = 6801

// ControlPort is the HSFZ control port on BMW's ENET gateway. It is a
// fallback candidate for the port scanner, not a diagnostic port.
const ControlPort = 6811

// DefaultTesterAddress is the 1-byte HSFZ source address conventionally
// used by external testers (ISTA, E-Sys and open-source tools) on BMW
// F-series vehicles. Unconfirmed against the owner's car.
const DefaultTesterAddress byte = 0xF4

// DefaultDMEAddress is the 1-byte HSFZ diagnostic address of the engine
// ECU (DME/DDE) on BMW F-series vehicles.
const DefaultDMEAddress byte = 0x12

// ControlWord is an HSFZ frame's 2-byte control field.
type ControlWord uint16

const (
	// ControlDiagnostic carries a UDS request (tester -> ECU) or response
	// (ECU -> tester): payload is source, target, UDS bytes.
	ControlDiagnostic ControlWord = 0x0001
	// ControlAcknowledge is the gateway echoing a received diagnostic
	// request back as a receipt; the ECU's real reply follows separately.
	ControlAcknowledge ControlWord = 0x0002
	// ControlAliveCheck is a keep-alive the gateway may send. This client
	// ignores it; whether a reply is required has not been confirmed.
	ControlAliveCheck ControlWord = 0x0012

	// Gateway error replies (0x0040-0x0045).
	ControlErrorTesterAddress ControlWord = 0x0040
	ControlErrorControlWord   ControlWord = 0x0041
	ControlErrorFormat        ControlWord = 0x0042
	ControlErrorTargetAddress ControlWord = 0x0043
	ControlErrorTooLarge      ControlWord = 0x0044
	ControlErrorNotReady      ControlWord = 0x0045
)

// IsError reports whether c is one of the gateway's error replies.
func (c ControlWord) IsError() bool {
	return c >= ControlErrorTesterAddress && c <= ControlErrorNotReady
}

// IsKnown reports whether c is a control word this package recognises.
// The port scanner uses it to tell an HSFZ peer from an unrelated service.
func (c ControlWord) IsKnown() bool {
	switch c {
	case ControlDiagnostic, ControlAcknowledge, ControlAliveCheck, 0x0011, 0x0013:
		return true
	}
	return c.IsError()
}

func (c ControlWord) String() string {
	switch c {
	case ControlDiagnostic:
		return "diagnostic"
	case ControlAcknowledge:
		return "acknowledge"
	case ControlAliveCheck:
		return "alive check"
	case ControlErrorTesterAddress:
		return "error: incorrect tester address"
	case ControlErrorControlWord:
		return "error: incorrect control word"
	case ControlErrorFormat:
		return "error: incorrect format"
	case ControlErrorTargetAddress:
		return "error: incorrect target address"
	case ControlErrorTooLarge:
		return "error: message too large"
	case ControlErrorNotReady:
		return "error: diagnostic application not ready"
	default:
		return fmt.Sprintf("control word 0x%04X", uint16(c))
	}
}

const headerSize = 6

// maxPayloadLength guards against allocating for a corrupt length field.
const maxPayloadLength = 64 * 1024
