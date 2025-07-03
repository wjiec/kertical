package filter

import (
	"github.com/google/nftables"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/condition"
	"github.com/wjiec/kertical/internal/portforwarding/nftables/mutation/predicate"
)

// ChainName returns a predicate function that checks if a chain has the specified name.
func ChainName(name string) func(*nftables.Chain) bool {
	return func(chain *nftables.Chain) bool {
		return chain.Name == name
	}
}

// ChainHook returns a predicate function that checks if a chain has the specified hook.
//
// Compares hook numbers using pointer equality to handle nil cases properly.
func ChainHook(hook *nftables.ChainHook) func(*nftables.Chain) bool {
	return func(chain *nftables.Chain) bool {
		return predicate.PtrEquals(chain.Hooknum, hook)
	}
}

// RuleExpr returns a predicate function that checks if a rule's expressions
// completely match those that would be built by the given condition.
func RuleExpr(cond condition.Condition) func(*nftables.Rule) bool {
	return func(rule *nftables.Rule) bool {
		return cond.Match(rule.Exprs) == len(rule.Exprs)
	}
}

// RuleChainName returns a predicate function that checks if a rule belongs
// to a chain with the specified name.
func RuleChainName(name string) func(*nftables.Rule) bool {
	return func(rule *nftables.Rule) bool {
		return rule.Chain.Name == name
	}
}
