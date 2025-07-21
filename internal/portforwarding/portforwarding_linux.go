//go:build linux

package portforwarding

import "github.com/wjiec/kertical/internal/portforwarding/nftables"

func init() {
	registerImplement(&realImplement[*nftables.NfTables]{
		Name:      "nftables",
		Available: nftables.Available,
		New: func(name string) (*nftables.NfTables, error) {
			return nftables.NewIPv4(name), nil
		},
		Order: 99,
	})
}
