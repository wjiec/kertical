//go:build linux

package transport

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// tcpListening attempts to connect to the specified network address
// to determine if the tcp port is in use.
func tcpListening(_ string, port uint16) (bool, error) {
	if listening, err := isListening("/proc/net/tcp", port, unix.BPF_TCP_LISTEN); err != nil || listening {
		return listening, err
	}
	return isListening("/proc/net/tcp6", port, unix.BPF_TCP_LISTEN)
}

// udpListening checks if a UDP port is in use (listening) on the local machine.
// It attempts to bind to the port to determine its status.
func udpListening(_ string, port uint16) (bool, error) {
	const UdpConn = 0x7

	if listening, err := isListening("/proc/net/udp", port, UdpConn); err != nil || listening {
		return listening, err
	}
	return isListening("/proc/net/udp6", port, UdpConn)
}

// isListening checks if a specific port is in the listening state by
// reading from a proc file and parsing its contents.
func isListening(filename string, port uint16, mask uint8) (bool, error) {
	fp, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer func() { _ = fp.Close() }()

	return parseAndCheck(fp, uint64(port), mask)
}

// parseAndCheck checks if a specific port is in the listening state by parsing the provided
// network connection data (typically from /proc/net/tcp or /proc/net/udp).
func parseAndCheck(r io.Reader, port uint64, mask uint8) (bool, error) {
	for sc := bufio.NewScanner(r); sc.Scan(); {
		fields := strings.Fields(sc.Text())
		if localAddr := strings.Split(fields[1], ":"); len(localAddr) == 2 {
			if localPort, err := strconv.ParseUint(localAddr[1], 16, 64); err == nil {
				if state, err := strconv.ParseUint(fields[3], 16, 64); err == nil {
					if localPort == port && uint8(state)&mask == mask {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}
