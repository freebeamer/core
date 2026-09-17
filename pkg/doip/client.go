package doip

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var (
	// ErrRoutingActivationDenied is returned when a DoIP entity responds
	// to a routing activation request with anything other than a success
	// code. Unwrap or inspect the wrapped RoutingActivationResponseCode
	// for the reason.
	ErrRoutingActivationDenied = errors.New("doip: routing activation denied")
	// ErrDiagnosticNegativeAck is returned when the target DoIP entity
	// rejects a diagnostic message at the DoIP level (distinct from a UDS
	// negative response, which is a normal, successfully-transported UDS
	// payload this package returns as-is for the caller to interpret).
	ErrDiagnosticNegativeAck = errors.New("doip: diagnostic message negative acknowledgement")
	// ErrUnexpectedPayloadType is returned when a response's payload type
	// doesn't match what the calling operation was waiting for.
	ErrUnexpectedPayloadType = errors.New("doip: unexpected payload type")
	// ErrNoTargetAddress is returned by SendDiagnostic when neither
	// Activate nor SetTargetAddress has established a target yet.
	ErrNoTargetAddress = errors.New("doip: no target address set")
)

// Client is a minimal DoIP client over one TCP connection: routing
// activation plus request/response diagnostic (UDS) messaging. It does
// not implement UDP vehicle discovery, TLS, or anything on DoIP's ECU
// reprogramming/emulation side — see docs/mhd-live-monitor-v0-plan.md for
// scope and evidence.
type Client struct {
	conn            net.Conn
	protocolVersion byte
	sourceAddress   uint16
	targetAddress   uint16
	haveTarget      bool
}

// Dial opens a TCP connection to a DoIP entity (host:port, typically port
// DefaultTCPPort) without yet performing routing activation — call
// Activate next.
func Dial(address string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("doip: dial %s: %w", address, err)
	}
	return &Client{conn: conn, protocolVersion: ProtocolVersion2012}, nil
}

// Close closes the underlying TCP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// TargetAddress returns the diagnostic target address Activate
// discovered (or SetTargetAddress set), and whether one has been
// established at all.
func (c *Client) TargetAddress() (address uint16, ok bool) {
	return c.targetAddress, c.haveTarget
}

// Activate performs DoIP routing activation (payload types 0x0005/0x0006,
// ISO 13400 Tables 46-49): required before any diagnostic message can be
// exchanged. sourceAddress is this client's own chosen logical address —
// any value in the "reserved for addresses of client" range (0x0E00 to
// 0x0FFF, ISO 13400 Table 13) is appropriate; this package does not pick
// one for the caller, since it's a value within the caller's own control,
// not something evidence needs to confirm.
//
// The responding entity's own logical address is returned and remembered
// as the default diagnostic target address. For a single-ECU adapter like
// MHD's, this is expected to be the DME itself, but that has not been
// independently confirmed against a real packet capture — callers that
// discover (from their own capture, or a negative acknowledgement) that a
// different target address is needed can override it with
// SetTargetAddress.
func (c *Client) Activate(sourceAddress uint16, activationType ActivationType, timeout time.Duration) (targetAddress uint16, err error) {
	c.sourceAddress = sourceAddress
	payload := make([]byte, 7)
	binary.BigEndian.PutUint16(payload[0:2], sourceAddress)
	payload[2] = byte(activationType)
	// payload[3:7] is the reserved field and stays zero.
	if err := c.send(PayloadRoutingActivationRequest, payload); err != nil {
		return 0, err
	}
	payloadType, body, err := c.receive(timeout)
	if err != nil {
		return 0, err
	}
	if payloadType != PayloadRoutingActivationResponse {
		return 0, fmt.Errorf("%w: got 0x%04X, want RoutingActivationResponse (0x%04X)", ErrUnexpectedPayloadType, payloadType, PayloadRoutingActivationResponse)
	}
	if len(body) < 9 {
		return 0, fmt.Errorf("%w: routing activation response too short (%d byte(s))", ErrInvalidHeader, len(body))
	}
	responderAddress := binary.BigEndian.Uint16(body[2:4])
	code := RoutingActivationResponseCode(body[4])
	if code != RoutingSuccess && code != RoutingSuccessConfirmationRequired {
		return 0, fmt.Errorf("%w: %s", ErrRoutingActivationDenied, code)
	}
	c.targetAddress = responderAddress
	c.haveTarget = true
	return responderAddress, nil
}

