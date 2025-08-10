package portforwarding

import (
	"os"
	"sync"

	"github.com/pkg/errors"
	netutils "k8s.io/utils/net"
	"k8s.io/utils/set"

	"github.com/wjiec/kertical/internal/portforwarding/transport"
)

var (
	ErrPortAlreadyInuse     = errors.New("port already in use")
	ErrPortNotForwarded     = errors.New("port is not forwarded yet")
	ErrPortAlreadyForwarded = errors.New("port already forwarded")
)

// PortForwarding defines the operations for managing port forwarding rules.
type PortForwarding interface {
	// AddForwarding creates a new port forwarding rule to redirect traffic received
	// on the specified protocol and port to the target IP address and port.
	AddForwarding(proto netutils.Protocol, from uint16, target []string, to uint16, comment string) error

	// RemoveForwarding deletes an existing port forwarding rule for the specified protocol and port.
	RemoveForwarding(proto netutils.Protocol, from uint16, target []string, to uint16) error

	// Close removes all port forwarding rule created by this instance.
	Close() error
}

// New creates a new PortForwarding implementation with safety guards.
func New(name string) (PortForwarding, error) {
	underlying, err := findPortForwardingImpl(os.Getenv("PORTFORWARDING_MODE"), name)
	if err != nil {
		return nil, err
	}

	return &guardPortForwarding{
		underlying: underlying,
		listens: map[netutils.Protocol]set.Set[uint16]{
			netutils.TCP: set.New[uint16](),
			netutils.UDP: set.New[uint16](),
		},
	}, nil
}

// guardPortForwarding wraps a PortForwarding implementation with additional safety checks.
//
// It prevents conflicts by tracking active port forwards and checking if ports are already in use.
type guardPortForwarding struct {
	underlying PortForwarding

	mu      sync.Mutex
	listens map[netutils.Protocol]set.Set[uint16]
}

// AddForwarding creates a new port forwarding rule after performing safety checks:
func (gpf *guardPortForwarding) AddForwarding(proto netutils.Protocol, from uint16, target []string, to uint16, comment string) error {
	gpf.mu.Lock()
	defer gpf.mu.Unlock()
	if gpf.listens[proto].Has(from) {
		return ErrPortAlreadyForwarded
	}

	// We check if any program is listening on this port on the host machine
	// after confirming there are no existing forwarding configurations
	if listening, err := transport.IsListening(proto, from); err != nil {
		return errors.Wrap(err, "failed to check ip port in the machine is listening")
	} else if listening {
		return ErrPortAlreadyInuse
	}

	if err := gpf.underlying.AddForwarding(proto, from, target, to, comment); err != nil {
		return err
	}

	gpf.listens[proto].Insert(from)
	return nil
}

// RemoveForwarding removes a port forwarding rule after verifying it exists.
func (gpf *guardPortForwarding) RemoveForwarding(proto netutils.Protocol, from uint16, target []string, to uint16) error {
	gpf.mu.Lock()
	defer gpf.mu.Unlock()
	if !gpf.listens[proto].Has(from) {
		return ErrPortNotForwarded
	}

	if err := gpf.underlying.RemoveForwarding(proto, from, target, to); err != nil {
		return err
	}

	gpf.listens[proto].Delete(from)
	return nil
}

// Close cleans up resources by delegating to the underlying implementation.
func (gpf *guardPortForwarding) Close() error {
	return gpf.underlying.Close()
}
