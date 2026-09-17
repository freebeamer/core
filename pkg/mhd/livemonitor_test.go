package mhd

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
	"testing"
	"time"

	"github.com/freebeamer/core/pkg/doip"
)

// rawValues gives a known raw integer per Fields entry (by Name),
// including at least one negative value for every signed field, so
// decodeMonitorReply's offset/width/signedness handling is exercised
// for every field, not just a happy-path subset.
var rawValues = map[string]int64{
	"RPM":            2500,
	"LOAD":           -500,
	"BOOST":          10000,
	"BOOSTTARGET":    20000,
	"BOOSTDEVIATION": -1000,
	"BOOSTDEVGRAD":   500,
	"ENGINETEMP":     -1000,
	"GEAR":           3,
	"ETHANOLCONTENT": 85,
	"WGDISTRIBFAC":   16384,
	"TURBOMASSFLOW":  3600,
	"TURBOMASSFLOW2": 3600,
	"CMPRMASSFLOW":   3600,
	"CMPRMASSFLOW2":  3600,
	"BOOSTSETPOINTF": 8192,
	"SPEED":          120,
	"AMBIENTTEMP":    -250,
	"INTAKEAIRTEMP":  400,
	"MHDMAFCALC":     2000,
}

// wantValues gives the expected converted Value for each raw entry
// above, computed independently from MG1.adx's own <MATH equation> for
// that field (not by calling pkg/expression), so the test cross-checks
// the whole decode pipeline rather than only itself.
var wantValues = map[string]float64{
	"RPM":            2500,
	"LOAD":           -500 * 0.01,
	"BOOST":          10000 * 0.001133107328125,
	"BOOSTTARGET":    20000 * 0.0018129717,
	"BOOSTDEVIATION": -1000 * 0.0018129717,
	"BOOSTDEVGRAD":   500 * 0.0018129717,
	"ENGINETEMP":     -1000 * 0.01,
	"GEAR":           3,
	"ETHANOLCONTENT": 85,
	"WGDISTRIBFAC":   16384.0 / 16384,
	"TURBOMASSFLOW":  3600 * 0.0390625 / 3.6,
	"TURBOMASSFLOW2": 3600 * 0.03125 / 3.6,
	"CMPRMASSFLOW":   3600 * 0.0625 / 3.6,
	"CMPRMASSFLOW2":  3600 * 0.03125 / 3.6,
	"BOOSTSETPOINTF": 8192.0 / 8192,
	"SPEED":          120,
	"AMBIENTTEMP":    -250.0 / 10,
	"INTAKEAIRTEMP":  400.0 / 10,
	"MHDMAFCALC":     2000 * 0.03472225,
}

// buildMonitorReply encodes rawValues into a full MonitorReplyLength
// reply (header + body) per each Fields entry's ByteOffset/SizeBits.
func buildMonitorReply(t *testing.T) []byte {
	t.Helper()
	reply := make([]byte, MonitorReplyLength)
	copy(reply, MonitorReplyHeader)
	body := reply[len(MonitorReplyHeader):]
	for _, field := range Fields {
		raw, ok := rawValues[field.Name]
		if !ok {
			t.Fatalf("no raw test value for field %s", field.Name)
		}
		switch field.SizeBits {
		case 8:
			body[field.ByteOffset] = byte(int8(raw))
		case 16:
			binary.LittleEndian.PutUint16(body[field.ByteOffset:], uint16(int16(raw)))
		default:
			t.Fatalf("field %s: unsupported size %d bits", field.Name, field.SizeBits)
		}
	}
	return reply
}

func TestDecodeMonitorReply(t *testing.T) {
	reply := buildMonitorReply(t)

	values, err := DecodeMonitorReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != len(Fields) {
		t.Fatalf("len(values) = %d, want %d", len(values), len(Fields))
	}

	for _, v := range values {
		wantRaw := rawValues[v.Field.Name]
		if v.Raw != wantRaw {
			t.Errorf("field %s: Raw = %d, want %d", v.Field.Name, v.Raw, wantRaw)
		}
		want := wantValues[v.Field.Name]
		if math.Abs(v.Value-want) > 1e-9 {
			t.Errorf("field %s: Value = %v, want %v", v.Field.Name, v.Value, want)
		}
	}

	got, ok := ByName(values, "LOAD")
	if !ok {
		t.Fatal("ByName(\"LOAD\") not found")
	}
	if got.Raw != -500 {
		t.Fatalf("ByName(\"LOAD\").Raw = %d, want -500", got.Raw)
	}

	if _, ok := ByName(values, "NOT_A_FIELD"); ok {
		t.Fatal("ByName(\"NOT_A_FIELD\") unexpectedly found")
	}
}

