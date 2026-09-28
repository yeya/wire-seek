package mtu

import (
	"net"
	"testing"
	"time"

	"github.com/yeya/wire-seek/output"
)

func TestNewDiscoverer(t *testing.T) {
	tests := []struct {
		name     string
		target   net.IP
		wantIPv6 bool
	}{
		{
			name:     "IPv4 address",
			target:   net.ParseIP("8.8.8.8"),
			wantIPv6: false,
		},
		{
			name:     "IPv6 address",
			target:   net.ParseIP("2001:4860:4860::8888"),
			wantIPv6: true,
		},
		{
			name:     "IPv4-mapped IPv6 treated as IPv4",
			target:   net.ParseIP("::ffff:8.8.8.8").To4(),
			wantIPv6: false,
		},
	}

	log := output.New(output.LevelNormal)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minMTU := MinMTU_IPv4
			if tt.wantIPv6 {
				minMTU = MinMTU_IPv6
			}
			d := NewDiscoverer(tt.target, log, Options{MinMTU: minMTU, MaxMTU: MaxMTU})
			if d.isIPv6 != tt.wantIPv6 {
				t.Errorf("NewDiscoverer().isIPv6 = %v, want %v", d.isIPv6, tt.wantIPv6)
			}
		})
	}
}

func TestPayloadSizeCalculation(t *testing.T) {
	// Test that payload size calculation is correct
	// Payload = MTU - IP header - ICMP header

	tests := []struct {
		name            string
		mtu             int
		isIPv6          bool
		expectedPayload int
	}{
		{
			name:            "1500 MTU IPv4",
			mtu:             1500,
			isIPv6:          false,
			expectedPayload: 1500 - IPv4HeaderSize - ICMPHeaderSize, // 1500 - 20 - 8 = 1472
		},
		{
			name:            "1500 MTU IPv6",
			mtu:             1500,
			isIPv6:          true,
			expectedPayload: 1500 - IPv6HeaderSize - ICMPHeaderSize, // 1500 - 40 - 8 = 1452
		},
		{
			name:            "1280 MTU IPv4",
			mtu:             1280,
			isIPv6:          false,
			expectedPayload: 1280 - IPv4HeaderSize - ICMPHeaderSize, // 1280 - 20 - 8 = 1252
		},
		{
			name:            "1280 MTU IPv6",
			mtu:             1280,
			isIPv6:          true,
			expectedPayload: 1280 - IPv6HeaderSize - ICMPHeaderSize, // 1280 - 40 - 8 = 1232
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := payloadSize(tt.mtu, tt.isIPv6)
			if payload != tt.expectedPayload {
				t.Errorf("payload = %d, want %d", payload, tt.expectedPayload)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// Verify header size constants
	if IPv4HeaderSize != 20 {
		t.Errorf("IPv4HeaderSize = %d, want 20", IPv4HeaderSize)
	}
	if IPv6HeaderSize != 40 {
		t.Errorf("IPv6HeaderSize = %d, want 40", IPv6HeaderSize)
	}
	if ICMPHeaderSize != 8 {
		t.Errorf("ICMPHeaderSize = %d, want 8", ICMPHeaderSize)
	}
	if MinMTU_IPv4 != 576 {
		t.Errorf("MinMTU_IPv4 = %d, want 576", MinMTU_IPv4)
	}
	if MinMTU_IPv6 != 1280 {
		t.Errorf("MinMTU_IPv6 = %d, want 1280", MinMTU_IPv6)
	}
	if MaxMTU != 1500 {
		t.Errorf("MaxMTU = %d, want 1500", MaxMTU)
	}
}

func TestNewDiscovererWithCustomMTU(t *testing.T) {
	tests := []struct {
		name    string
		target  net.IP
		minMTU  int
		maxMTU  int
		wantMin int
		wantMax int
	}{
		{
			name:    "Default IPv4 range",
			target:  net.ParseIP("8.8.8.8"),
			minMTU:  MinMTU_IPv4,
			maxMTU:  MaxMTU,
			wantMin: 576,
			wantMax: 1500,
		},
		{
			name:    "Default IPv6 range",
			target:  net.ParseIP("2001:4860:4860::8888"),
			minMTU:  MinMTU_IPv6,
			maxMTU:  MaxMTU,
			wantMin: 1280,
			wantMax: 1500,
		},
		{
			name:    "Custom WireGuard range",
			target:  net.ParseIP("10.0.0.1"),
			minMTU:  1280,
			maxMTU:  1420,
			wantMin: 1280,
			wantMax: 1420,
		},
		{
			name:    "Jumbo frame range",
			target:  net.ParseIP("192.168.1.1"),
			minMTU:  1500,
			maxMTU:  9000,
			wantMin: 1500,
			wantMax: 9000,
		},
		{
			name:    "Below protocol minimum (allowed with warning)",
			target:  net.ParseIP("8.8.8.8"),
			minMTU:  400,
			maxMTU:  1500,
			wantMin: 400,
			wantMax: 1500,
		},
	}

	log := output.New(output.LevelQuiet)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDiscoverer(tt.target, log, Options{MinMTU: tt.minMTU, MaxMTU: tt.maxMTU})
			if d.minMTU != tt.wantMin {
				t.Errorf("NewDiscoverer().minMTU = %d, want %d", d.minMTU, tt.wantMin)
			}
			if d.maxMTU != tt.wantMax {
				t.Errorf("NewDiscoverer().maxMTU = %d, want %d", d.maxMTU, tt.wantMax)
			}
		})
	}
}

