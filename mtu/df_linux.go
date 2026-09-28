//go:build linux

package mtu

import (
	"net"

	"golang.org/x/sys/unix"
)

// setDontFragment disables fragmentation on Linux. IP_PMTUDISC_DO sets the
// DF bit on IPv4; IPV6_PMTUDISC_DO stops the kernel fragmenting locally on
// IPv6 (routers never fragment IPv6).
func setDontFragment(conn net.PacketConn, isIPv6 bool) error {
	if isIPv6 {
		return setsockoptInt(conn, unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IPV6_PMTUDISC_DO)
	}
	return setsockoptInt(conn, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO)
}
