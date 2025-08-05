package transport

import (
	"fmt"

	netutils "k8s.io/utils/net"

	"github.com/wjiec/kertical/internal/kertical"
)

// IsListening checks if a process is already listening on the specified protocol and port.
//
// Returns true if the port is in use, false otherwise.
func IsListening(proto netutils.Protocol, port uint16) (bool, error) {
	switch proto {
	case netutils.TCP:
		return tcpListening(kertical.NodeIP(), port)
	case netutils.UDP:
		return udpListening(kertical.NodeIP(), port)
	default:
		return false, fmt.Errorf("unknown transport protocol: %s", proto)
	}
}
