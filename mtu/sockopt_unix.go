//go:build linux || darwin || freebsd

package mtu

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// setsockoptInt sets an integer socket option on the connection's descriptor
func setsockoptInt(conn net.PacketConn, level, opt, value int) error {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return errors.New("connection does not expose a file descriptor")
	}
	rawConn, err := sc.SyscallConn()
	if err != nil {
		return err
	}

	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), level, opt, value)
	}); err != nil {
		return err
	}
	return sockErr
}
