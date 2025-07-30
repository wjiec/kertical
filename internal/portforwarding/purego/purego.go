package purego

import (
	"context"
	"net"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/util/rand"
	netutils "k8s.io/utils/net"
)

// Available returns true that the pure Go port forwarding implementation always available
func Available() (bool, error) {
	return true, nil
}

// PureGo is a port forwarding implementation using pure Go
type PureGo struct {
	sync.Mutex

	name     string
	ctx      context.Context
	cancel   context.CancelFunc
	forwards [1 << 16]forwarder
}

// New creates a new PureGo port forwarder with the specified name
func New(name string) *PureGo {
	ctx, cancel := context.WithCancel(context.Background())
	return &PureGo{name: name, ctx: ctx, cancel: cancel}
}

// forwarder holds cancellation functions for TCP and UDP forwarding on a single port
type forwarder struct {
	tcp context.CancelFunc
	udp context.CancelFunc
}

// AddForwarding creates a new port forwarding for the specified protocol and ports.
func (p *PureGo) AddForwarding(proto netutils.Protocol, from uint16, target []string, to uint16, _ string) error {
	ctx, cancel := context.WithCancel(p.ctx)

	switch proto {
	case netutils.TCP:
		if err := forwardTCP(ctx, from, newTcpDialer(target, to)); err != nil {
			defer cancel()
			return err
		}
		p.forwards[from].tcp = cancel
	case netutils.UDP:
		if err := forwardUDP(ctx, from, newUdpDialer(target, to)); err != nil {
			defer cancel()
			return err
		}
		p.forwards[from].udp = cancel
	default:
		panic("unsupported protocol")
	}

	return nil
}

// RemoveForwarding stops an active port forwarding for the specified protocol and ports.
func (p *PureGo) RemoveForwarding(proto netutils.Protocol, from uint16, _ []string, _ uint16) error {
	switch proto {
	case netutils.TCP:
		if p.forwards[from].tcp != nil {
			p.forwards[from].tcp()
		}
	case netutils.UDP:
		if p.forwards[from].udp != nil {
			p.forwards[from].udp()
		}
	default:
		panic("unsupported protocol")
	}
	return nil
}

// Close terminates all active port forwarding operations and releases resources
func (p *PureGo) Close() error {
	p.cancel()
	return nil
}

// newTcpDialer creates a dialer function that randomly selects one of the
// provided addresses to establish a TCP connection on the specified port.
func newTcpDialer(addresses []string, port uint16) TcpDialer {
	return func() (net.Conn, error) {
		return net.Dial("tcp", addresses[rand.Intn(len(addresses))]+":"+strconv.Itoa(int(port)))
	}
}

// newUdpDialer creates a dialer function that randomly selects one of the
// provided addresses to establish a UDP connection on the specified port.
func newUdpDialer(addresses []string, port uint16) UdpDialer {
	return func() (*net.UDPConn, error) {
		addr, err := net.ResolveUDPAddr("udp", addresses[rand.Intn(len(addresses))]+":"+strconv.Itoa(int(port)))
		if err != nil {
			return nil, err
		}
		return net.DialUDP("udp", nil, addr)
	}
}
