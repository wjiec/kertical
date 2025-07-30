//go:build linux

package filter

import (
	"bytes"

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

// ChainPriority returns a predicate function that checks if a chain has the specified priority.
//
// Compares priority numbers using pointer equality to handle nil cases properly.
func ChainPriority(priority *nftables.ChainPriority) func(*nftables.Chain) bool {
	return func(chain *nftables.Chain) bool {
		return predicate.PtrEquals(chain.Priority, priority)
	}
}

// RuleExpr returns a predicate function that checks if a rule's expressions
// completely match those that would be built by the given condition.
func RuleExpr(cond condition.Condition) func(*nftables.Rule) bool {
	return func(rule *nftables.Rule) bool {
		return cond.Match(rule.Exprs) == len(rule.Exprs)
	}
}

// RuleComment returns a predicate function that checks if a rule's user data
// matches the provided byte slice exactly.
func RuleComment(comment []byte) func(*nftables.Rule) bool {
	return func(rule *nftables.Rule) bool {
		return bytes.Equal(rule.UserData, comment)
	}
}

// RuleChainName returns a predicate function that checks if a rule belongs
// to a chain with the specified name.
func RuleChainName(name string) func(*nftables.Rule) bool {
	return func(rule *nftables.Rule) bool {
		return rule.Chain.Name == name
	}
}

// SetName returns a predicate function that checks if a set/map has the specified name.
func SetName(name string) func(*nftables.Set) bool {
	return func(set *nftables.Set) bool {
		return set.Name == name
	}
}
