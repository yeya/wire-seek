# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Wire-seek is a WireGuard MTU optimization tool that discovers the optimal MTU for WireGuard tunnels using ICMP Path MTU Discovery with the Don't Fragment bit.

## Build and Development Commands

```bash
# Build the project
go build ./...

# Run tests
go test ./...

# Run a single test
go test -run TestName ./path/to/package

# Run tests with coverage
go test -cover ./...

# Format code
go fmt ./...

# Vet code for issues
go vet ./...
```

## Usage

```bash
# Basic usage (testing path to WireGuard endpoint, outside tunnel)
wire-seek <target-host>

# With options
wire-seek -v -target example.com   # Verbose mode
wire-seek example.com -q           # Flags may follow the host
wire-seek -6 example.com           # Force IPv6
wire-seek -tunnel 8.8.8.8          # Testing THROUGH an existing tunnel
```

**Important**: Use `-tunnel` flag when testing through an existing WireGuard tunnel to avoid double-subtracting overhead.

## Architecture

- `main.go` - CLI entry point, argument parsing, and result display
- `mtu/` - MTU discovery package
  - `discover.go` - Binary search PMTUD algorithm using ICMP echo with fragmentation disabled
    - One ICMP socket is opened per run; replies are matched by source, echo ID (raw sockets) and sequence
    - Each size is retried (`-attempts`) on timeout; send errors such as EMSGSIZE are definitive
    - `Discoverer.probe` is swappable so `search()` can be unit-tested without a network
    - IPv4 range: 576-1500 bytes (RFC 791 minimum)
    - IPv6 range: 1280-1500 bytes (RFC 8200 minimum)
    - Verbose mode (`-v`) shows detailed socket and error diagnostics
  - `wireguard.go` - WireGuard overhead calculations (60 bytes IPv4, 80 bytes IPv6)
  - `df_linux.go`, `df_bsd.go`, `df_windows.go`, `df_other.go` - Platform-specific `setDontFragment` (IPv4 and IPv6)
  - `sockopt_unix.go` - Shared setsockopt helper for Linux/macOS/FreeBSD
- `output/` - Leveled output (quiet / normal / verbose)

## Key Implementation Details

- **Linux**: `IP_MTU_DISCOVER`/`IPV6_MTU_DISCOVER` set to `*_PMTUDISC_DO`
- **macOS (11+) / FreeBSD**: `IP_DONTFRAG` / `IPV6_DONTFRAG`
- **Windows**: `IP_DONTFRAGMENT` / `IPV6_DONTFRAG`
- **Unprivileged ICMP**: Falls back to datagram ICMP sockets (Linux 3.0+ with `net.ipv4.ping_group_range`, macOS) when raw sockets are unavailable
- **Go version**: `go.mod` declares the minimum (1.26); CI and releases build with the latest stable Go. Do not add a `toolchain` line newer than the Go version golangci-lint is built with, or the lint job fails.

## Git Commit Guidelines

- Do NOT include `Co-Authored-By` lines in commit messages
