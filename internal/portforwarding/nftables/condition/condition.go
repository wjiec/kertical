//go:build linux

package condition

import (
	"net"
	"reflect"

	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
	netutils "k8s.io/utils/net"
)

// A Condition used for building nftables expressions.
type Condition interface {
	// Build returns the nftables expressions for the condition.
	Build() []expr.Any

	// Match checks if the given expressions match this condition.
	// Returns the number of matching expressions (0 if no match).
	Match(in []expr.Any) int
}

// Combine returns a Condition that combines multiple conditions into a single one.
func Combine(conditions ...Condition) Condition {
	return &combine{conditions: conditions}
}

// combine implements the Condition interface by combining multiple conditions.
type combine struct {
	conditions []Condition
}

// Build creates nftables expressions by concatenating the expressions
// from all the combined conditions in the order they were provided.
func (c *combine) Build() []expr.Any {
	var res []expr.Any
	for _, condition := range c.conditions {
		res = append(res, condition.Build()...)
	}
	return res
}

// Match checks if the input expressions match this combined condition.
//
// It sequentially tries to match each constituent condition against the
// remaining unmatched portion of the input.
//
// Returns the total number of matched expressions if all conditions match
// in sequence, or 0 if any condition fails.
func (c *combine) Match(in []expr.Any) int {
	var matched int
	for _, condition := range c.conditions {
		if matches := condition.Match(in[matched:]); matches == 0 {
			return 0
		} else {
			matched += matches
		}
	}
	return matched
}

// Counter returns a Condition that adds packet counting functionality.
func Counter() Condition {
	return &counter{}
}

// counter implements the Condition interface for counting packets.
type counter struct{}

// Build creates the nftables expression that adds packet and byte
// counting to a rule when included.
func (*counter) Build() []expr.Any {
	return []expr.Any{
		&expr.Counter{},
	}
}

// Match checks if the first expression is a Counter expression.
//
// Returns the length of the built expressions if matched, 0 otherwise.
func (c *counter) Match(in []expr.Any) int {
	if _, ok := in[0].(*expr.Counter); ok {
		return len(c.Build())
	}
	return 0
}

// TransportProtocol returns a Condition that matches packets of the specified transport protocol.
func TransportProtocol(proto netutils.Protocol) Condition {
	switch proto {
	case netutils.TCP:
		return Tcp()
	case netutils.UDP:
		return Udp()
	default:
		panic("unknown transport protocol")
	}
}

// Tcp returns a Condition that matches TCP protocol traffic.
func Tcp() Condition { return &transportProtocol{proto: unix.IPPROTO_TCP} }

// Udp returns a Condition that matches UDP protocol traffic.
func Udp() Condition { return &transportProtocol{proto: unix.IPPROTO_UDP} }

// transportProtocol implements the Condition interface for transport layer protocols.
type transportProtocol struct{ proto byte }

// Build creates the nftables expressions to match a specific transport protocol.
func (tp *transportProtocol) Build() []expr.Any {
	// It loads the L4 protocol into register 0x1 and then compare
	// it with the specified protocol value.
	return []expr.Any{
		&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 0x1},
		&expr.Cmp{Register: 0x1, Op: expr.CmpOpEq, Data: []byte{tp.proto}},
	}
}

// Match checks if the input expressions match this transport protocol condition.
func (tp *transportProtocol) Match(in []expr.Any) int {
	return prefixMatch(tp.Build(), in)
}

// DestinationPort returns a Condition that matches packets with the specified destination port.
func DestinationPort(port uint16) Condition {
	return &destinationPort{port: port}
}

// destinationPort implements the Condition interface for matching destination ports.
type destinationPort struct{ port uint16 }

// Build creates nftables expressions to match packets with a specific destination port.
func (dp *destinationPort) Build() []expr.Any {
	// It loads 2 bytes from offset 2 in the transport header (where the destination port is located)
	// into register 0x1, then compares it with the specified port value.
	return []expr.Any{
		&expr.Payload{Base: expr.PayloadBaseTransportHeader, Offset: 0x2, Len: 0x2, DestRegister: 0x1},
		&expr.Cmp{Register: 0x1, Op: expr.CmpOpEq, Data: binaryutil.BigEndian.PutUint16(dp.port)},
	}
}

