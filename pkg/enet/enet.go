// Package enet connects to a BMW ENET diagnostic endpoint (the car's
// gateway directly, or a WiFi-ENET adapter such as MHD's) without the
// caller having to know which protocol or port it speaks.
//
// Connect tries the preferred port first (default 6801, HSFZ), detecting
// HSFZ or ISO 13400 DoIP on it. If that fails it falls back to the other
// well-known ENET ports and then to a TCP sweep of the host, probing
// every open port for either protocol. The first port where an ECU
// actually answers wins.
//
// Why: on 2026-11-27 the MHD WiFi-ENET adapter refused DoIP on
// 192.168.4.1:13400 while MHD's own app was connected to 192.168.4.1:6801
// (see pkg/hsfz). Other adapters and the car's own ENET port may differ,
// hence detection plus a fallback scan instead of a hard-coded pair.
package enet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/freebeamer/core/pkg/doip"
	"github.com/freebeamer/core/pkg/hsfz"
)

// Protocol identifies the diagnostic transport found on a port.
type Protocol string

const (
	// Auto detects the protocol (HSFZ first unless the port is 13400).
	Auto Protocol = ""
	HSFZ Protocol = "hsfz"
	DoIP Protocol = "doip"
)

// ParseProtocol accepts "auto", "hsfz" or "doip".
func ParseProtocol(s string) (Protocol, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return Auto, nil
	case "hsfz":
		return HSFZ, nil
	case "doip":
		return DoIP, nil
	}
	return Auto, fmt.Errorf("enet: unknown protocol %q (want auto, hsfz or doip)", s)
}

// DefaultPort is the port tried first: HSFZ diagnostics.
const DefaultPort = hsfz.DefaultTCPPort

// KnownPorts are tried, in order, before the full sweep.
var KnownPorts = []int{hsfz.DefaultTCPPort, doip.DefaultTCPPort, hsfz.ControlPort}

// DefaultSkipPorts are never probed by the sweep. 5377 is the channel
// MHD's app opens to its own adapter (observed 2026-11-27); it is not
// the car, and sending it our frames could interfere with MHD.
var DefaultSkipPorts = []int{5377}

// testerPresent is UDS TesterPresent (0x3E, sub-function 0x00): the
// harmless request used to check an ECU answers on a candidate port.
var testerPresent = []byte{0x3E, 0x00}

// Options tunes Connect. Zero values select the defaults noted.
type Options struct {
	// Port is tried first. Default DefaultPort (6801).
	Port int
	// Protocol forces a protocol; Auto (default) detects it.
	Protocol Protocol
	// DoIPSource is this client's DoIP logical address. Default 0x0E00.
	DoIPSource uint16
	// HSFZSource is this client's HSFZ tester address. Default 0xF4.
	HSFZSource byte
	// Target overrides the diagnostic target address, if set.
	Target *uint16
	// DialTimeout per TCP connect during probing. Default 2s.
	DialTimeout time.Duration
	// ProbeTimeout for a protocol handshake on an open port. Default 1.5s.
	ProbeTimeout time.Duration
	// NoScan disables the fallback scanner: only Port is tried.
	NoScan bool
	// SweepPorts overrides the full sweep's port list (default 1-65535).
	SweepPorts []int
	// SweepTimeout per port during the sweep. Default 400ms.
	SweepTimeout time.Duration
	// SweepConcurrency is how many ports are swept at once. Default 128.
	SweepConcurrency int
	// SweepDeadline bounds the whole sweep. Default 90s.
	SweepDeadline time.Duration
	// SkipPorts are excluded from the sweep. Default DefaultSkipPorts.
	SkipPorts []int
	// Log, if set, receives one line per step.
	Log func(string)
}

func (o *Options) defaults() {
	if o.Port == 0 {
		o.Port = DefaultPort
	}
	if o.DoIPSource == 0 {
		o.DoIPSource = 0x0E00
	}
	if o.HSFZSource == 0 {
		o.HSFZSource = hsfz.DefaultTesterAddress
	}
	if o.DialTimeout == 0 {
		o.DialTimeout = 2 * time.Second
	}
	if o.ProbeTimeout == 0 {
		o.ProbeTimeout = 1500 * time.Millisecond
	}
	if o.SweepTimeout == 0 {
		o.SweepTimeout = 400 * time.Millisecond
	}
	if o.SweepConcurrency == 0 {
		o.SweepConcurrency = 128
	}
	if o.SweepDeadline == 0 {
		o.SweepDeadline = 90 * time.Second
	}
	if o.SkipPorts == nil {
		o.SkipPorts = DefaultSkipPorts
	}
	if o.Log == nil {
		o.Log = func(string) {}
	}
}

