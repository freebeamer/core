// Package mhd decodes the live-monitor UDS exchange MHD Tuning's own
// published TunerPro RT ADX file defines for BMW MG1-family DMEs: a
// fixed request/reply pair carried over pkg/doip (ISO 13400 DoIP, the
// transport MHD's WiFi-ENET adapter — and any compatible generic ENET
// hardware — uses). Only the read-only monitor path is implemented; the
// write/live-tuning (calibration RAM patching) mechanism is not
// evidenced anywhere outside MHD's closed MHD.dll plugin and is
// explicitly out of scope. See docs/mhd-live-monitor-v0-plan.md for the
// evidence this package is built from, including the exact source file
// fetched and the field-by-field signedness derivation.
package mhd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/freebeamer/core/pkg/doip"
	"github.com/freebeamer/core/pkg/expression"
)

// MonitorRequest is the fixed 3-byte UDS request MG1.adx's MONITORREQUEST
// node sends to start a monitor exchange: service 0x2C
// (DynamicallyDefineDataIdentifier, ISO 14229) with sub-parameters
// 0x88 0x01.
var MonitorRequest = []byte{0x2C, 0x88, 0x01}

// MonitorReplyHeader is MG1.adx's MONITORREPLY <headerstring> (36 88 ->
// six c eight eight): the fixed 2-byte prefix identifying a monitor
// reply. 0x6C is the standard ISO 14229 positive-response ID for service
// 0x2C (service | 0x40).
var MonitorReplyHeader = []byte{0x6C, 0x88}

// MonitorReplyBodyLength is MG1.adx's declared <packetbodylength> (36):
// the number of data bytes following MonitorReplyHeader. It exactly
// spans every Fields entry's ByteOffset+SizeBits/8, from RPM at offset 0
// to MHD+MAF at offset 0x22 (16 bits) ending at byte 36.
const MonitorReplyBodyLength = 36

// monitorReplyHeaderLength mirrors len(MonitorReplyHeader) as a
// constant (MonitorReplyHeader is a var, so its len isn't one) so
// MonitorReplyLength can be a const too.
const monitorReplyHeaderLength = 2

// MonitorReplyLength is a full monitor reply's total length: header
// followed by the 36-byte body.
const MonitorReplyLength = monitorReplyHeaderLength + MonitorReplyBodyLength

// Field describes one scalar within a monitor reply's body, transcribed
// directly from one <ADXVALUE> element in MG1.adx.
type Field struct {
	// Name is the ADX <ADXVALUE id="...">.
	Name string
	// Title is the ADX <ADXVALUE title="...">, a human-readable label.
	Title string
	// Units is the ADX <units> value, transcribed as declared. RPM's
	// declared "l/min" looks like an authoring mistake in MHD's own
	// file (not a transcription error here) but is kept as-is rather
	// than silently corrected.
	Units string
	// ByteOffset is the ADX <packetoffset>: an offset into the 36-byte
	// body (MonitorReplyHeader is not counted).
	ByteOffset int
	// SizeBits is the ADX <sizeinbits> (8 when the file omits it, per
	// its own <DEFAULTS datasizeinbits="8">).
	SizeBits int
	// Signed reports whether the raw field is two's-complement.
	// MG1.adx's <DEFAULTS signed="0"> applies to every field — no
	// <ADXVALUE> in the file carries its own signed="..." override —
	// but several fields' declared <range> is only reachable at all if
	// the raw value is actually signed: e.g. LOAD's range
	// -327.68..327.67 at equation X*0.01 spans exactly the signed
	// int16 domain (-32768..32767) and never the unsigned one. Signed
	// is set true for those fields as an evidence-derived correction
	// to the file's blanket default, cross-checked by comparing each
	// field's declared range against what its equation produces over
	// the full signed vs. unsigned raw domain — not an independently
	// confirmed fact. See docs/mhd-live-monitor-v0-plan.md.
	Signed bool
	// Conversion is the ADX <MATH equation>: a freehorse-expression-v1
	// expression (see pkg/expression) in the raw field value X.
	Conversion string
}

