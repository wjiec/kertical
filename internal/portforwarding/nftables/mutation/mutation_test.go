//go:build linux

package mutation_test

import (
	"net"
	"testing"

	"github.com/google/nftables"
	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation"
)

func TestTable(t *testing.T) {
	assert.NotNil(t, mutation.Table("nat", nftables.ChainHookPrerouting))
}

func TestChain(t *testing.T) {
	assert.NotNil(t, mutation.Chain("app-prerouting"))
}

func TestBaseChain(t *testing.T) {
	assert.NotNil(t, mutation.BaseChain("prerouting"))
}

func TestRegularChain(t *testing.T) {
	assert.NotNil(t, mutation.RegularChain("app-prerouting"))
}

func TestRule(t *testing.T) {
	assert.NotNil(t, mutation.Rule(condition.Counter(), condition.Masquerade()))
}

func TestIPv4AddrSet(t *testing.T) {
	assert.NotNil(t, mutation.IPv4AddrSet("targets"))
}

func TestIndexedIPv4AddrMap(t *testing.T) {
	assert.NotNil(t, mutation.IndexedIPv4AddrMap("targets"))
}

func TestIPv4Addr(t *testing.T) {
	assert.NotNil(t, mutation.IPv4Addr(net.ParseIP("10.1.2.3")))
}

func TestIndexedIPv4Addr(t *testing.T) {
	assert.NotNil(t, mutation.IndexedIPv4Addr(net.ParseIP("10.1.2.3"), 3))
}
