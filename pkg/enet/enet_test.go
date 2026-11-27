package enet

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/freebeamer/core/pkg/doip"
	"github.com/freebeamer/core/pkg/hsfz"
)

// listen starts a server on a free localhost port; serve runs per conn.
func listen(t *testing.T, serve func(net.Conn)) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer conn.Close(); serve(conn) }()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// closedPort returns a localhost port with nothing listening on it.
func closedPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// hsfzECU acks every diagnostic request and, if ecuAnswers, replies to
// TesterPresent (0x7E 0x00) and echoes other requests as 0x62 + request.
func hsfzECU(ecuAnswers bool) func(net.Conn) {
	return func(conn net.Conn) {
		for {
			var header [6]byte
			if _, err := io.ReadFull(conn, header[:]); err != nil {
				return
			}
			length, control := hsfz.ParseHeader(header)
			body := make([]byte, length)
			if _, err := io.ReadFull(conn, body); err != nil || control != hsfz.ControlDiagnostic {
				return
			}
			conn.Write(hsfz.PackFrame(hsfz.ControlAcknowledge, body))
			if !ecuAnswers {
				continue
			}
			reply := []byte{body[1], body[0]}
			if bytes.Equal(body[2:], []byte{0x3E, 0x00}) {
				reply = append(reply, 0x7E, 0x00)
			} else {
				reply = append(append(reply, 0x62), body[2:]...)
			}
			conn.Write(hsfz.PackFrame(hsfz.ControlDiagnostic, reply))
		}
	}
}

// doipEntity answers routing activation as logical address 0x0010.
func doipEntity(conn net.Conn) {
	var header [8]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return
	}
	length := binary.BigEndian.Uint32(header[4:8])
	if length > 64 {
		return // not a routing activation (e.g. an HSFZ probe): stay silent
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}
	resp := make([]byte, 9)
	copy(resp[0:2], body[0:2])
	binary.BigEndian.PutUint16(resp[2:4], 0x0010)
	resp[4] = byte(doip.RoutingSuccess)
	msg := make([]byte, 8+len(resp))
	msg[0], msg[1] = doip.ProtocolVersion2012, 0xFF^doip.ProtocolVersion2012
	binary.BigEndian.PutUint16(msg[2:4], uint16(doip.PayloadRoutingActivationResponse))
	binary.BigEndian.PutUint32(msg[4:8], uint32(len(resp)))
	copy(msg[8:], resp)
	conn.Write(msg)
	io.Copy(io.Discard, conn)
}

func fastOptions(port int, sweep ...int) Options {
	return Options{
		Port:         port,
		DialTimeout:  300 * time.Millisecond,
		ProbeTimeout: 300 * time.Millisecond,
		SweepPorts:   sweep,
		SweepTimeout: 200 * time.Millisecond,
	}
}

func TestConnectHSFZOnPreferredPort(t *testing.T) {
	port := listen(t, hsfzECU(true))
	link, err := Connect(context.Background(), "127.0.0.1", fastOptions(port, []int{}...))
	if err != nil {
		t.Fatal(err)
	}
	defer link.Close()
	if link.Protocol != HSFZ || link.Port != port || link.Scanned || link.Target != 0x12 {
		t.Fatalf("link = %+v", link)
	}
	reply, err := link.SendDiagnostic([]byte{0x2C, 0x88, 0x01}, time.Second)
	if err != nil || !bytes.Equal(reply, []byte{0x62, 0x2C, 0x88, 0x01}) {
		t.Fatalf("reply = % X, %v", reply, err)
	}
}

func TestConnectFallsBackToSweep(t *testing.T) {
	preferred := closedPort(t)
	other := closedPort(t)
	actual := listen(t, hsfzECU(true))
	var log []string
	opts := fastOptions(preferred, other, actual)
	opts.Log = func(s string) { log = append(log, s) }
	link, err := Connect(context.Background(), "127.0.0.1", opts)
	if err != nil {
		t.Fatalf("%v\nlog:\n%s", err, strings.Join(log, "\n"))
	}
	defer link.Close()
	if link.Port != actual || !link.Scanned || link.Protocol != HSFZ {
		t.Fatalf("link = %+v", link)
	}
}

func TestConnectDetectsDoIP(t *testing.T) {
	port := listen(t, doipEntity)
	link, err := Connect(context.Background(), "127.0.0.1", fastOptions(port, []int{}...))
	if err != nil {
		t.Fatal(err)
	}
	defer link.Close()
	if link.Protocol != DoIP || link.Target != 0x0010 || link.Responder != 0x0010 {
		t.Fatalf("link = %+v", link)
	}
}

func TestConnectForcedProtocolSkipsTheOther(t *testing.T) {
	port := listen(t, doipEntity)
	opts := fastOptions(port, []int{}...)
	opts.Protocol = DoIP
	start := time.Now()
	link, err := Connect(context.Background(), "127.0.0.1", opts)
	if err != nil {
		t.Fatal(err)
	}
	link.Close()
	if time.Since(start) > 250*time.Millisecond {
		t.Fatalf("forced DoIP took %v: HSFZ probe should have been skipped", time.Since(start))
	}
}

func TestConnectReportsSilentECU(t *testing.T) {
	port := listen(t, hsfzECU(false))
	opts := fastOptions(port, []int{}...)
	opts.NoScan = true
	_, err := Connect(context.Background(), "127.0.0.1", opts)
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "speaks hsfz") || !strings.Contains(err.Error(), "ignition") {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectNothingFound(t *testing.T) {
	junk := listen(t, func(conn net.Conn) { conn.Write([]byte("HTTP/1.0 400 Bad Request\r\n\r\n")) })
	_, err := Connect(context.Background(), "127.0.0.1", fastOptions(closedPort(t), junk))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "open port(s) [") {
		t.Fatalf("error should list the open ports found: %v", err)
	}
}

func TestConnectNoScan(t *testing.T) {
	actual := listen(t, hsfzECU(true))
	opts := fastOptions(closedPort(t), actual)
	opts.NoScan = true
	if _, err := Connect(context.Background(), "127.0.0.1", opts); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound with scanning disabled", err)
	}
}

func TestSweepHonoursSkipPorts(t *testing.T) {
	actual := listen(t, hsfzECU(true))
	opts := fastOptions(closedPort(t), actual)
	opts.SkipPorts = []int{actual}
	if _, err := Connect(context.Background(), "127.0.0.1", opts); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v: a skipped port must not be probed", err)
	}
}

func TestParseProtocol(t *testing.T) {
	for in, want := range map[string]Protocol{"": Auto, "auto": Auto, "HSFZ": HSFZ, "doip": DoIP} {
		if got, err := ParseProtocol(in); err != nil || got != want {
			t.Errorf("ParseProtocol(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseProtocol("kline"); err == nil {
		t.Error("ParseProtocol(kline) should fail")
	}
}

// A 16-bit target is DoIP-only: auto-detect must not spend a connection
// on an HSFZ probe (adapters may accept a single client).
func TestConnectWideTargetGoesStraightToDoIP(t *testing.T) {
	accepted := 0
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		accepted++
		listener.Close() // single-client adapter
		doipEntity(conn)
	}()
	target := uint16(0x5678)
	opts := fastOptions(listener.Addr().(*net.TCPAddr).Port, []int{}...)
	opts.Target = &target
	link, err := Connect(context.Background(), "127.0.0.1", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer link.Close()
	if link.Protocol != DoIP || link.Target != 0x5678 || link.Responder != 0x0010 || accepted != 1 {
		t.Fatalf("link = %+v, accepted = %d", link, accepted)
	}
}
