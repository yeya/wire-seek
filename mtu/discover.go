package mtu

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/yeya/wire-seek/output"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// MinMTU_IPv4 is the minimum MTU for IPv4 networks (RFC 791)
	MinMTU_IPv4 = 576
	// MinMTU_IPv6 is the minimum MTU for IPv6 networks (RFC 8200)
	MinMTU_IPv6 = 1280
	// MaxMTU is the standard Ethernet MTU
	MaxMTU = 1500
	// MaxPacketSize is the largest IP packet size that can be expressed
	MaxPacketSize = 65535
	// ICMPHeaderSize is the size of ICMP header
	ICMPHeaderSize = 8
	// IPv4HeaderSize is the size of IPv4 header
	IPv4HeaderSize = 20
	// IPv6HeaderSize is the size of IPv6 header
	IPv6HeaderSize = 40

	// DefaultTimeout is the default time to wait for each echo reply
	DefaultTimeout = 2 * time.Second
	// DefaultAttempts is the default number of probes sent per MTU size
	DefaultAttempts = 2

	protoICMP   = 1
	protoICMPv6 = 58
)

// Options configures a Discoverer
type Options struct {
	MinMTU   int
	MaxMTU   int
	Timeout  time.Duration // per-probe reply timeout (0 = DefaultTimeout)
	Attempts int           // probes per size before declaring it too large (0 = DefaultAttempts)
}

// Discoverer handles MTU discovery operations
type Discoverer struct {
	target   net.IP
	isIPv6   bool
	log      *output.Logger
	minMTU   int
	maxMTU   int
	timeout  time.Duration
	attempts int

	conn  *icmp.PacketConn
	raw   bool // true for raw sockets, false for unprivileged datagram ICMP
	seq   int
	probe func(size int) bool // replaceable in tests
}

// NewDiscoverer creates a new MTU discoverer for the given target
func NewDiscoverer(target net.IP, log *output.Logger, opts Options) *Discoverer {
	d := &Discoverer{
		target:   target,
		isIPv6:   target.To4() == nil,
		log:      log,
		minMTU:   opts.MinMTU,
		maxMTU:   opts.MaxMTU,
		timeout:  opts.Timeout,
		attempts: opts.Attempts,
	}
	if d.timeout <= 0 {
		d.timeout = DefaultTimeout
	}
	if d.attempts <= 0 {
		d.attempts = DefaultAttempts
	}
	d.probe = d.canSendSize
	return d
}

// FindPathMTU uses binary search to find the path MTU to the target
func (d *Discoverer) FindPathMTU() (int, error) {
	if err := d.open(); err != nil {
		return 0, err
	}
	defer d.close()
	return d.search()
}

// search runs the binary search over [minMTU, maxMTU] using d.probe
func (d *Discoverer) search() (int, error) {
	low, high := d.minMTU, d.maxMTU

	d.log.Info("Discovering path MTU (range: %d-%d)...\n", low, high)

	// First, verify we can reach the target at all
	if !d.probe(low) {
		return 0, fmt.Errorf("no reply from target at minimum MTU (%d); "+
			"the host may be down or blocking ICMP echo", low)
	}

	// Invariant: low is known to work, sizes above high are known to fail
	for low < high {
		mid := low + (high-low+1)/2

		d.log.Debug("  Testing MTU %d... ", mid)

		if d.probe(mid) {
			d.log.Debug("OK\n")
			low = mid
		} else {
			d.log.Debug("Too large\n")
			high = mid - 1
		}
	}

	return low, nil
}

// open creates the ICMP socket used for all probes, preferring raw sockets
// and falling back to unprivileged datagram ICMP (Linux 3.0+, macOS).
func (d *Discoverer) open() error {
	rawNet, dgramNet := "ip4:icmp", "udp4"
	if d.isIPv6 {
		rawNet, dgramNet = "ip6:ipv6-icmp", "udp6"
	}

	conn, rawErr := icmp.ListenPacket(rawNet, "")
	if rawErr == nil {
		d.raw = true
		d.log.Debug("Using raw ICMP socket (%s)\n", rawNet)
	} else {
		d.log.Debug("Raw ICMP socket unavailable: %v\n", rawErr)
		var dgramErr error
		conn, dgramErr = icmp.ListenPacket(dgramNet, "")
		if dgramErr != nil {
			return fmt.Errorf("cannot open ICMP socket (raw: %v; unprivileged: %v); "+
				"run as root/administrator, or on Linux allow unprivileged ping via "+
				"sysctl net.ipv4.ping_group_range", rawErr, dgramErr)
		}
		d.log.Debug("Using unprivileged ICMP (%s)\n", dgramNet)
	}

	if err := setDontFragment(underlyingConn(conn, d.isIPv6), d.isIPv6); err != nil {
		d.log.Debug("Warning: failed to set Don't Fragment: %v\n", err)
	}

	d.conn = conn
	return nil
}

func (d *Discoverer) close() {
	if d.conn != nil {
		_ = d.conn.Close()
		d.conn = nil
	}
}

