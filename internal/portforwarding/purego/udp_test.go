package purego

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_forwardUDP(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	go func() {
		assert.NoError(t, forwardUDP(ctx, 54321, "114.114.114.114:53"))
	}()

	time.Sleep(100 * time.Millisecond) // waiting udp forwarder start
	lAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:54321")
	if assert.NoError(t, err) {
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.DialUDP("udp", nil, lAddr)
			},
		}

		answer, err := r.LookupIP(ctx, "ip4", "github.com")
		if assert.NoError(t, err) {
			assert.NotEmpty(t, answer)
		}
	}
}
