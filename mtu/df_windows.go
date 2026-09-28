//go:build windows

package mtu

import (
	"errors"
	"net"
	"syscall"
)

const (
	// IP_DONTFRAGMENT is the Windows socket option for Don't Fragment (ws2ipdef.h)
	IP_DONTFRAGMENT = 14
	// IPV6_DONTFRAG is the Windows socket option to disable IPv6 fragmentation (ws2ipdef.h)
	IPV6_DONTFRAG = 14
)

// setDontFragment disables fragmentation on Windows
func setDontFragment(conn net.PacketConn, isIPv6 bool) error {
	level, opt := syscall.IPPROTO_IP, IP_DONTFRAGMENT
	if isIPv6 {
		level, opt = syscall.IPPROTO_IPV6, IPV6_DONTFRAG
	}

	sc, ok := conn.(syscall.Conn)
	if !ok {
		return errors.New("connection does not expose a socket handle")
	}
	rawConn, err := sc.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(syscall.Handle(fd), level, opt, 1)
	}); err != nil {
		return err
	}
	return sockErr
}
