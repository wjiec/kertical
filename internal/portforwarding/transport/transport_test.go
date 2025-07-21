package transport_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	netutils "k8s.io/utils/net"

	"github.com/wjiec/kertical/internal/portforwarding/transport"
)

func TestIsListening(t *testing.T) {
	t.Run("tcp is listening", func(t *testing.T) {
		l, err := net.Listen("tcp", ":0")
		if assert.NoError(t, err) {
			defer func() { _ = l.Close() }()
		}

		listening, err := transport.IsListening(netutils.TCP, uint16(l.Addr().(*net.TCPAddr).Port))
		if assert.NoError(t, err) {
			assert.True(t, listening)
		}
	})

	t.Run("udp is listening", func(t *testing.T) {
		l, err := net.ListenPacket("udp", ":53535")
		if assert.NoError(t, err) {
			defer func() { _ = l.Close() }()
		}

		listening, err := transport.IsListening(netutils.UDP, uint16(l.LocalAddr().(*net.UDPAddr).Port))
		if assert.NoError(t, err) {
			assert.True(t, listening)
		}
	})

	t.Run("tcp not listening", func(t *testing.T) {
		listening, err := transport.IsListening(netutils.TCP, 55555)
		if assert.NoError(t, err) {
			assert.False(t, listening)
		}
	})

	t.Run("udp not listening", func(t *testing.T) {
		listening, err := transport.IsListening(netutils.UDP, 55555)
		if assert.NoError(t, err) {
			assert.False(t, listening)
		}
	})
}
