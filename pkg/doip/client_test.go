package doip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// startMockServer accepts exactly one TCP connection and runs handler
// against it in a background goroutine, returning the listener's address.
// handler uses t.Errorf (not Fatal, which isn't goroutine-safe) to report
// protocol mismatches.
func startMockServer(t *testing.T, handler func(t *testing.T, conn net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(t, conn)
	}()
	return listener.Addr().String()
}

func readMessage(t *testing.T, conn net.Conn) (PayloadType, []byte) {
	t.Helper()
	var headerBuf [headerSize]byte
	if _, err := io.ReadFull(conn, headerBuf[:]); err != nil {
		t.Errorf("mock server: read header: %v", err)
		return 0, nil
	}
	head, err := parseHeader(headerBuf)
	if err != nil {
		t.Errorf("mock server: %v", err)
		return 0, nil
	}
	body := make([]byte, head.PayloadLength)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Errorf("mock server: read payload: %v", err)
		return 0, nil
	}
	return head.PayloadType, body
}

func writeRoutingActivationResponse(conn net.Conn, clientAddress, responderAddress uint16, code RoutingActivationResponseCode) {
	payload := make([]byte, 9)
	binary.BigEndian.PutUint16(payload[0:2], clientAddress)
	binary.BigEndian.PutUint16(payload[2:4], responderAddress)
	payload[4] = byte(code)
	conn.Write(packMessage(ProtocolVersion2012, PayloadRoutingActivationResponse, payload))
}

func TestClientActivateAndSendDiagnostic(t *testing.T) {
	const responderAddress = uint16(0x1234)
	monitorReply := append([]byte{0x6C, 0x88}, make([]byte, 34)...)

	address := startMockServer(t, func(t *testing.T, conn net.Conn) {
		payloadType, body := readMessage(t, conn)
		if payloadType != PayloadRoutingActivationRequest || len(body) < 2 {
			t.Errorf("mock server: unexpected activation request: type=0x%04X body=% X", payloadType, body)
			return
		}
		clientAddress := binary.BigEndian.Uint16(body[0:2])
		writeRoutingActivationResponse(conn, clientAddress, responderAddress, RoutingSuccess)

		payloadType, body = readMessage(t, conn)
		if payloadType != PayloadDiagnosticMessage || len(body) < 4 {
			t.Errorf("mock server: unexpected diagnostic message: type=0x%04X body=% X", payloadType, body)
			return
		}
		source, target := binary.BigEndian.Uint16(body[0:2]), binary.BigEndian.Uint16(body[2:4])
		if target != responderAddress {
			t.Errorf("mock server: diagnostic target = 0x%04X, want 0x%04X", target, responderAddress)
		}
		if uds := body[4:]; !bytes.Equal(uds, []byte{0x2C, 0x88, 0x01}) {
			t.Errorf("mock server: uds request = % X, want 2C 88 01", uds)
		}

		ack := make([]byte, 5)
		binary.BigEndian.PutUint16(ack[0:2], responderAddress)
		binary.BigEndian.PutUint16(ack[2:4], source)
		conn.Write(packMessage(ProtocolVersion2012, PayloadDiagnosticMessagePositiveAck, ack))

		reply := make([]byte, 4+len(monitorReply))
		binary.BigEndian.PutUint16(reply[0:2], responderAddress)
		binary.BigEndian.PutUint16(reply[2:4], source)
		copy(reply[4:], monitorReply)
		conn.Write(packMessage(ProtocolVersion2012, PayloadDiagnosticMessage, reply))
	})

	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	target, err := client.Activate(0x0E00, ActivationDefault, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if target != responderAddress {
		t.Fatalf("Activate target = 0x%04X, want 0x%04X", target, responderAddress)
	}
	if got, ok := client.TargetAddress(); !ok || got != responderAddress {
		t.Fatalf("TargetAddress() = 0x%04X, %v", got, ok)
	}

	reply, err := client.SendDiagnostic([]byte{0x2C, 0x88, 0x01}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reply, monitorReply) {
		t.Fatalf("reply = % X, want % X", reply, monitorReply)
	}
}

func TestClientActivateRejectsDenied(t *testing.T) {
	address := startMockServer(t, func(t *testing.T, conn net.Conn) {
		_, body := readMessage(t, conn)
		clientAddress := binary.BigEndian.Uint16(body[0:2])
		writeRoutingActivationResponse(conn, clientAddress, 0, RoutingDeniedUnsupportedActivationType)
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Activate(0x0E00, ActivationDefault, time.Second); !errors.Is(err, ErrRoutingActivationDenied) {
		t.Fatalf("error = %v, want ErrRoutingActivationDenied", err)
	}
}

func TestClientSendDiagnosticNegativeAck(t *testing.T) {
	const responderAddress = uint16(0x1234)
	address := startMockServer(t, func(t *testing.T, conn net.Conn) {
		_, body := readMessage(t, conn)
		clientAddress := binary.BigEndian.Uint16(body[0:2])
		writeRoutingActivationResponse(conn, clientAddress, responderAddress, RoutingSuccess)

		_, body = readMessage(t, conn)
		source := binary.BigEndian.Uint16(body[0:2])
		nack := make([]byte, 5)
		binary.BigEndian.PutUint16(nack[0:2], responderAddress)
		binary.BigEndian.PutUint16(nack[2:4], source)
		nack[4] = 0x03 // UnknownTargetAddress
		conn.Write(packMessage(ProtocolVersion2012, PayloadDiagnosticMessageNegativeAck, nack))
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Activate(0x0E00, ActivationDefault, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SendDiagnostic([]byte{0x2C, 0x88, 0x01}, time.Second); !errors.Is(err, ErrDiagnosticNegativeAck) {
		t.Fatalf("error = %v, want ErrDiagnosticNegativeAck", err)
	}
}

func TestClientSendDiagnosticWithoutActivateFails(t *testing.T) {
	address := startMockServer(t, func(t *testing.T, conn net.Conn) {})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.SendDiagnostic([]byte{0x2C, 0x88, 0x01}, time.Second); !errors.Is(err, ErrNoTargetAddress) {
		t.Fatalf("error = %v, want ErrNoTargetAddress", err)
	}
}

func TestClientReceiveRejectsGenericNegativeAck(t *testing.T) {
	address := startMockServer(t, func(t *testing.T, conn net.Conn) {
		readMessage(t, conn)
		conn.Write(packMessage(ProtocolVersion2012, PayloadGenericNegativeAck, []byte{0x01}))
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Activate(0x0E00, ActivationDefault, time.Second); err == nil {
		t.Fatal("expected an error from a generic negative acknowledgement")
	}
}

func TestDialRejectsUnreachableAddress(t *testing.T) {
	if _, err := Dial("127.0.0.1:1", 50*time.Millisecond); err == nil {
		t.Fatal("expected a dial error for an unreachable address")
	}
}
