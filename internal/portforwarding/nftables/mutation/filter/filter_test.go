package filter_test

import (
	"testing"

	"github.com/google/nftables"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation/filter"
)

func TestChainName(t *testing.T) {
	if f := filter.ChainName("foo"); assert.NotNil(t, f) {
		assert.True(t, f(&nftables.Chain{Name: "foo"}))
		assert.False(t, f(&nftables.Chain{Name: "bar"}))
	}
}

func TestChainHook(t *testing.T) {
	if f := filter.ChainHook(nftables.ChainHookPrerouting); assert.NotNil(t, f) {
		assert.True(t, f(&nftables.Chain{Hooknum: nftables.ChainHookPrerouting}))
		assert.True(t, f(&nftables.Chain{Hooknum: ptr.To(nftables.ChainHook(0))}))
		assert.False(t, f(&nftables.Chain{Hooknum: nftables.ChainHookPostrouting}))
	}
}

func TestRuleExpr(t *testing.T) {
	if f := filter.RuleExpr(condition.Counter()); assert.NotNil(t, f) {
		assert.True(t, f(&nftables.Rule{Exprs: condition.Counter().Build()}))
		assert.False(t, f(&nftables.Rule{Exprs: condition.Masquerade().Build()}))
	}
}

func TestRuleChainName(t *testing.T) {
	if f := filter.RuleChainName("foo"); assert.NotNil(t, f) {
		assert.True(t, f(&nftables.Rule{Chain: &nftables.Chain{Name: "foo"}}))
		assert.False(t, f(&nftables.Rule{Chain: &nftables.Chain{Name: "bar"}}))
	}
}