// underlyingConn returns the net.PacketConn backing an icmp.PacketConn
func underlyingConn(c *icmp.PacketConn, isIPv6 bool) net.PacketConn {
	if isIPv6 {
		if p := c.IPv6PacketConn(); p != nil {
			return p.PacketConn
		}
		return nil
	}
	if p := c.IPv4PacketConn(); p != nil {
		return p.PacketConn
	}
	return nil
}

// ProbeOverhead returns the IP + ICMP header size of an echo probe, which is
// the smallest MTU that can be tested
func ProbeOverhead(isIPv6 bool) int {
	if isIPv6 {
		return IPv6HeaderSize + ICMPHeaderSize
	}
	return IPv4HeaderSize + ICMPHeaderSize
}

// payloadSize returns the ICMP echo payload size for a packet of the given MTU
func payloadSize(mtu int, isIPv6 bool) int {
	return mtu - ProbeOverhead(isIPv6)
}

// canSendSize tests if a packet of the given size can reach the target
// without fragmentation, retrying on timeouts to tolerate packet loss
func (d *Discoverer) canSendSize(size int) bool {
	payload := payloadSize(size, d.isIPv6)
	if payload < 0 {
		return false
	}

	for attempt := 1; attempt <= d.attempts; attempt++ {
		ok, retry := d.pingWithDF(payload)
		if ok {
			return true
		}
		if !retry {
			return false
		}
		if attempt < d.attempts {
			d.log.Debug("(retry) ")
		}
	}
	return false
}

// pingWithDF sends a single ICMP echo request with fragmentation disabled
// and waits for the matching reply. retry reports whether the failure was a
// timeout (possibly packet loss) rather than a definitive local error.
func (d *Discoverer) pingWithDF(payloadSize int) (ok, retry bool) {
	var (
		proto               = protoICMP
		reqType   icmp.Type = ipv4.ICMPTypeEcho
		replyType icmp.Type = ipv4.ICMPTypeEchoReply
	)
	if d.isIPv6 {
		proto = protoICMPv6
		reqType = ipv6.ICMPTypeEchoRequest
		replyType = ipv6.ICMPTypeEchoReply
	}

	d.seq = (d.seq + 1) & 0xffff
	id := os.Getpid() & 0xffff

	msg := icmp.Message{
		Type: reqType,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  d.seq,
			Data: make([]byte, payloadSize),
		},
	}

	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		d.log.Debug("  Error marshaling ICMP message: %v\n", err)
		return false, false
	}

	if err := d.conn.SetDeadline(time.Now().Add(d.timeout)); err != nil {
		d.log.Debug("  Error setting deadline: %v\n", err)
		return false, false
	}

	var dst net.Addr = &net.UDPAddr{IP: d.target}
	if d.raw {
		dst = &net.IPAddr{IP: d.target}
	}

	// A send error (typically EMSGSIZE when the kernel already knows the
	// path MTU is smaller) is definitive, so it is not retried.
	if _, err := d.conn.WriteTo(msgBytes, dst); err != nil {
		d.log.Debug("  Error sending packet: %v\n", err)
		return false, false
	}

	// Read until the matching reply arrives or the deadline expires. Raw
	// sockets receive all ICMP traffic, so unrelated packets are skipped.
	reply := make([]byte, max(payloadSize+ProbeOverhead(d.isIPv6)+64, 1500))
	for {
		n, peer, err := d.conn.ReadFrom(reply)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return false, true
			}
			d.log.Debug("  Error reading reply: %v\n", err)
			return false, false
		}

		if !addrIP(peer).Equal(d.target) {
			continue
		}

		rm, err := icmp.ParseMessage(proto, stripIPv4Header(reply[:n], d.raw && !d.isIPv6))
		if err != nil {
			d.log.Debug("  Error parsing reply: %v\n", err)
			continue
		}
		if rm.Type != replyType {
			// Our own request is looped back when probing a local address
			if rm.Type != reqType {
				d.log.Debug("  Ignoring ICMP type %v from target\n", rm.Type)
			}
			continue
		}
		echo, isEcho := rm.Body.(*icmp.Echo)
		if !isEcho || echo.Seq != d.seq {
			continue
		}
		// The kernel rewrites the ID of unprivileged ICMP sockets, and
		// already filters replies for them, so only check it for raw sockets
		if d.raw && echo.ID != id {
			continue
		}
		return true, false
	}
}

// stripIPv4Header removes a leading IPv4 header, which some platforms (e.g.
// Windows) include in data read from raw IPv4 sockets
func stripIPv4Header(b []byte, raw bool) []byte {
	if !raw || len(b) < IPv4HeaderSize || b[0]>>4 != 4 {
		return b
	}
	if hl := int(b[0]&0x0f) * 4; hl >= IPv4HeaderSize && hl <= len(b) {
		return b[hl:]
	}
	return b
}

// addrIP extracts the IP address from a net.Addr returned by ReadFrom
func addrIP(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPAddr:
		return v.IP
	case *net.UDPAddr:
		return v.IP
	}
	return nil
}