// Match checks if the input expressions match this destination ports condition.
func (dp *destinationPort) Match(in []expr.Any) int {
	return prefixMatch(dp.Build(), in)
}

// SourcePort returns a Condition that matches packets with the specified source port.
func SourcePort(port uint16) Condition {
	return &sourcePort{port: port}
}

// sourcePort implements the Condition interface for matching source ports.
type sourcePort struct{ port uint16 }

// Build creates nftables expressions to match packets with a specific source port.
func (sp *sourcePort) Build() []expr.Any {
	// It loads 2 bytes from offset 0 in the transport header (where the source port is located)
	// into register 0x1, then compares it with the specified port value.
	return []expr.Any{
		&expr.Payload{Base: expr.PayloadBaseTransportHeader, Offset: 0x0, Len: 0x2, DestRegister: 0x1},
		&expr.Cmp{Register: 0x1, Op: expr.CmpOpEq, Data: binaryutil.BigEndian.PutUint16(sp.port)},
	}
}

func (sp *sourcePort) Match(in []expr.Any) int {
	return prefixMatch(sp.Build(), in)
}

// DestinationIp returns a Condition that matches packets with the specified destination IP address.
func DestinationIp(target DynamicCondition) Condition {
	return &destinationIp{target: target, reg: 0x1}
}

// destinationIp implements the Condition interface for matching destination IP addresses.
type destinationIp struct {
	target DynamicCondition
	reg    uint32
}

// Build creates nftables expressions to match packets with a specific destination IP.
func (di *destinationIp) Build() []expr.Any {
	// It loads 4 bytes from offset 0x10 in the network header (where the destination IP is located in IPv4)
	// into register 0x1, then compares it with the specified IP address.
	return append([]expr.Any{
		&expr.Payload{Base: expr.PayloadBaseNetworkHeader, Offset: 0x10, Len: 0x4, DestRegister: di.reg},
	}, di.target.Build(di.reg)...)
}

// Match checks if the input expressions match this destination IP condition.
func (di *destinationIp) Match(in []expr.Any) int {
	allBuilt, targetBuilt := di.Build(), di.target.Build(di.reg)
	if matched := prefixMatch(allBuilt[:len(allBuilt)-len(targetBuilt)], in); matched != 0 {
		if targetMatched := di.target.Match(di.reg, in[matched:]); targetMatched != 0 {
			return matched + targetMatched
		}
	}
	return 0
}

// DestinationNAT returns a Condition that performs destination NAT (DNAT)
// to the specified target IP and port.
func DestinationNAT(target DynamicCondition, port uint16) Condition {
	return &destinationNAT{target: target, port: port, reg: 0x1}
}

// destinationNAT implements the Condition interface for destination network address translation.
type destinationNAT struct {
	target DynamicCondition
	port   uint16
	reg    uint32
}

// Build creates nftables expressions for destination NAT (DNAT).
func (d *destinationNAT) Build() []expr.Any {
	// It loads the target IPv4 address into register 0x1, the target port into register 0x2,
	// and then applies the DNAT operation using these registers.
	return append(d.target.Build(d.reg),
		&expr.Immediate{Register: 0x2, Data: binaryutil.BigEndian.PutUint16(d.port)},
		&expr.NAT{
			Type: expr.NATTypeDestNAT, Family: unix.NFPROTO_IPV4,
			RegAddrMin: 0x1, RegAddrMax: 0x1, RegProtoMin: 0x2, RegProtoMax: 0x2,
			Specified: true,
		},
	)
}

// Match checks if the input expressions match this destination NAT condition.
func (d *destinationNAT) Match(in []expr.Any) int {
	allBuilt, targetBuilt := d.Build(), d.target.Build(d.reg)
	if targetMatched := d.target.Match(d.reg, in); targetMatched == len(targetBuilt) {
		if fixedMatched := prefixMatch(allBuilt[targetMatched:], in[targetMatched:]); fixedMatched != 0 {
			return targetMatched + fixedMatched
		}
	}
	return 0
}

