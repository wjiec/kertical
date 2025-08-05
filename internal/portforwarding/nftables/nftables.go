//go:build linux

package nftables

import (
	"bytes"
	"math/rand"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/google/nftables"
	"github.com/pkg/errors"
	netutils "k8s.io/utils/net"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/netlink"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/sysctl"
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
	name   string
	family nftables.TableFamily
	once   sync.Once
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
func (nft *NfTables) AddForwarding(proto netutils.Protocol, from uint16, target []string, to uint16, comment string) (err error) {
	nft.once.Do(func() { err = nft.start() })
	if err != nil {
		return errors.Wrap(err, "failed to start port forwarding")
	}

	return nft.present(nft.buildForwarding(proto, from, target, to, comment)...)
}

// RemoveForwarding removes rules that forward traffic from the specified port.
func (nft *NfTables) RemoveForwarding(proto netutils.Protocol, from uint16, target []string, to uint16) error {
	return nft.cleanUp(nft.buildForwarding(proto, from, target, to, "")...)
}

// Close removes all chains and rules created by this NfTables instance.
// This cleans up the entire port forwarding configuration.
func (nft *NfTables) Close() error {
	return nft.cleanUp(nft.structure(true)...)
}

// start initializes the basic chain structure needed for port forwarding.
//
// It creates all necessary tables and chains but doesn't add any forwarding rules yet.
func (nft *NfTables) start() error {
	// Force cleanup of any residual rules from previous runs before starting
	if err := nft.Close(); err != nil {
		return errors.Wrap(err, "failed to start port forwarding")
	}
	return nft.present(nft.structure(false)...)
}

// structure defines the basic nftables structure required for port forwarding
func (nft *NfTables) structure(forceClean bool) []mutation.TableMutation {
	return []mutation.TableMutation{
		// Setup NAT PREROUTING chain for incoming traffic redirection
		nft.foundation("nat", "PREROUTING", nftables.ChainHookPrerouting, forceClean),

		// Setup NAT POSTROUTING chain for outgoing traffic masquerading
		nft.foundation("nat", "POSTROUTING", nftables.ChainHookPostrouting, forceClean),

		// Setup filter FORWARD chain with rule to allow established connections
		nft.foundation("filter", "FORWARD", nftables.ChainHookForward, forceClean,
			// Allow established connections through the firewall
			mutation.Rule(
				condition.TrackingEstablishedRelated(),
				condition.Accept(),
			).First(true).Comment(nft.name),
		),

		// Setup NAT OUTPUT chain for locally generated traffic
		nft.foundation("nat", "OUTPUT", nftables.ChainHookOutput, forceClean),
	}
}

// buildForwarding creates the set of table mutations needed to implement port forwarding
// for a specific protocol, port, and target. The rules use connection tracking marks to
// associate the rules with each other, which helps with proper cleanup and prevents
// interference between different forwarding rules.
func (nft *NfTables) buildForwarding(proto netutils.Protocol, from uint16, target []string, to uint16, comment string) []mutation.TableMutation {
	targetIp := mapTo(target, func(s string, _ int) net.IP { return net.ParseIP(s).To4() })
	slices.SortFunc(targetIp, func(a, b net.IP) int {
		return bytes.Compare(a, b)
	})

	outgoing := func(suffix string) (condition.DynamicCondition, []mutation.SetMutation) {
		if len(targetIp) == 1 {
			return condition.ImmediateIp(targetIp[0]), nil
		}

		outgoingMapId := rand.Uint32()
		outgoingMapName := nft.internalNameOf("map", strconv.Itoa(int(from)), suffix)
		return condition.LoadBalancing(outgoingMapId, outgoingMapName, len(target)), []mutation.SetMutation{
			mutation.IndexedIPv4AddrMap(outgoingMapId, outgoingMapName).
				AddElement(mapTo(targetIp, func(ip net.IP, index int) mutation.SetElementMutation {
					return mutation.IndexedIPv4Addr(ip, uint32(index))
				})...).Comment(comment),
		}
	}

	incoming := func(suffix string) (condition.DynamicCondition, []mutation.SetMutation) {
		if len(targetIp) == 1 {
			return condition.CompareEqualsIp(targetIp[0]), nil
		}

		incomingSetId := rand.Uint32()
		incomingSetName := nft.internalNameOf("map", strconv.Itoa(int(from)), suffix)
		return condition.LookupIp(incomingSetId, incomingSetName), []mutation.SetMutation{
			mutation.IPv4AddrSet(incomingSetId, incomingSetName).
				AddElement(mapTo(targetIp, func(ip net.IP, index int) mutation.SetElementMutation {
					return mutation.IPv4Addr(ip)
				})...).Comment(comment),
		}
	}

	var mutations []mutation.TableMutation

	// Redirect incoming traffic (DNAT)
	preroutingConstrain, preroutingMaps := outgoing("pre")
	mutations = append(mutations, nft.scaffold("nat", "PREROUTING", nftables.ChainHookPrerouting,
		mutation.Rule(
			condition.TransportProtocol(proto),
			condition.DestinationPort(from),
			condition.Counter(),
			condition.SetTrackingMark(uint32(from)),
			condition.DestinationNAT(preroutingConstrain, to),
		).Comment(comment),
	).AddSet(preroutingMaps...))

	// Handle return traffic with masquerading (SNAT)
	postroutingConstrain, postroutingSets := incoming("post")
	mutations = append(mutations, nft.scaffold("nat", "POSTROUTING", nftables.ChainHookPostrouting,
		mutation.Rule(
			condition.DestinationIp(postroutingConstrain),
			condition.TransportProtocol(proto),
			condition.DestinationPort(to),
			condition.TrackingMark(uint32(from)),
			condition.Counter(),
			condition.Masquerade(),
		).Comment(comment),
	).AddSet(postroutingSets...))

	// Allow connections through the firewall
	forwardConstrain, forwardSets := incoming("forward")
	mutations = append(mutations, nft.scaffold("filter", "FORWARD", nftables.ChainHookForward,
		mutation.Rule(
			condition.DestinationIp(forwardConstrain),
			condition.TransportProtocol(proto),
			condition.DestinationPort(to),
			condition.TrackingMark(uint32(from)),
			condition.Counter(),
			condition.Accept(),
		).Comment(comment),
	).AddSet(forwardSets...))

	// Handle locally generated traffic
	outputConstrain, outputMaps := outgoing("output")
	mutations = append(mutations, nft.scaffold("nat", "OUTPUT", nftables.ChainHookOutput,
		mutation.Rule(
			condition.SourceLocalAddr(),
			condition.TransportProtocol(proto),
			condition.DestinationPort(from),
			condition.Counter(),
			condition.DestinationNAT(outputConstrain, to),
		).Comment(comment),
	).AddSet(outputMaps...))

	return mutations
}

// foundation creates a complete table mutation that includes both the base chain and regular chain.
func (nft *NfTables) foundation(table, chain string, hook *nftables.ChainHook, forceClean bool, rules ...mutation.RuleMutation) mutation.TableMutation {
	return mutation.Table(table, hook).
		AddChain(mutation.RegularChain(nft.nameOf(chain), forceClean).
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
		AddChain(mutation.Chain(nft.nameOf(chain)).
			AddRule(rules...),
		)
}

// nameOf creates a namespaced chain name using the NfTables instance name as prefix.
func (nft *NfTables) nameOf(suffix string) string {
	return strings.Join([]string{nft.name, strings.ToUpper(suffix)}, "-")
}

// internalNameOf creates a namespaced internal resource name (like sets or maps)
func (nft *NfTables) internalNameOf(kind string, suffix ...string) string {
	components := make([]string, 0, len(suffix)+1)
	components = append(components, nft.name)
	components = append(components, suffix...)
	return strings.ToLower("__" + kind + "_" + strings.Join(components, "_"))
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
		return m.CleanUp(rd)
	}, mutations)
}

// MutationFunc is a function type that extracts an action function from a mutation.
type MutationFunc func(mutation.TableMutation, mutation.TableReader) func(mutation.TableWriter) error

// runMutation processes a series of mutations using the specified action function.
func (nft *NfTables) runMutation(action MutationFunc, mutations []mutation.TableMutation) error {
	for _, mut := range mutations {
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

		// flush batched commands
		if err = rw.Flush(); err != nil {
			return netlink.IgnoreNotFound(err)
		}
	}
	return nil
}

// mapTo transforms a slice of type T to a slice of type R using the provided transform function.
func mapTo[T any, R any](s []T, transform func(T, int) R) []R {
	res := make([]R, 0, len(s))
	for i, elem := range s {
		res = append(res, transform(elem, i))
	}
	return res
}
