package portforwarding

import (
	"slices"

	"github.com/pkg/errors"

	"github.com/wjiec/kertical/internal/portforwarding/purego"
)

func init() {
	// registers the built-in pure Go port forwarding implementation
	registerImplement(&realImplement[*purego.PureGo]{
		Name:      "pure-go",
		Available: purego.Available,
		New: func(name string) (*purego.PureGo, error) {
			return purego.New(name), nil
		},
	})
}

// realImplement is a generic wrapper for port forwarding implementations
type realImplement[T PortForwarding] struct {
	Name      string
	Available func() (bool, error)
	New       func(string) (T, error)
	Order     int
}

// registry holds all registered port forwarding implementations
var registry []*realImplement[PortForwarding]

// findPortForwardingImpl locates a suitable PortForwarding implementation from the registry.
//
// If a 'preference' is specified, it first tries to return an available implementation
// matching the preference name. If no preferred implementation is found or is unavailable,
// it selects the first available implementation in order of priority.
func findPortForwardingImpl(preference, name string) (PortForwarding, error) {
	if len(preference) != 0 {
		for _, impl := range registry {
			if impl.Name == preference {
				if available, err := impl.Available(); err != nil {
					return nil, errors.Wrapf(err, "failed to check %q is available", impl.Name)
				} else if available {
					return impl.New(name)
				}
			}
		}
	}

	for _, impl := range registry {
		if available, err := impl.Available(); err != nil {
			return nil, errors.Wrapf(err, "failed to check %q is available", impl.Name)
		} else if available {
			return impl.New(name)
		}
	}
	return nil, errors.Errorf("no underlying port forwarding is available")
}

// registerImplement adds a new port forwarding implementation to the registry
func registerImplement[T PortForwarding](impl *realImplement[T]) {
	registry = append(registry, &realImplement[PortForwarding]{
		Name:      impl.Name,
		Available: impl.Available,
		New:       func(name string) (PortForwarding, error) { return impl.New(name) },
		Order:     impl.Order,
	})

	slices.SortFunc(registry, func(a, b *realImplement[PortForwarding]) int {
		return b.Order - a.Order
	})
}