// Link is an open diagnostic connection on whichever protocol and port
// Connect found. It satisfies mhd.Transport.
type Link struct {
	Protocol Protocol
	Port     int
	// Target is the diagnostic target address in use (DoIP: 16-bit
	// logical address; HSFZ: 1-byte ECU address, widened).
	Target uint16
	// Responder is DoIP's routing activation responder (0 for HSFZ).
	Responder uint16
	// Scanned reports whether the fallback scanner was needed.
	Scanned bool

	send  func([]byte, time.Duration) ([]byte, error)
	close func() error
}

// SendDiagnostic sends a UDS request to Target and returns the reply.
func (l *Link) SendDiagnostic(request []byte, timeout time.Duration) ([]byte, error) {
	return l.send(request, timeout)
}

// Close closes the connection.
func (l *Link) Close() error { return l.close() }

// ErrNotFound is returned when no port on the host answered as HSFZ or
// DoIP with a responding ECU.
var ErrNotFound = errors.New("enet: no HSFZ or DoIP diagnostic service found")

// attempt records why one port/protocol did not work, for the final error.
type attempt struct {
	port       int
	protocol   Protocol
	identified bool // the peer spoke this protocol, but the ECU didn't answer
	err        error
}

// Connect finds and opens a diagnostic link on host. See the package
// documentation for the search order.
func Connect(ctx context.Context, host string, opts Options) (*Link, error) {
	opts.defaults()
	var attempts []attempt
	tried := map[int]bool{}

	try := func(port int) *Link {
		tried[port] = true
		link, atts := probePort(ctx, host, port, opts)
		attempts = append(attempts, atts...)
		return link
	}

	opts.Log(fmt.Sprintf("trying %s:%d (%s)", host, opts.Port, protocolLabel(opts.Protocol)))
	if link := try(opts.Port); link != nil {
		return link, nil
	}
	if opts.NoScan {
		return nil, notFound(host, attempts, nil, false)
	}

	for _, port := range KnownPorts {
		if tried[port] || ctx.Err() != nil {
			continue
		}
		opts.Log(fmt.Sprintf("fallback: trying known ENET port %d", port))
		if link := try(port); link != nil {
			link.Scanned = true
			return link, nil
		}
	}

	opts.Log("fallback: sweeping " + host + " for open TCP ports")
	open := sweep(ctx, host, opts, tried)
	opts.Log(fmt.Sprintf("sweep found %d open port(s): %v", len(open), open))
	for _, port := range open {
		if ctx.Err() != nil {
			break
		}
		if link := try(port); link != nil {
			link.Scanned = true
			return link, nil
		}
	}
	return nil, notFound(host, attempts, open, true)
}

func protocolLabel(p Protocol) string {
	if p == Auto {
		return "auto-detect"
	}
	return string(p)
}

// probePort tries each candidate protocol on one port.
func probePort(ctx context.Context, host string, port int, opts Options) (*Link, []attempt) {
	order := []Protocol{HSFZ, DoIP}
	switch {
	case opts.Protocol != Auto:
		order = []Protocol{opts.Protocol}
	case opts.Target != nil && *opts.Target > 0xFF:
		// A 16-bit target can only be a DoIP logical address; probing
		// HSFZ would waste a connection on adapters that allow just one.
		order = []Protocol{DoIP}
	case port == doip.DefaultTCPPort:
		order = []Protocol{DoIP, HSFZ}
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	var atts []attempt
	for _, protocol := range order {
		if ctx.Err() != nil {
			break
		}
		var link *Link
		var identified bool
		var err error
		if protocol == HSFZ {
			link, identified, err = probeHSFZ(address, opts)
		} else {
			link, identified, err = probeDoIP(address, opts)
		}
		if link != nil {
			link.Port = port
			opts.Log(fmt.Sprintf("connected: %s on port %d, target 0x%X", protocol, port, link.Target))
			return link, nil
		}
		opts.Log(fmt.Sprintf("port %d %s: %v", port, protocol, err))
		atts = append(atts, attempt{port, protocol, identified, err})
		if isRefused(err) {
			break // nothing listening: the other protocol won't help
		}
	}
	return nil, atts
}

// probeHSFZ dials, sends TesterPresent and waits for the ECU's reply.
// identified is true when the peer sent at least one valid HSFZ frame.
func probeHSFZ(address string, opts Options) (link *Link, identified bool, err error) {
	if opts.Target != nil && *opts.Target > 0xFF {
		return nil, false, fmt.Errorf("HSFZ target address must fit one byte, got 0x%X", *opts.Target)
	}
	conn, err := net.DialTimeout("tcp", address, opts.DialTimeout)
	if err != nil {
		return nil, false, err
	}
	client := hsfz.NewClient(conn)
	client.SetSourceAddress(opts.HSFZSource)
	if opts.Target != nil {
		client.SetTargetAddress(byte(*opts.Target))
	}
	target := client.TargetAddress()
	if _, err := conn.Write(hsfz.PackFrame(hsfz.ControlDiagnostic, append([]byte{opts.HSFZSource, target}, testerPresent...))); err != nil {
		conn.Close()
		return nil, false, err
	}
	deadline := time.Now().Add(opts.ProbeTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			conn.Close()
			if identified {
				return nil, true, errors.New("HSFZ gateway answered but the ECU did not (ignition off?)")
			}
			return nil, false, errors.New("no HSFZ reply")
		}
		control, body, err := client.ReadFrame(remaining)
		if err != nil {
			conn.Close()
			if identified && errors.Is(err, hsfz.ErrTimeout) {
				return nil, true, errors.New("HSFZ gateway answered but the ECU did not (ignition off?)")
			}
			return nil, identified, err
		}
		if !control.IsKnown() {
			conn.Close()
			return nil, false, fmt.Errorf("not HSFZ (unknown control word 0x%04X)", uint16(control))
		}
		identified = true
		if control.IsError() {
			conn.Close()
			return nil, true, fmt.Errorf("HSFZ %s", control)
		}
		if control == hsfz.ControlDiagnostic && len(body) >= 2 && body[0] == target && body[1] == opts.HSFZSource {
			return &Link{
				Protocol: HSFZ,
				Target:   uint16(target),
				send:     client.SendDiagnostic,
				close:    client.Close,
			}, true, nil
		}
	}
}

