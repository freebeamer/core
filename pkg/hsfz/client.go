package hsfz

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var (
	// ErrInvalidFrame is returned for a frame whose length field is
	// implausible or whose diagnostic payload is too short.
	ErrInvalidFrame = errors.New("hsfz: invalid frame")
	// ErrMessageTooLarge is returned when a frame's declared length
	// exceeds maxPayloadLength.
	ErrMessageTooLarge = errors.New("hsfz: message too large")
	// ErrGateway is returned when the gateway answers with one of its
	// error control words (0x0040-0x0045).
	ErrGateway = errors.New("hsfz: gateway error")
	// ErrTimeout is returned when no matching ECU reply arrives in time.
	ErrTimeout = errors.New("hsfz: timed out waiting for diagnostic response")
)

// Client is a minimal HSFZ client over one TCP connection. Unlike DoIP
// there is no routing activation: diagnostic messages can be sent as
// soon as the connection is open.
type Client struct {
	conn          net.Conn
	sourceAddress byte
	targetAddress byte
}

// Dial opens a TCP connection to an HSFZ gateway (host:port, normally
// DefaultTCPPort) with the default tester and DME addresses.
func Dial(address string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("hsfz: dial %s: %w", address, err)
	}
	return NewClient(conn), nil
}

// NewClient wraps an already-open connection.
func NewClient(conn net.Conn) *Client {
	return &Client{conn: conn, sourceAddress: DefaultTesterAddress, targetAddress: DefaultDMEAddress}
}

// Close closes the underlying TCP connection.
func (c *Client) Close() error { return c.conn.Close() }

// SetSourceAddress overrides the 1-byte tester address (default 0xF4).
func (c *Client) SetSourceAddress(address byte) { c.sourceAddress = address }

// SetTargetAddress overrides the 1-byte ECU address (default 0x12, DME).
func (c *Client) SetTargetAddress(address byte) { c.targetAddress = address }

// TargetAddress returns the current 1-byte ECU address.
func (c *Client) TargetAddress() byte { return c.targetAddress }

// SendDiagnostic sends a UDS request to the target ECU and returns the
// ECU's UDS reply. The gateway's acknowledge echo, alive checks and
// replies from other ECUs are skipped. A UDS "response pending" negative
// response (0x7F, service, 0x78) keeps waiting for the final reply; any
// other UDS negative response is returned as ordinary bytes for the
// caller to interpret, as pkg/doip does.
func (c *Client) SendDiagnostic(request []byte, timeout time.Duration) ([]byte, error) {
	payload := make([]byte, 2+len(request))
	payload[0] = c.sourceAddress
	payload[1] = c.targetAddress
	copy(payload[2:], request)
	if err := c.send(ControlDiagnostic, payload); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ErrTimeout
		}
		control, body, err := c.receive(remaining)
		if err != nil {
			return nil, err
		}
		switch {
		case control.IsError():
			return nil, fmt.Errorf("%w: %s", ErrGateway, control)
		case control != ControlDiagnostic:
			continue // acknowledge echo, alive check, anything else
		}
		if len(body) < 2 {
			return nil, fmt.Errorf("%w: diagnostic payload of %d byte(s)", ErrInvalidFrame, len(body))
		}
		if body[0] != c.targetAddress || body[1] != c.sourceAddress {
			continue // another ECU, or our own request echoed as data
		}
		uds := body[2:]
		if len(uds) >= 3 && uds[0] == 0x7F && uds[2] == 0x78 {
			continue // response pending
		}
		return uds, nil
	}
}

// ReadFrame reads one raw frame. Exposed for protocol detection by the
// port scanner (pkg/enet), which needs to know whether a peer speaks HSFZ
// at all before interpreting anything else.
func (c *Client) ReadFrame(timeout time.Duration) (ControlWord, []byte, error) {
	return c.receive(timeout)
}

func (c *Client) send(control ControlWord, payload []byte) error {
	if _, err := c.conn.Write(PackFrame(control, payload)); err != nil {
		return fmt.Errorf("hsfz: write: %w", err)
	}
	return nil
}

func (c *Client) receive(timeout time.Duration) (ControlWord, []byte, error) {
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, nil, fmt.Errorf("hsfz: set read deadline: %w", err)
	}
	var header [headerSize]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return 0, nil, ErrTimeout
		}
		return 0, nil, fmt.Errorf("hsfz: read header: %w", err)
	}
	length, control := ParseHeader(header)
	if length > maxPayloadLength {
		return 0, nil, fmt.Errorf("%w: %d byte(s)", ErrMessageTooLarge, length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return 0, nil, fmt.Errorf("hsfz: read payload: %w", err)
	}
	return control, body, nil
}

// PackFrame renders one HSFZ frame: 6-byte header followed by payload.
func PackFrame(control ControlWord, payload []byte) []byte {
	frame := make([]byte, headerSize+len(payload))
	binary.BigEndian.PutUint32(frame[0:4], uint32(len(payload)))
	binary.BigEndian.PutUint16(frame[4:6], uint16(control))
	copy(frame[headerSize:], payload)
	return frame
}

// ParseHeader decodes the 6-byte HSFZ header.
func ParseHeader(raw [headerSize]byte) (length uint32, control ControlWord) {
	return binary.BigEndian.Uint32(raw[0:4]), ControlWord(binary.BigEndian.Uint16(raw[4:6]))
}
