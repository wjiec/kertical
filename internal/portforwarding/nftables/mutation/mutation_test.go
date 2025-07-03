package mutation_test

import (
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
