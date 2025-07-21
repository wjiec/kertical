//go:build linux

package nftables

import (
	"encoding/binary"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/google/nftables"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/sysctl"
	"github.com/wjiec/kertical/internal/portforwarding/transport"
)

// Available checks if the system is properly configured for port forwarding.
//
// It verifies that IPv4 forwarding is enabled in the kernel, which is a
// prerequisite for packets to be forwarded between network interfaces.
func Available() (bool, error) {
	forwarding, err := sysctl.NetIPv4Forward()
	if err != nil {
		return false, err
	}

	return forwarding == 1, nil
}

// NfTables implements the PortForwarding interface using nftables.
type NfTables struct {
	name    string
	family  nftables.TableFamily
	locally bool
	once    sync.Once
}

// New creates a new NfTables instance with the specified name and address family.
func New(name string, family nftables.TableFamily) *NfTables {
	// The name is converted to uppercase and used as a prefix for custom chains
	// to ensure uniqueness and avoid conflicts with other nftables rules.
	nft := &NfTables{name: strings.ToUpper(name), family: family}

	return nft
}

// NewIPv4 creates a new NfTables instance configured for IPv4 port forwarding.
func NewIPv4(name string) *NfTables {
	return New(name, nftables.TableFamilyIPv4)
}

// AddForwarding creates all the necessary nftables rules to forward traffic
// from a specific port to a target IP address and port.
func (nft *NfTables) AddForwarding(proto transport.Protocol, from uint16, target string, to uint16, comment string) error {
	var err error
	nft.once.Do(func() { err = nft.start() })
	if err != nil {
		return errors.Wrap(err, "nftables: failed to start port forwarding")
	}

	return nft.present(nft.buildForwarding(proto, from, target, to, comment)...)
}

// RemoveForwarding removes rules that forward traffic from the specified port.
func (nft *NfTables) RemoveForwarding(proto transport.Protocol, from uint16, target string, to uint16) error {
	return nft.cleanUp(nft.buildForwarding(proto, from, target, to, "")...)
}

// Close removes all chains and rules created by this NfTables instance.
// This cleans up the entire port forwarding configuration.
func (nft *NfTables) Close() error {
	return nft.cleanUp(
		nft.foundation("nat", "PREROUTING", nftables.ChainHookPrerouting),
		nft.foundation("nat", "POSTROUTING", nftables.ChainHookPostrouting),
		nft.foundation("filter", "FORWARD", nftables.ChainHookForward,
			// Allow established connections through the firewall
			mutation.Rule(
				condition.TrackingEstablishedRelated(),
				condition.Accept(),
			).First(true).Comment(nft.name),
		),
		nft.foundation("nat", "OUTPUT", nftables.ChainHookOutput),
	)
}

// start initializes the basic chain structure needed for port forwarding.
//
// It creates all necessary tables and chains but doesn't add any forwarding rules yet.
func (nft *NfTables) start() error {
	return nft.present(
		nft.foundation("nat", "PREROUTING", nftables.ChainHookPrerouting),
		nft.foundation("nat", "POSTROUTING", nftables.ChainHookPostrouting),
		nft.foundation("filter", "FORWARD", nftables.ChainHookForward,
			// Allow established connections through the firewall
			mutation.Rule(
				condition.TrackingEstablishedRelated(),
				condition.Accept(),
			).First(true).Comment(nft.name),
		),
		nft.foundation("nat", "OUTPUT", nftables.ChainHookOutput),
	)
}

// buildForwarding creates the set of table mutations needed to implement port forwarding
// for a specific protocol, port, and target. The rules use connection tracking marks to
// associate the rules with each other, which helps with proper cleanup and prevents
// interference between different forwarding rules.
func (nft *NfTables) buildForwarding(proto transport.Protocol, from uint16, target string, to uint16, comment string) []mutation.TableMutation {
	mutations := []mutation.TableMutation{
		// Redirect incoming traffic (DNAT)
		nft.scaffold("nat", "PREROUTING", nftables.ChainHookPrerouting,
			mutation.Rule(
				condition.TransportProtocol(proto),
				condition.DestinationPort(from),
				condition.Counter(),
				condition.SetTrackingMark(uint32(from)),
				condition.DestinationNAT(net.ParseIP(target), to),
			).Comment(comment),
		),
		// Handle return traffic with masquerading (SNAT)
		nft.scaffold("nat", "POSTROUTING", nftables.ChainHookPostrouting,
			mutation.Rule(
				condition.DestinationIp(net.ParseIP(target)),
				condition.TransportProtocol(proto),
				condition.DestinationPort(to),
				condition.TrackingMark(uint32(from)),
				condition.Counter(),
				condition.Masquerade(),
			).Comment(comment),
		),
		// Allow connections through the firewall
		nft.scaffold("filter", "FORWARD", nftables.ChainHookForward,
			// Allow new connections to the forwarded service through the firewall
			mutation.Rule(
				condition.DestinationIp(net.ParseIP(target)),
				condition.TransportProtocol(proto),
				condition.DestinationPort(to),
				condition.TrackingMark(uint32(from)),
				condition.Counter(),
				condition.Accept(),
			).Comment(comment),
		),
	}

	if nft.locally {
		// Optional: Handle locally generated traffic
		mutations = append(mutations, nft.scaffold("nat", "OUTPUT", nftables.ChainHookOutput,
			mutation.Rule(
				condition.SourceLocalAddr(),
				condition.TransportProtocol(proto),
				condition.DestinationPort(from),
				condition.Counter(),
				condition.DestinationNAT(net.ParseIP(target), to),
			).Comment(comment),
		))
	}

	return mutations
}

