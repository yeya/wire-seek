# Wire-Seek

[![CI](https://github.com/yeya/wire-seek/actions/workflows/ci.yml/badge.svg)](https://github.com/yeya/wire-seek/actions/workflows/ci.yml)
[![Release](https://github.com/yeya/wire-seek/actions/workflows/release.yml/badge.svg)](https://github.com/yeya/wire-seek/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/yeya/wire-seek)](https://goreportcard.com/report/github.com/yeya/wire-seek)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/yeya/wire-seek)](https://go.dev/)

A WireGuard MTU optimization tool that discovers the optimal MTU for WireGuard tunnels using ICMP Path MTU Discovery.

## Why?

If your WireGuard MTU is too high, packets get fragmented (slow). If it's too low, you're wasting bandwidth on overhead. Wire-Seek finds the sweet spot automatically.

## How It Works

1. **Path MTU Discovery** - Sends ICMP Echo packets with fragmentation disabled (DF bit on IPv4, no local fragmentation on IPv6)
2. **Binary Search** - Efficiently finds the optimal MTU in ~10 probes instead of 900+
3. **Loss Tolerance** - Retries a size that gets no reply before treating it as too large, so a single dropped packet doesn't skew the result
4. **WireGuard Calculation** - Subtracts the correct overhead for your tunnel:
   - IPv4: 60 bytes (20 IP + 8 UDP + 32 WireGuard)
   - IPv6: 80 bytes (40 IP + 8 UDP + 32 WireGuard)

## Important: Testing Inside vs Outside a Tunnel

> **You MUST use the `--tunnel` flag when testing through an existing WireGuard tunnel.**

| Scenario | Command | What It Does |
|----------|---------|--------------|
| **Setting up a new tunnel** | `wire-seek <endpoint-ip>` | Tests path to endpoint, subtracts WireGuard overhead |
| **Verifying existing tunnel** | `wire-seek --tunnel 8.8.8.8` | Tests through tunnel, reports effective MTU directly |

**Why does this matter?**

- **Without `--tunnel`**: Wire-Seek subtracts WireGuard overhead (60/80 bytes) from the path MTU
- **With `--tunnel`**: The overhead is already applied by your tunnel, so we report the path MTU as-is

If you test through a tunnel WITHOUT the `--tunnel` flag, you'll get a value ~60 bytes too low (double subtraction).

## Installation

### From Releases

Download the latest binary from [Releases](https://github.com/yeya/wire-seek/releases).

```bash
# Linux (amd64)
curl -LO https://github.com/yeya/wire-seek/releases/latest/download/wire-seek-linux-amd64
chmod +x wire-seek-linux-amd64
sudo mv wire-seek-linux-amd64 /usr/local/bin/wire-seek
```

### From Source

```bash
go install github.com/yeya/wire-seek@latest
```

Or clone and build:

```bash
git clone https://github.com/yeya/wire-seek.git
cd wire-seek
go build -o wire-seek .
```

## Usage

```bash
# Basic usage (root is needed for raw ICMP sockets; see "Running without root")
sudo wire-seek <target-host>

# Examples
sudo wire-seek 10.0.0.1              # WireGuard peer IP (outside tunnel)
sudo wire-seek vpn.example.com       # Hostname
sudo wire-seek -v 8.8.8.8            # Verbose mode
sudo wire-seek -6 2001:db8::1        # Force IPv6
sudo wire-seek -q vpn.example.com    # Quiet mode (for scripting)
sudo wire-seek -max-mtu 1420 10.0.0.1    # Limit to WireGuard-safe range
sudo wire-seek -max-mtu 9000 10.0.0.1    # Jumbo frame network
sudo wire-seek 10.0.0.1 -v               # Flags may also follow the host
sudo wire-seek -timeout 1s -attempts 3 vpn.example.com  # Lossy/slow links

# Testing through an existing tunnel (IMPORTANT: use --tunnel flag!)
sudo wire-seek -tunnel 8.8.8.8       # Verify tunnel MTU is working
```

### Options

| Flag | Description |
|------|-------------|
| `-target` | Target host or IP address |
| `-6` | Use IPv6 (fails if the host has no IPv6 address) |
| `-v` | Verbose output (shows binary search progress) |
| `-q` | Quiet output (prints only the MTU value, for scripting) |
| `-tunnel` | Testing through an existing tunnel (skips WireGuard overhead calculation) |
| `-min-mtu` | Minimum MTU to test (default: 576 for IPv4, 1280 for IPv6) |
| `-max-mtu` | Maximum MTU to test (default: 1500, increase for jumbo frames) |
| `-timeout` | Time to wait for each echo reply (default: 2s) |
| `-attempts` | Probes per MTU size before treating it as too large (default: 2) |

Exit codes: `0` success, `1` resolution/discovery failure, `2` invalid usage.

### Running without root

If raw ICMP sockets are unavailable, Wire-Seek falls back to unprivileged ICMP
sockets (Linux and macOS). On Linux, allow them for your group with:

```bash
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

### Example Output

```
Wire-Seek: WireGuard MTU Optimizer
Target: 10.0.0.1 (10.0.0.1)
Protocol: IPv4

Discovering path MTU (range: 576-1500)...

Results:
  Path MTU: 1500 bytes
  WireGuard MTU: 1440 bytes

Add to your WireGuard config:
  MTU = 1440
```

## Applying the MTU

Add the discovered MTU to your WireGuard configuration:

```ini
[Interface]
PrivateKey = ...
Address = 10.0.0.2/24
MTU = 1440  # <-- Add this line
```

Then restart the interface:

```bash
sudo wg-quick down wg0
sudo wg-quick up wg0
```

## Supported Platforms

| Platform | Architecture | Status |
|----------|--------------|--------|
| Linux | amd64, arm64 | ✅ Full support |
| macOS (11+) | amd64, arm64 | ✅ Full support |
| FreeBSD | amd64 | ✅ Full support |
| Windows | amd64 | ✅ Full support |

## License

MIT License - see [LICENSE](LICENSE) for details.