// SetTargetAddress overrides the diagnostic target address Activate
// discovered, for a network where the DoIP entity that responds to
// routing activation isn't the same one actually holding the requested
// data (e.g. a gateway in front of the DME rather than the DME itself) —
// not evidenced as necessary for MHD's adapter, but available if a real
// capture shows otherwise.
func (c *Client) SetTargetAddress(address uint16) {
	c.targetAddress = address
	c.haveTarget = true
}

// SendDiagnostic sends a raw UDS request to the current target address
// (see Activate/SetTargetAddress) and returns the UDS response bytes from
// the ECU's own DiagnosticMessage reply, after first consuming and
// validating the DoIP-level acknowledgement every diagnostic message gets
// (ISO 13400 5.3.3, Tables 21-26). A UDS-level negative response (e.g. a
// standard 0x7F NRC byte) is returned as ordinary response bytes for the
// caller to interpret — that's a successfully transported UDS exchange,
// distinct from ErrDiagnosticNegativeAck, which is DoIP itself rejecting
// the message (bad address, message too large, ...).
func (c *Client) SendDiagnostic(request []byte, timeout time.Duration) ([]byte, error) {
	if !c.haveTarget {
		return nil, ErrNoTargetAddress
	}
	payload := make([]byte, 4+len(request))
	binary.BigEndian.PutUint16(payload[0:2], c.sourceAddress)
	binary.BigEndian.PutUint16(payload[2:4], c.targetAddress)
	copy(payload[4:], request)
	if err := c.send(PayloadDiagnosticMessage, payload); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("doip: timed out waiting for diagnostic response")
		}
		payloadType, body, err := c.receive(remaining)
		if err != nil {
			return nil, err
		}
		switch payloadType {
		case PayloadDiagnosticMessagePositiveAck:
			// Just a DoIP-level receipt confirmation; the actual UDS
			// reply (if any) is a separate DiagnosticMessage that
			// follows.
			continue
		case PayloadDiagnosticMessageNegativeAck:
			if len(body) < 5 {
				return nil, fmt.Errorf("%w: malformed negative acknowledgement", ErrDiagnosticNegativeAck)
			}
			return nil, fmt.Errorf("%w: code 0x%02X", ErrDiagnosticNegativeAck, body[4])
		case PayloadDiagnosticMessage:
			if len(body) < 4 {
				return nil, fmt.Errorf("%w: malformed diagnostic message", ErrInvalidHeader)
			}
			return body[4:], nil
		default:
			// Unrelated DoIP traffic (e.g. an alive check request);
			// ignore and keep waiting for our own response.
			continue
		}
	}
}

func (c *Client) send(payloadType PayloadType, payload []byte) error {
	if _, err := c.conn.Write(packMessage(c.protocolVersion, payloadType, payload)); err != nil {
		return fmt.Errorf("doip: write: %w", err)
	}
	return nil
}

func (c *Client) receive(timeout time.Duration) (PayloadType, []byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, nil, fmt.Errorf("doip: set read deadline: %w", err)
	}
	var headerBuf [headerSize]byte
	if _, err := io.ReadFull(c.conn, headerBuf[:]); err != nil {
		return 0, nil, fmt.Errorf("doip: read header: %w", err)
	}
	head, err := parseHeader(headerBuf)
	if err != nil {
		return 0, nil, err
	}
	if head.PayloadLength > maxPayloadLength {
		return 0, nil, fmt.Errorf("%w: %d byte(s)", ErrMessageTooLarge, head.PayloadLength)
	}
	body := make([]byte, head.PayloadLength)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return 0, nil, fmt.Errorf("doip: read payload: %w", err)
	}
	if head.PayloadType == PayloadGenericNegativeAck {
		var code byte
		if len(body) > 0 {
			code = body[0]
		}
		return 0, nil, fmt.Errorf("doip: generic negative acknowledgement, code 0x%02X", code)
	}
	return head.PayloadType, body, nil
}
