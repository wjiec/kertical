package derive_test

import (
	"testing"

	"github.com/google/nftables"
	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/derive"
)

func TestChainType(t *testing.T) {
	assert.NotNil(t, derive.ChainType(nftables.ChainHookPrerouting))
	assert.NotNil(t, derive.ChainType(nftables.ChainHookPostrouting))
	assert.NotNil(t, derive.ChainType(nftables.ChainHookOutput))
	assert.NotNil(t, derive.ChainType(nftables.ChainHookForward))
}

func TestChainPriority(t *testing.T) {
	assert.NotNil(t, derive.ChainPriority(nftables.ChainHookPrerouting))
	assert.NotNil(t, derive.ChainPriority(nftables.ChainHookPostrouting))
	assert.NotNil(t, derive.ChainPriority(nftables.ChainHookOutput))
	assert.NotNil(t, derive.ChainPriority(nftables.ChainHookForward))
}

func TestChainPolicy(t *testing.T) {
	assert.NotNil(t, derive.ChainPolicy(nftables.ChainHookPrerouting))
	assert.NotNil(t, derive.ChainPolicy(nftables.ChainHookPostrouting))
	assert.NotNil(t, derive.ChainPolicy(nftables.ChainHookOutput))
	assert.NotNil(t, derive.ChainPolicy(nftables.ChainHookForward))
}