// Masquerade returns a Condition that applies masquerading to outgoing packets.
// Masquerading is a form of source NAT that automatically uses the outgoing interface's IP address.
func Masquerade() Condition {
	return &masquerade{}
}

// masquerade implements the Condition interface for IP masquerading.
type masquerade struct{}

// Build creates the nftables expression that performs IP masquerading.
func (*masquerade) Build() []expr.Any {
	// This replaces the source IP address of outgoing packets with the address
	// of the outgoing interface, enabling internet access for private network hosts.
	return []expr.Any{
		&expr.Masq{},
	}
}

// Match checks if the input expressions match this masquerade condition.
func (m *masquerade) Match(in []expr.Any) int {
	return prefixMatch(m.Build(), in)
}

// SourceLocalAddr returns a Condition that matches packets whose source address
// is a local address on the host system.
func SourceLocalAddr() Condition {
	return &sourceLocalAddr{}
}

// sourceLocalAddr implements the Condition interface for matching packets
// with source addresses that are configured on the local system.
type sourceLocalAddr struct{}

// Build creates nftables expressions to match packets whose source IP
// address is a local address on the routing system.
func (*sourceLocalAddr) Build() []expr.Any {
	// It uses the FIB (Forwarding Information Base) to look up the address type of the
	// source address, then checks if it's of type RTN_LOCAL (locally configured address).
	return []expr.Any{
		&expr.Fib{Register: 0x1, ResultADDRTYPE: true, FlagSADDR: true},
		// see http://git.netfilter.org/nftables/tree/src/fib.c for more details.
		&expr.Cmp{Register: 0x1, Op: expr.CmpOpEq, Data: binaryutil.NativeEndian.PutUint32(unix.RTN_LOCAL)},
	}
}

// Match checks if the input expressions match this source local address condition.
func (sla *sourceLocalAddr) Match(in []expr.Any) int {
	return prefixMatch(sla.Build(), in)
}

// TrackingEstablishedRelated returns a Condition that matches packets belonging to established,related connections.
func TrackingEstablishedRelated() Condition {
	return &trackingEstablishedRelated{}
}

// trackingEstablishedRelated implements the Condition interface for matching established,related connections.
type trackingEstablishedRelated struct{}

// Build creates nftables expressions to match packets that belong to established,related connections.
func (*trackingEstablishedRelated) Build() []expr.Any {
	zero := binaryutil.NativeEndian.PutUint32(0)
	mask := binaryutil.NativeEndian.PutUint32(expr.CtStateBitESTABLISHED | expr.CtStateBitRELATED)
	// It uses connection tracking (ct) to retrieve the connection state,
	// then applies a bitwise operation to mask for the established state bits (0x6),
	// and finally checks if the result is non-zero (indicating an established connection).
	return []expr.Any{
		&expr.Ct{Register: 0x1, Key: expr.CtKeySTATE}, // conntrack state (bitmask of enum ip_conntrack_info)
		// see http://git.netfilter.org/nftables/tree/src/ct.c for more details.
		&expr.Bitwise{SourceRegister: 0x1, Len: 0x4, Mask: mask, Xor: zero, DestRegister: 0x1},
		&expr.Cmp{Register: 0x1, Op: expr.CmpOpNeq, Data: zero},
	}
}

// Match checks if the input expressions match this established,related condition.
func (ter *trackingEstablishedRelated) Match(in []expr.Any) int {
	return prefixMatch(ter.Build(), in)
}

// SetTrackingMark returns a Condition that sets a mark in the connection tracking system.
//
// Connection marks are used to identify and manage connections across different chains.
func SetTrackingMark(mark uint32) Condition { return &setTrackingMark{mark: mark} }

// setTrackingMark implements the Condition interface for setting connection tracking marks.
type setTrackingMark struct{ mark uint32 }

// Build creates nftables expressions to set a mark in the connection tracking system.
func (tm *setTrackingMark) Build() []expr.Any {
	// It loads the mark value into register 1, then sets it as the connection tracking mark.
	return []expr.Any{
		&expr.Immediate{Register: 0x1, Data: binaryutil.NativeEndian.PutUint32(tm.mark)},
		&expr.Ct{Key: expr.CtKeyMARK, Register: 0x1, SourceRegister: true},
	}
}

