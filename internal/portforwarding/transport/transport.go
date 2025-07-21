package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"

	netutils "k8s.io/utils/net"
)

// IsListening checks if a process is already listening on the specified protocol and port.
//
// Returns true if the port is in use, false otherwise.
func IsListening(proto netutils.Protocol, port uint16) (bool, error) {
	switch proto {
	case netutils.TCP:
		return tcpListening(":" + strconv.Itoa(int(port)))
	case netutils.UDP:
		return udpListening(":" + strconv.Itoa(int(port)))
	default:
		return false, fmt.Errorf("unknown transport protocol: %s", proto)
	}
}

// tcpListening attempts to connect to the specified network address
// to determine if the tcp port is in use.
func tcpListening(address string) (bool, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		var syscallErr *os.SyscallError
		if errors.As(err, &syscallErr) && errors.Is(syscallErr.Err, syscall.ECONNREFUSED) {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = conn.Close() }()

	return true, nil
}

// udpListening checks if a UDP port is in use (listening) on the local machine.
// It attempts to bind to the port to determine its status.
func udpListening(address string) (bool, error) {
	l, err := net.ListenPacket("udp", address)
	if err != nil {
		var syscallErr *os.SyscallError
		if errors.As(err, &syscallErr) && errors.Is(syscallErr.Err, syscall.EADDRINUSE) {
			return true, nil
		}
		return false, err
	}
	defer func() { _ = l.Close() }()

	return false, nil
}
