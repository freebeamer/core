package hsfz

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// fakeGateway accepts one connection and runs serve on it.
func fakeGateway(t *testing.T, serve func(conn net.Conn)) string {
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
		serve(conn)
	}()
	return listener.Addr().String()
}

func readFrame(t *testing.T, conn net.Conn) (ControlWord, []byte) {
	t.Helper()
	var header [headerSize]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		t.Errorf("gateway: read header: %v", err)
		return 0, nil
	}
	length, control := ParseHeader(header)
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Errorf("gateway: read body: %v", err)
	}
	return control, body
}

func TestPackFrameLayout(t *testing.T) {
	got := PackFrame(ControlDiagnostic, []byte{0xF4, 0x12, 0x22, 0xF1, 0x90})
	want := []byte{0, 0, 0, 5, 0x00, 0x01, 0xF4, 0x12, 0x22, 0xF1, 0x90}
	if !bytes.Equal(got, want) {
		t.Fatalf("PackFrame = % X, want % X", got, want)
	}
	var header [headerSize]byte
	copy(header[:], got)
	if length, control := ParseHeader(header); length != 5 || control != ControlDiagnostic {
		t.Fatalf("ParseHeader = %d, %v", length, control)
	}
}

func TestSendDiagnosticSkipsAckPendingAndOtherECUs(t *testing.T) {
	address := fakeGateway(t, func(conn net.Conn) {
		control, body := readFrame(t, conn)
		if control != ControlDiagnostic || !bytes.Equal(body, []byte{0xF4, 0x12, 0x22, 0xF1, 0x90}) {
			t.Errorf("gateway got %v % X", control, body)
		}
		conn.Write(PackFrame(ControlAcknowledge, body))                                   // echo
		conn.Write(PackFrame(ControlAliveCheck, nil))                                     // keep-alive
		conn.Write(PackFrame(ControlDiagnostic, []byte{0x40, 0xF4, 0x62, 0x00}))          // other ECU
		conn.Write(PackFrame(ControlDiagnostic, []byte{0x12, 0xF4, 0x7F, 0x22, 0x78}))    // pending
		conn.Write(PackFrame(ControlDiagnostic, []byte{0x12, 0xF4, 0x62, 0xF1, 0x90, 1})) // reply
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reply, err := client.SendDiagnostic([]byte{0x22, 0xF1, 0x90}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x62, 0xF1, 0x90, 1}; !bytes.Equal(reply, want) {
		t.Fatalf("reply = % X, want % X", reply, want)
	}
}

func TestSendDiagnosticReturnsUDSNegativeResponse(t *testing.T) {
	address := fakeGateway(t, func(conn net.Conn) {
		readFrame(t, conn)
		conn.Write(PackFrame(ControlDiagnostic, []byte{0x12, 0xF4, 0x7F, 0x2C, 0x31}))
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reply, err := client.SendDiagnostic([]byte{0x2C, 0x88, 0x01}, time.Second)
	if err != nil || !bytes.Equal(reply, []byte{0x7F, 0x2C, 0x31}) {
		t.Fatalf("reply = % X, %v", reply, err)
	}
}

func TestSendDiagnosticGatewayError(t *testing.T) {
	address := fakeGateway(t, func(conn net.Conn) {
		_, body := readFrame(t, conn)
		conn.Write(PackFrame(ControlErrorTargetAddress, body))
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.SendDiagnostic([]byte{0x3E, 0x00}, time.Second); !errors.Is(err, ErrGateway) {
		t.Fatalf("err = %v, want ErrGateway", err)
	}
}

func TestSendDiagnosticTimeout(t *testing.T) {
	address := fakeGateway(t, func(conn net.Conn) {
		readFrame(t, conn)
		time.Sleep(300 * time.Millisecond)
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.SendDiagnostic([]byte{0x3E, 0x00}, 100*time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestReceiveRejectsOversizedFrame(t *testing.T) {
	address := fakeGateway(t, func(conn net.Conn) {
		readFrame(t, conn)
		conn.Write([]byte{0x7F, 0xFF, 0xFF, 0xFF, 0x00, 0x01})
	})
	client, err := Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.SendDiagnostic([]byte{0x3E, 0x00}, time.Second); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("err = %v, want ErrMessageTooLarge", err)
	}
}
