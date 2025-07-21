package purego

import (
	"context"
	"fmt"
	"sync"

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
func (p *PureGo) AddForwarding(proto netutils.Protocol, from uint16, target string, to uint16, comment string) error {
	ctx, cancel := context.WithCancel(p.ctx)
	forwardedTo := fmt.Sprintf("%s:%d", target, to)

	switch proto {
	case netutils.TCP:
		if err := forwardTCP(ctx, from, forwardedTo); err != nil {
			defer cancel()
			return err
		}
		p.forwards[from].tcp = cancel
	case netutils.UDP:
		if err := forwardUDP(ctx, from, forwardedTo); err != nil {
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
func (p *PureGo) RemoveForwarding(proto netutils.Protocol, from uint16, _ string, _ uint16) error {
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
