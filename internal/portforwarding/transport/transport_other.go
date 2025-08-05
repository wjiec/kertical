//go:build !linux

package transport

import (
	"errors"
	"net"
	"os"
	"strconv"
	"syscall"
)

// tcpListening attempts to connect to the specified network address
// to determine if the tcp port is in use.
func tcpListening(addr string, port uint16) (bool, error) {
	conn, err := net.Dial("tcp", addr+":"+strconv.Itoa(int(port)))
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
func udpListening(addr string, port uint16) (bool, error) {
	l, err := net.ListenPacket("udp", addr+":"+strconv.Itoa(int(port)))
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