// Match checks if the input expressions match this set tracking mark condition.
func (tm *setTrackingMark) Match(in []expr.Any) int {
	return prefixMatch(tm.Build(), in)
}

// TrackingMark returns a Condition that matches packets with a specific connection tracking mark.
//
// This can be used to identify connections that were previously marked.
func TrackingMark(mark uint32) Condition {
	return &trackingMark{mark: mark}
}

// trackingMark implements the Condition interface for matching connection tracking marks.
type trackingMark struct{ mark uint32 }

// Build creates nftables expressions to match packets with a specific connection tracking mark.
func (tm *trackingMark) Build() []expr.Any {
	// It loads the connection tracking mark into register 0x1, then compares it with the specified mark value.
	return []expr.Any{
		&expr.Ct{Key: expr.CtKeyMARK, Register: 0x1},
		&expr.Cmp{Register: 0x1, Op: expr.CmpOpEq, Data: binaryutil.NativeEndian.PutUint32(tm.mark)},
	}
}

// Match checks if the input expressions match this tracking mark condition.
func (tm *trackingMark) Match(in []expr.Any) int {
	return prefixMatch(tm.Build(), in)
}

// Accept returns a Condition that accepts matching packets.
func Accept() Condition {
	return &accept{}
}

// accept implements the Condition interface for accepting packets.
type accept struct{}

// Build creates the nftables expression that applies the accept verdict to matching packets,
// allowing them to continue through the network stack.
func (*accept) Build() []expr.Any {
	return []expr.Any{
		&expr.Verdict{Kind: expr.VerdictAccept},
	}
}

// Match checks if the input expressions match this accept condition.
func (a *accept) Match(in []expr.Any) int {
	return prefixMatch(a.Build(), in)
}

// Jump returns a Condition that jumps to the specified chain for additional processing.
//
// After processing in the target chain completes, execution returns to the next rule
// in the current chain.
func Jump(chain string) Condition {
	return &jump{chain: chain}
}

// jump implements the Condition interface for jumping to another chain.
type jump struct{ chain string }

// Build creates the nftables expression that jumps to the specified chain.
func (j *jump) Build() []expr.Any {
	return []expr.Any{
		&expr.Verdict{Kind: expr.VerdictJump, Chain: j.chain},
	}
}

// Match checks if the input expressions match this jump condition.
func (j *jump) Match(in []expr.Any) int {
	return prefixMatch(j.Build(), in)
}

// comparators is a registry of specialized comparison functions for different nftables expression types.
var comparators = map[reflect.Type]func(a, b expr.Any) bool{}

func init() {
	comparators[reflect.TypeOf(&expr.Lookup{})] = lookupEquals
}

// compareExprEquals determines if two nftables expressions are equivalent.
// It uses specialized comparison functions for certain expression types when available,
// falling back to deep equality comparison for types without custom comparators.
func compareExprEquals(a, b expr.Any) bool {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}

	if cmp := comparators[reflect.TypeOf(a)]; cmp != nil {
		return cmp(a, b)
	}
	return reflect.DeepEqual(a, b)
}

// prefixMatch compares expected expressions against the beginning of actual expressions.
//
// Returns the number of matching expressions if the actual expressions start with
// the expected expressions, 0 otherwise.
func prefixMatch(expected []expr.Any, actual []expr.Any) int {
	if el, al := len(expected), len(actual); al >= el {
		for i := 0; i < el; i++ {
			if !compareExprEquals(expected[i], actual[i]) {
				return 0
			}
		}
		return el
	}
	return 0
}

// DynamicCondition represents a condition that works with a specific register
// and can build expressions using that register as well as match existing expressions.
type DynamicCondition interface {
	// Build generates nftables expressions using the specified register
	Build(reg uint32) []expr.Any

	// Match checks if expressions match this condition for the given register
	Match(reg uint32, in []expr.Any) int
}

// ImmediateIp creates a constraint that loads a specific IP address into the specified register.
func ImmediateIp(target net.IP) DynamicCondition {
	return &immediateIp{target: target}
}

