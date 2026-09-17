package doip

import (
	"errors"
	"reflect"
	"testing"
)

func TestPackMessageRoundTrip(t *testing.T) {
	payload := []byte{0x2C, 0x88, 0x01}
	message := packMessage(ProtocolVersion2012, PayloadDiagnosticMessage, payload)
	want := []byte{0x02, 0xFD, 0x80, 0x01, 0x00, 0x00, 0x00, 0x03, 0x2C, 0x88, 0x01}
	if !reflect.DeepEqual(message, want) {
		t.Fatalf("packMessage = % X, want % X", message, want)
	}

	var headerBuf [headerSize]byte
	copy(headerBuf[:], message[:headerSize])
	head, err := parseHeader(headerBuf)
	if err != nil {
		t.Fatal(err)
	}
	if head.ProtocolVersion != ProtocolVersion2012 || head.PayloadType != PayloadDiagnosticMessage || head.PayloadLength != uint32(len(payload)) {
		t.Fatalf("parsed header = %#v", head)
	}
}

func TestParseHeaderRejectsBadInverseByte(t *testing.T) {
	var headerBuf [headerSize]byte
	headerBuf[0] = ProtocolVersion2012
	headerBuf[1] = 0x00 // should be 0xFF ^ 0x02 = 0xFD
	if _, err := parseHeader(headerBuf); !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("error = %v, want ErrInvalidHeader", err)
	}
}

func TestPackMessageEmptyPayload(t *testing.T) {
	message := packMessage(ProtocolVersion2012, PayloadAliveCheckRequest, nil)
	want := []byte{0x02, 0xFD, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00}
	if !reflect.DeepEqual(message, want) {
		t.Fatalf("packMessage = % X, want % X", message, want)
	}
}

func TestRoutingActivationResponseCodeString(t *testing.T) {
	tests := []struct {
		code RoutingActivationResponseCode
		want string
	}{
		{RoutingSuccess, "success"},
		{RoutingDeniedUnsupportedActivationType, "denied: unsupported activation type"},
		{RoutingActivationResponseCode(0x99), "unknown response code 0x99"},
	}
	for _, test := range tests {
		if got := test.code.String(); got != test.want {
			t.Errorf("%#v.String() = %q, want %q", test.code, got, test.want)
		}
	}
}
