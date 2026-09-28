//go:build !linux && !darwin && !freebsd && !windows

package mtu

import (
	"errors"
	"net"
)

// setDontFragment is not supported on this platform, so probes may be
// fragmented and the discovered MTU may be too high.
func setDontFragment(conn net.PacketConn, isIPv6 bool) error {
	return errors.New("setting Don't Fragment is not supported on this platform")
}