// immediateIp implements DynamicCondition for loading an IP address into a register
type immediateIp struct {
	target net.IP
}

// Build creates an expression that loads the target IP into the specified register
func (i *immediateIp) Build(reg uint32) []expr.Any {
	return []expr.Any{
		&expr.Immediate{Register: reg, Data: i.target.To4()},
	}
}

// Match checks if the input expressions match loading this IP address
func (i *immediateIp) Match(reg uint32, in []expr.Any) int {
	return prefixMatch(i.Build(reg), in)
}

// LoadBalancing creates a constraint that implements simple load balancing
// by generating an incremental numeric value and using it to look up an IP address
// from an indexed set. This enables round-robin distribution across multiple endpoints.
func LoadBalancing(setId uint32, setName string, size int) DynamicCondition {
	return &loadBalancing{setId: setId, setName: setName, elemSize: size}
}

// loadBalancing implements DynamicCondition for distributing traffic across multiple endpoints
type loadBalancing struct {
	setId    uint32
	setName  string
	elemSize int
}

// Build creates expressions that generate a number and look up a corresponding IP address
func (lb *loadBalancing) Build(reg uint32) []expr.Any {
	return []expr.Any{
		&expr.Numgen{Register: reg, Modulus: uint32(lb.elemSize), Type: unix.NFT_NG_INCREMENTAL},
		&expr.Lookup{SourceRegister: reg, SetID: lb.setId, SetName: lb.setName, DestRegister: reg, IsDestRegSet: true},
	}
}

// Match checks if the input expressions match this load balancing
func (lb *loadBalancing) Match(reg uint32, in []expr.Any) int {
	return prefixMatch(lb.Build(reg), in)
}

// CompareEqualsIp creates a condition that checks if a register contains a specific IP address
func CompareEqualsIp(target net.IP) DynamicCondition {
	return &compareEqualsIp{target: target}
}

// compareEqualsIp implements DynamicCondition for IP equality comparison
type compareEqualsIp struct {
	target net.IP
}

// Build creates an expression that compares the register value with the target IP
func (cei *compareEqualsIp) Build(reg uint32) []expr.Any {
	return []expr.Any{
		&expr.Cmp{Register: reg, Op: expr.CmpOpEq, Data: cei.target.To4()},
	}
}

// Match checks if the input expressions match this IP comparison
func (cei *compareEqualsIp) Match(reg uint32, in []expr.Any) int {
	return prefixMatch(cei.Build(reg), in)
}

// LookupIp creates a condition that checks if a register's value exists in a set
func LookupIp(setId uint32, setName string) DynamicCondition {
	return &lookupIp{setId: setId, setName: setName}
}

// lookupIp implements DynamicCondition for IP set membership tests
type lookupIp struct {
	setId   uint32
	setName string
}

// Build creates an expression that checks if the register value is in the specified set
func (li *lookupIp) Build(reg uint32) []expr.Any {
	return []expr.Any{
		&expr.Lookup{SourceRegister: reg, SetID: li.setId, SetName: li.setName},
	}
}

// Match checks if the input expressions match this set lookup operation
func (li *lookupIp) Match(reg uint32, in []expr.Any) int {
	return prefixMatch(li.Build(reg), in)
}

// lookupEquals compares two nftables expressions to check if they represent equivalent lookup operations.
// This is used for matching existing rules in the nftables ruleset against desired configurations.
//
// Note that for set identification, either matching SetID OR matching SetName is sufficient,
// since these are two different ways to reference the same set.
func lookupEquals(a, b expr.Any) bool {
	if lookup1, ok := a.(*expr.Lookup); ok {
		if lookup2, ok := b.(*expr.Lookup); ok {
			return lookup1.SourceRegister == lookup2.SourceRegister &&
				lookup1.DestRegister == lookup2.DestRegister &&
				lookup1.IsDestRegSet == lookup2.IsDestRegSet &&
				(lookup1.SetID == lookup2.SetID || lookup1.SetName == lookup2.SetName) &&
				lookup1.Invert == lookup2.Invert
		}
	}
	return false
}