func TestNewDiscovererDefaults(t *testing.T) {
	d := NewDiscoverer(net.ParseIP("8.8.8.8"), output.New(output.LevelQuiet), Options{MinMTU: 576, MaxMTU: 1500})
	if d.timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", d.timeout, DefaultTimeout)
	}
	if d.attempts != DefaultAttempts {
		t.Errorf("attempts = %d, want %d", d.attempts, DefaultAttempts)
	}

	d = NewDiscoverer(net.ParseIP("8.8.8.8"), output.New(output.LevelQuiet),
		Options{MinMTU: 576, MaxMTU: 1500, Timeout: 500 * time.Millisecond, Attempts: 5})
	if d.timeout != 500*time.Millisecond || d.attempts != 5 {
		t.Errorf("custom options not applied: timeout=%v attempts=%d", d.timeout, d.attempts)
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name    string
		minMTU  int
		maxMTU  int
		pathMTU int // largest size the fake network accepts
		want    int
		wantErr bool
	}{
		{"full ethernet", 576, 1500, 1500, 1500, false},
		{"PPPoE", 576, 1500, 1492, 1492, false},
		{"odd value", 576, 1500, 1373, 1373, false},
		{"exactly minimum", 576, 1500, 576, 576, false},
		{"one above minimum", 576, 1500, 577, 577, false},
		{"one below maximum", 576, 1500, 1499, 1499, false},
		{"path above range is capped", 1280, 1420, 1500, 1420, false},
		{"single-value range", 1400, 1400, 1500, 1400, false},
		{"jumbo", 1500, 9000, 9000, 9000, false},
		{"unreachable", 576, 1500, 500, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDiscoverer(net.ParseIP("192.0.2.1"), output.New(output.LevelQuiet),
				Options{MinMTU: tt.minMTU, MaxMTU: tt.maxMTU})
			probes := 0
			d.probe = func(size int) bool {
				probes++
				if size < tt.minMTU || size > tt.maxMTU {
					t.Errorf("probed size %d outside range %d-%d", size, tt.minMTU, tt.maxMTU)
				}
				return size <= tt.pathMTU
			}

			got, err := d.search()
			if (err != nil) != tt.wantErr {
				t.Fatalf("search() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("search() = %d, want %d", got, tt.want)
			}
			// 1 reachability probe + ceil(log2(range)) search probes
			if limit := 2 + bitsLen(tt.maxMTU-tt.minMTU); probes > limit {
				t.Errorf("search() used %d probes, want at most %d", probes, limit)
			}
		})
	}
}

func bitsLen(n int) int {
	b := 0
	for ; n > 0; n >>= 1 {
		b++
	}
	return b
}

func TestProbeOverhead(t *testing.T) {
	if got := ProbeOverhead(false); got != 28 {
		t.Errorf("ProbeOverhead(IPv4) = %d, want 28", got)
	}
	if got := ProbeOverhead(true); got != 48 {
		t.Errorf("ProbeOverhead(IPv6) = %d, want 48", got)
	}
}

func TestAddrIP(t *testing.T) {
	ip := net.ParseIP("192.0.2.1")
	if got := addrIP(&net.IPAddr{IP: ip}); !got.Equal(ip) {
		t.Errorf("addrIP(IPAddr) = %v, want %v", got, ip)
	}
	if got := addrIP(&net.UDPAddr{IP: ip}); !got.Equal(ip) {
		t.Errorf("addrIP(UDPAddr) = %v, want %v", got, ip)
	}
	if got := addrIP(&net.TCPAddr{IP: ip}); got != nil {
		t.Errorf("addrIP(TCPAddr) = %v, want nil", got)
	}
}

func TestStripIPv4Header(t *testing.T) {
	icmpMsg := []byte{0, 0, 0, 0, 0, 1, 0, 1} // echo reply (type 0)
	withHeader := append(append([]byte{0x45}, make([]byte, 19)...), icmpMsg...)

	if got := stripIPv4Header(withHeader, true); string(got) != string(icmpMsg) {
		t.Errorf("stripIPv4Header(with header) = %v, want %v", got, icmpMsg)
	}
	if got := stripIPv4Header(icmpMsg, true); string(got) != string(icmpMsg) {
		t.Errorf("stripIPv4Header(bare ICMP) = %v, want unchanged", got)
	}
	if got := stripIPv4Header(withHeader, false); len(got) != len(withHeader) {
		t.Errorf("stripIPv4Header(non-raw) modified the buffer")
	}
}