func TestDecodeMonitorReplyRejectsWrongLength(t *testing.T) {
	if _, err := DecodeMonitorReply(make([]byte, MonitorReplyLength-1)); err == nil {
		t.Fatal("expected an error for a short reply")
	}
}

func TestDecodeMonitorReplyRejectsWrongHeader(t *testing.T) {
	reply := buildMonitorReply(t)
	reply[0] = 0x7F
	if _, err := DecodeMonitorReply(reply); err == nil {
		t.Fatal("expected an error for a mismatched header")
	}
}

// startMockServer accepts exactly one TCP connection and runs handler
// against it in a background goroutine, returning the listener's
// address. Mirrors pkg/doip's own test helper of the same name.
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

func TestMonitorRead(t *testing.T) {
	const responderAddress = uint16(0x1234)
	monitorReply := buildMonitorReply(t)

	address := startMockServer(t, func(t *testing.T, conn net.Conn) {
		var headerBuf [8]byte
		if _, err := io.ReadFull(conn, headerBuf[:]); err != nil {
			t.Errorf("mock server: read activation header: %v", err)
			return
		}
		length := binary.BigEndian.Uint32(headerBuf[4:8])
		activationBody := make([]byte, length)
		if _, err := io.ReadFull(conn, activationBody); err != nil {
			t.Errorf("mock server: read activation body: %v", err)
			return
		}
		clientAddress := binary.BigEndian.Uint16(activationBody[0:2])

		activationResponse := make([]byte, 9)
		binary.BigEndian.PutUint16(activationResponse[0:2], clientAddress)
		binary.BigEndian.PutUint16(activationResponse[2:4], responderAddress)
		activationResponse[4] = byte(doip.RoutingSuccess)
		conn.Write(packDoipMessage(doip.PayloadRoutingActivationResponse, activationResponse))

		if _, err := io.ReadFull(conn, headerBuf[:]); err != nil {
			t.Errorf("mock server: read diagnostic header: %v", err)
			return
		}
		length = binary.BigEndian.Uint32(headerBuf[4:8])
		diagnosticBody := make([]byte, length)
		if _, err := io.ReadFull(conn, diagnosticBody); err != nil {
			t.Errorf("mock server: read diagnostic body: %v", err)
			return
		}
		source := binary.BigEndian.Uint16(diagnosticBody[0:2])
		if uds := diagnosticBody[4:]; !bytes.Equal(uds, MonitorRequest) {
			t.Errorf("mock server: uds request = % X, want % X", uds, MonitorRequest)
		}

		reply := make([]byte, 4+len(monitorReply))
		binary.BigEndian.PutUint16(reply[0:2], responderAddress)
		binary.BigEndian.PutUint16(reply[2:4], source)
		copy(reply[4:], monitorReply)
		conn.Write(packDoipMessage(doip.PayloadDiagnosticMessage, reply))
	})

	client, err := doip.Dial(address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Activate(0x0E00, doip.ActivationDefault, time.Second); err != nil {
		t.Fatal(err)
	}

	monitor := NewMonitor(client)
	values, err := monitor.Read(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ByName(values, "RPM")
	if !ok || got.Raw != rawValues["RPM"] {
		t.Fatalf("ByName(\"RPM\") = %+v, %v", got, ok)
	}
}

// packDoipMessage renders one DoIP message using this package's own
// dependency on ProtocolVersion2012, mirroring the private packMessage
// helper pkg/doip keeps unexported for its own tests.
func packDoipMessage(payloadType doip.PayloadType, payload []byte) []byte {
	message := make([]byte, 8+len(payload))
	message[0] = doip.ProtocolVersion2012
	message[1] = 0xFF ^ doip.ProtocolVersion2012
	binary.BigEndian.PutUint16(message[2:4], uint16(payloadType))
	binary.BigEndian.PutUint32(message[4:8], uint32(len(payload)))
	copy(message[8:], payload)
	return message
}
