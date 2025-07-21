package portforwarding

import (
	"slices"

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

// registerImplement adds a new port forwarding implementation to the registry
func registerImplement[T PortForwarding](impl *realImplement[T]) {
	registry = append(registry, &realImplement[PortForwarding]{
		Available: impl.Available,
		New:       func(name string) (PortForwarding, error) { return impl.New(name) },
	})

	slices.SortFunc(registry, func(a, b *realImplement[PortForwarding]) int {
		return b.Order - a.Order
	})
}