// Fields is every <ADXVALUE> MG1.adx's MONITORMACRO reply carries, in
// the file's own declaration order (ascending ByteOffset).
var Fields = []Field{
	{Name: "RPM", Title: "RPM", Units: "l/min", ByteOffset: 0x00, SizeBits: 16, Signed: false, Conversion: "X"},
	{Name: "LOAD", Title: "Load", Units: "%", ByteOffset: 0x02, SizeBits: 16, Signed: true, Conversion: "X*0.01"},
	{Name: "BOOST", Title: "Boost", Units: "psi", ByteOffset: 0x04, SizeBits: 16, Signed: false, Conversion: "X*0.001133107328125"},
	{Name: "BOOSTTARGET", Title: "Boost Target", Units: "psi", ByteOffset: 0x06, SizeBits: 16, Signed: false, Conversion: "X*0.0018129717"},
	{Name: "BOOSTDEVIATION", Title: "Boost Deviation", Units: "psi", ByteOffset: 0x08, SizeBits: 16, Signed: true, Conversion: "X*0.0018129717"},
	{Name: "BOOSTDEVGRAD", Title: "Boost Deviation Gradient", Units: "psi", ByteOffset: 0x0A, SizeBits: 16, Signed: true, Conversion: "X*0.0018129717"},
	{Name: "ENGINETEMP", Title: "Engine Temp", Units: "°C", ByteOffset: 0x0C, SizeBits: 16, Signed: true, Conversion: "X*0.01"},
	{Name: "GEAR", Title: "Gear", Units: "", ByteOffset: 0x0E, SizeBits: 8, Signed: false, Conversion: "X"},
	{Name: "ETHANOLCONTENT", Title: "Ethanol Content (Active)", Units: "%", ByteOffset: 0x0F, SizeBits: 8, Signed: false, Conversion: "X"},
	{Name: "WGDISTRIBFAC", Title: "Distribution Factor", Units: "-", ByteOffset: 0x10, SizeBits: 16, Signed: false, Conversion: "X/16384"},
	{Name: "TURBOMASSFLOW", Title: "Air Mass Flow (Exhaust, new style)", Units: "g/s", ByteOffset: 0x12, SizeBits: 16, Signed: false, Conversion: "X*0.0390625/3.6"},
	{Name: "TURBOMASSFLOW2", Title: "Air Mass Flow (Exhaust, old style)", Units: "g/s", ByteOffset: 0x14, SizeBits: 16, Signed: false, Conversion: "X*0.03125/3.6"},
	{Name: "CMPRMASSFLOW", Title: "Air Mass Flow (Compressor, S58)", Units: "g/s", ByteOffset: 0x16, SizeBits: 16, Signed: false, Conversion: "X*0.0625/3.6"},
	{Name: "CMPRMASSFLOW2", Title: "Air Mass Flow (Compressor, B58)", Units: "g/s", ByteOffset: 0x18, SizeBits: 16, Signed: false, Conversion: "X*0.03125/3.6"},
	{Name: "BOOSTSETPOINTF", Title: "Boost Setpoint Factor", Units: "-", ByteOffset: 0x1A, SizeBits: 16, Signed: false, Conversion: "X/8192"},
	{Name: "SPEED", Title: "Speed", Units: "km/h", ByteOffset: 0x1C, SizeBits: 16, Signed: false, Conversion: "X"},
	{Name: "AMBIENTTEMP", Title: "Ambient Temp", Units: "°C", ByteOffset: 0x1E, SizeBits: 16, Signed: true, Conversion: "X/10"},
	{Name: "INTAKEAIRTEMP", Title: "Intake Air Temp", Units: "°C", ByteOffset: 0x20, SizeBits: 16, Signed: true, Conversion: "X/10"},
	{Name: "MHDMAFCALC", Title: "MHD+ MAF", Units: "g/s", ByteOffset: 0x22, SizeBits: 16, Signed: false, Conversion: "X*0.03472225"},
}

// decodeRaw extracts this field's raw integer value from body (the
// bytes following MonitorReplyHeader), little-endian per MG1.adx's
// <DEFAULTS lsbfirst="1">.
func (f Field) decodeRaw(body []byte) (int64, error) {
	end := f.ByteOffset + f.SizeBits/8
	if end > len(body) {
		return 0, fmt.Errorf("mhd: field %s: offset %d+%d byte(s) exceeds body length %d", f.Name, f.ByteOffset, f.SizeBits/8, len(body))
	}
	switch f.SizeBits {
	case 8:
		raw := body[f.ByteOffset]
		if f.Signed {
			return int64(int8(raw)), nil
		}
		return int64(raw), nil
	case 16:
		raw := binary.LittleEndian.Uint16(body[f.ByteOffset:end])
		if f.Signed {
			return int64(int16(raw)), nil
		}
		return int64(raw), nil
	default:
		return 0, fmt.Errorf("mhd: field %s: unsupported size %d bits", f.Name, f.SizeBits)
	}
}

// Value is one Field decoded from a monitor reply: its raw integer
// value and that value after applying Field.Conversion.
type Value struct {
	Field Field
	Raw   int64
	Value float64
}

// DecodeMonitorReply decodes a full monitor reply — as returned by
// doip.Client.SendDiagnostic for a MonitorRequest — into one Value per
// Fields entry, in Fields order.
func DecodeMonitorReply(reply []byte) ([]Value, error) {
	if len(reply) != MonitorReplyLength {
		return nil, fmt.Errorf("mhd: monitor reply length = %d, want %d", len(reply), MonitorReplyLength)
	}
	if !bytes.Equal(reply[:len(MonitorReplyHeader)], MonitorReplyHeader) {
		return nil, fmt.Errorf("mhd: monitor reply header = % X, want % X", reply[:len(MonitorReplyHeader)], MonitorReplyHeader)
	}
	body := reply[len(MonitorReplyHeader):]

	values := make([]Value, len(Fields))
	for i, field := range Fields {
		raw, err := field.decodeRaw(body)
		if err != nil {
			return nil, err
		}
		expr, err := expression.Parse(field.Conversion)
		if err != nil {
			return nil, fmt.Errorf("mhd: field %s: parse conversion %q: %w", field.Name, field.Conversion, err)
		}
		value, err := expr.Eval(float64(raw))
		if err != nil {
			return nil, fmt.Errorf("mhd: field %s: evaluate conversion: %w", field.Name, err)
		}
		values[i] = Value{Field: field, Raw: raw, Value: value}
	}
	return values, nil
}

// ByName returns the first Value in values whose Field.Name matches
// name, and whether one was found.
func ByName(values []Value, name string) (Value, bool) {
	for _, v := range values {
		if v.Field.Name == name {
			return v, true
		}
	}
	return Value{}, false
}

// Monitor wraps a doip.Client that has already completed DoIP routing
// activation (doip.Client.Activate) to repeatedly issue MG1.adx's
// monitor exchange and decode the reply.
type Monitor struct {
	client *doip.Client
}

// NewMonitor wraps client for use with Read. client must already be
// past routing activation.
func NewMonitor(client *doip.Client) *Monitor {
	return &Monitor{client: client}
}

// Read sends MonitorRequest and decodes the reply into one Value per
// Fields entry.
func (m *Monitor) Read(timeout time.Duration) ([]Value, error) {
	reply, err := m.client.SendDiagnostic(MonitorRequest, timeout)
	if err != nil {
		return nil, err
	}
	return DecodeMonitorReply(reply)
}
