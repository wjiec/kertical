package transport_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	netutils "k8s.io/utils/net"

	"github.com/wjiec/kertical/internal/portforwarding/transport"
)

const PortNumber = 23323

func TestIsListening(t *testing.T) {
	t.Run("tcp is listening", func(t *testing.T) {
		l, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(PortNumber))
		if assert.NoError(t, err) {
			defer func() { _ = l.Close() }()

			listening, err := transport.IsListening(netutils.TCP, PortNumber)
			if assert.NoError(t, err) {
				assert.True(t, listening)
			}
		}
	})

	t.Run("udp is listening", func(t *testing.T) {
		l, err := net.ListenPacket("udp", ":"+strconv.Itoa(PortNumber))
		if assert.NoError(t, err) {
			defer func() { _ = l.Close() }()
		}

		listening, err := transport.IsListening(netutils.UDP, PortNumber)
		if assert.NoError(t, err) {
			assert.True(t, listening)
		}
	})

	t.Run("tcp not listening", func(t *testing.T) {
		listening, err := transport.IsListening(netutils.TCP, PortNumber)
		if assert.NoError(t, err) {
			assert.False(t, listening)
		}
	})

	t.Run("udp not listening", func(t *testing.T) {
		listening, err := transport.IsListening(netutils.UDP, PortNumber)
		if assert.NoError(t, err) {
			assert.False(t, listening)
		}
	})
}