// probeDoIP dials and performs routing activation.
func probeDoIP(address string, opts Options) (link *Link, identified bool, err error) {
	client, err := doip.Dial(address, opts.DialTimeout)
	if err != nil {
		return nil, false, err
	}
	responder, err := client.Activate(opts.DoIPSource, doip.ActivationDefault, opts.ProbeTimeout)
	if err != nil {
		client.Close()
		return nil, errors.Is(err, doip.ErrRoutingActivationDenied), err
	}
	target := responder
	if opts.Target != nil {
		target = *opts.Target
		client.SetTargetAddress(target)
	}
	return &Link{
		Protocol:  DoIP,
		Target:    target,
		Responder: responder,
		send:      client.SendDiagnostic,
		close:     client.Close,
	}, true, nil
}

// isRefused reports a TCP reset on connect: nothing listens on the port.
// The string match covers platforms whose errno doesn't map to
// syscall.ECONNREFUSED.
func isRefused(err error) bool {
	return err != nil && (errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "refused"))
}

// sweep returns the host's open TCP ports from opts.SweepPorts (default
// 1-65535), excluding already-tried and skipped ports, in ascending order.
func sweep(ctx context.Context, host string, opts Options, tried map[int]bool) []int {
	skip := map[int]bool{}
	for _, p := range opts.SkipPorts {
		skip[p] = true
	}
	ports := opts.SweepPorts
	if ports == nil {
		ports = make([]int, 0, 65535)
		for p := 1; p <= 65535; p++ {
			ports = append(ports, p)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, opts.SweepDeadline)
	defer cancel()

	jobs := make(chan int)
	var mu sync.Mutex
	var open []int
	var wg sync.WaitGroup
	dialer := net.Dialer{Timeout: opts.SweepTimeout}
	for i := 0; i < opts.SweepConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for port := range jobs {
				conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
				if err != nil {
					continue
				}
				conn.Close()
				mu.Lock()
				open = append(open, port)
				mu.Unlock()
			}
		}()
	}
feed:
	for _, port := range ports {
		if tried[port] || skip[port] {
			continue
		}
		select {
		case jobs <- port:
		case <-ctx.Done():
			opts.Log("sweep stopped early: " + ctx.Err().Error())
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	sort.Ints(open)
	return open
}

func notFound(host string, attempts []attempt, open []int, swept bool) error {
	var b strings.Builder
	for _, a := range attempts {
		if a.identified {
			fmt.Fprintf(&b, "; port %d speaks %s but: %v", a.port, a.protocol, a.err)
		}
	}
	if len(attempts) > 0 {
		a := attempts[0]
		fmt.Fprintf(&b, "; port %d (%s): %v", a.port, a.protocol, a.err)
	}
	if swept {
		fmt.Fprintf(&b, "; sweep found open port(s) %v, none answered as HSFZ or DoIP", open)
	} else {
		b.WriteString("; fallback scan disabled")
	}
	return fmt.Errorf("%w on %s%s", ErrNotFound, host, b.String())
}