// foundation creates a complete table mutation that includes both the base chain and regular chain.
func (nft *NfTables) foundation(table, chain string, hook *nftables.ChainHook, rules ...mutation.RuleMutation) mutation.TableMutation {
	return mutation.Table(table, hook).
		AddChain(mutation.RegularChain(nft.nameOf(chain)).
			AddRule(rules...)).
		AddChain(mutation.BaseChain(chain).
			AddRule(
				mutation.Rule(
					condition.Counter(),
					condition.Jump(nft.nameOf(chain)),
				).Comment(nft.name),
			),
		)
}

// scaffold creates a table mutation that only adds rules to an existing regular chain.
func (nft *NfTables) scaffold(table, chain string, hook *nftables.ChainHook, rules ...mutation.RuleMutation) mutation.TableMutation {
	return mutation.Table(table, hook).
		AddChain(mutation.Chain(nft.nameOf(chain)).AddRule(rules...))
}

// nameOf creates a namespaced chain name using the NfTables instance name as prefix.
func (nft *NfTables) nameOf(suffix string) string {
	return strings.Join([]string{nft.name, strings.ToUpper(suffix)}, "-")
}

// present applies a series of mutations to ensure they are present in the current state.
func (nft *NfTables) present(mutations ...mutation.TableMutation) error {
	return nft.runMutation(func(m mutation.TableMutation, rd mutation.TableReader) func(mutation.TableWriter) error {
		return m.Present(rd)
	}, mutations)
}

// cleanUp reverses a series of mutations, removing any changes they made.
func (nft *NfTables) cleanUp(mutations ...mutation.TableMutation) error {
	return nft.runMutation(func(m mutation.TableMutation, rd mutation.TableReader) func(mutation.TableWriter) error {
		clean := m.CleanUp(rd)
		return func(tw mutation.TableWriter) error {
			return ignoreNotFound(clean(tw))
		}
	}, mutations)
}

// MutationFunc is a function type that extracts an action function from a mutation.
type MutationFunc func(mutation.TableMutation, mutation.TableReader) func(mutation.TableWriter) error

// runMutation processes a series of mutations using the specified action function.
func (nft *NfTables) runMutation(action MutationFunc, mutations []mutation.TableMutation) error {
	for _, mut := range mutations {
		if mut == nil {
			continue
		}

		// creating a fresh reader/writer for each one.
		rw, err := newReadWriter(nft.name, nft.family)
		if err != nil {
			return err
		}

		if f := action(mut, rw); f != nil {
			if err = f(rw); err != nil {
				return err
			}
		}
	}
	return nil
}

// withOpenConn creates a new nftables connection and executes the provided action function with it.
func withOpenConn(action func(*nftables.Conn) error) error {
	// In the nftables package, each command that needs to be sent is first stored
	// in the messages field of the netlink connection object, and then submitted
	// together during Flush. Therefore, we need to create a new connection each
	// time to prevent asynchronous or concurrency issues.
	conn, err := nftables.New()
	if err != nil {
		return errors.Wrap(err, "failed to open netlink connection for nftables")
	}

	if err = action(conn); err != nil {
		return err
	}
	return errors.Wrap(conn.Flush(), "failed to send all commands to nftables")
}

// ignoreNotFound returns nil if the error is nil or represents a "not found" error.
// Otherwise, returns the original error.
func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}

	var opError *netlink.OpError
	if errors.As(err, &opError) {
		if os.IsNotExist(opError.Err) {
			return nil
		}
	}
	return err
}

// marshalUserComment prepares a comment string for inclusion in a nftables rule.
func marshalUserComment(comment string) []byte {
	if len(comment) == 0 {
		return nil
	}

	buf := make([]byte, len(comment)+3) // the length of the comment + '\x00'
	binary.BigEndian.PutUint16(buf, uint16(len(comment)+1))
	copy(buf[2:], comment)
	return buf
}

// unmarshalUserComment extracts a comment string from nftables rule user data.
//
// Returns an empty string if the data is invalid or too short.
func unmarshalUserComment(data []byte) string {
	if len(data) < 2 {
		return ""
	}

	sz := binary.BigEndian.Uint16(data[:2])
	if len(data) < int(sz)+2 { // header + (comment + '\x00')
		return ""
	}
	return string(data[2 : 1+sz])
}
