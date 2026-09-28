//go:build darwin || freebsd

package mtu

import (
	"net"

	"golang.org/x/sys/unix"
)

// setDontFragment disables fragmentation on macOS (11+) and FreeBSD using
// the IP_DONTFRAG / IPV6_DONTFRAG socket options.
func setDontFragment(conn net.PacketConn, isIPv6 bool) error {
	if isIPv6 {
		return setsockoptInt(conn, unix.IPPROTO_IPV6, unix.IPV6_DONTFRAG, 1)
	}
	return setsockoptInt(conn, unix.IPPROTO_IP, unix.IP_DONTFRAG, 1)
}
